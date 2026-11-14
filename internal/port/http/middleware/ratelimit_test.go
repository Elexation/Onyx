package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func requestFrom(addr string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.RemoteAddr = addr
	return r
}

// Rotating addresses inside one IPv6 /64 must land in one bucket
func TestStreamRateLimiterMasksIPv6To64(t *testing.T) {
	l := NewStreamRateLimiter(false)
	h := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	var limited int
	for i := 1; i <= int(streamBurstCapacity)+1; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, requestFrom(fmt.Sprintf("[2001:db8:0:1::%x]:4000", i)))
		if rec.Code == http.StatusTooManyRequests {
			limited++
		}
	}
	if limited != 1 {
		t.Fatalf("%d of %d requests from one /64 limited, want 1", limited, int(streamBurstCapacity)+1)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, requestFrom("[2001:db8:0:2::1]:4000"))
	if rec.Code != http.StatusOK {
		t.Fatalf("another /64 got %d, want its own bucket", rec.Code)
	}
}

func TestThumbRateLimiterAllowsGridBurst(t *testing.T) {
	l := NewThumbRateLimiter(false)
	h := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	for i := 0; i < 150; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, requestFrom("10.0.0.5:4000"))
		if rec.Code != http.StatusOK {
			t.Fatalf("thumbnail %d of a 150-tile grid got %d", i+1, rec.Code)
		}
	}
}

func TestConcurrencyLimiterMasksIPv6To64(t *testing.T) {
	l := NewConcurrencyLimiter(false, 2)
	release := make(chan struct{})
	var entered sync.WaitGroup
	var admitted atomic.Int32
	h := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only the first two hold their slot; a wrongly admitted third returns at once
		if admitted.Add(1) <= 2 {
			entered.Done()
			<-release
		}
	}))
	var done sync.WaitGroup
	for i := 1; i <= 2; i++ {
		entered.Add(1)
		done.Add(1)
		go func(i int) {
			defer done.Done()
			h.ServeHTTP(httptest.NewRecorder(), requestFrom(fmt.Sprintf("[2001:db8::%x]:4000", i)))
		}(i)
	}
	entered.Wait()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, requestFrom("[2001:db8::3]:4000"))
	close(release)
	done.Wait()
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("third request from one /64 got %d, want 429", rec.Code)
	}
}

// A full map must evict keys with few failures before a guesser's, even one between cooldowns
func TestRateLimiterEvictionKeepsLiveLockout(t *testing.T) {
	rl := &RateLimiter{attempts: map[string]*loginAttempts{}}
	now := time.Now()
	rl.attempts["locked"] = &loginAttempts{failCount: lockoutAfter, lastFail: now.Add(-time.Second)}
	// Cooldown long over, but forgetting it would hand back free attempts
	rl.attempts["between-cooldowns"] = &loginAttempts{failCount: lockoutAfter + 1, lastFail: now.Add(-2 * time.Minute)}
	rl.attempts["idle-old"] = &loginAttempts{failCount: 1, lastFail: now.Add(-500 * time.Millisecond)}
	rl.attempts["idle-new"] = &loginAttempts{failCount: 1, lastFail: now}

	rl.evictOldestLocked()
	rl.evictOldestLocked()
	for _, key := range []string{"locked", "between-cooldowns"} {
		if _, ok := rl.attempts[key]; !ok {
			t.Fatalf("evicted %q while keys with few failures remained", key)
		}
	}
	if len(rl.attempts) != 2 {
		t.Fatalf("got %v, want both low-failure keys evicted", rl.attempts)
	}

	// Only guessers' keys left: the oldest goes, so a new client can still get a slot
	rl.attempts = map[string]*loginAttempts{
		"a": {failCount: lockoutAfter, lastFail: now.Add(-2 * time.Second)},
		"b": {failCount: lockoutAfter, lastFail: now.Add(-time.Second)},
	}
	rl.evictOldestLocked()
	if _, ok := rl.attempts["a"]; ok || len(rl.attempts) != 1 {
		t.Fatalf("all locked: got %v, want only the oldest removed", rl.attempts)
	}
}

func TestRateLimiterMasksIPv6To64(t *testing.T) {
	rl := &RateLimiter{attempts: map[string]*loginAttempts{}}
	h := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	codes := map[int]int{}
	for i := 1; i <= lockoutAfter+1; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, requestFrom(fmt.Sprintf("[2001:db8::%x]:4000", i)))
		codes[rec.Code]++
	}
	if codes[http.StatusTooManyRequests] != 1 {
		t.Fatalf("status counts %v, want the attempt after %d failures locked out", codes, lockoutAfter)
	}
}
