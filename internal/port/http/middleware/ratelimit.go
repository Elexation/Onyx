package middleware

import (
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const maxAttemptEntries = 10000

type loginAttempts struct {
	failCount int
	lastFail  time.Time
}

type RateLimiter struct {
	mu           sync.Mutex
	attempts     map[string]*loginAttempts
	trustedProxy bool
}

func NewRateLimiter(trustedProxy bool) *RateLimiter {
	rl := &RateLimiter{
		attempts:     make(map[string]*loginAttempts),
		trustedProxy: trustedProxy,
	}
	go rl.cleanup()
	return rl
}

// Middleware atomically checks the lockout gate and pre-records the current
// request as a failure before dispatching. Handlers MUST call RecordSuccess on
// success to roll back the increment; failures require no handler action. The
// pre-increment closes the check-then-act race where concurrent requests all
// read the same low count and pass the gate in parallel.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := ClientIP(r, rl.trustedProxy)

		rl.mu.Lock()
		a, ok := rl.attempts[ip]
		if !ok {
			if len(rl.attempts) >= maxAttemptEntries {
				rl.evictOldestLocked()
			}
			a = &loginAttempts{}
			rl.attempts[ip] = a
		}
		if a.failCount >= 5 {
			cooldown := cooldownFor(a.failCount)
			elapsed := time.Since(a.lastFail)
			if elapsed < cooldown {
				rl.mu.Unlock()
				slog.Info("security_event", "event", "rate_limit_triggered", "ip", ip, "path", r.URL.Path, "fail_count", a.failCount)
				remaining := int((cooldown - elapsed).Seconds()) + 1
				w.Header().Set("Retry-After", strconv.Itoa(remaining))
				http.Error(w, `{"error":"too many attempts, try again later"}`, http.StatusTooManyRequests)
				return
			}
		}
		a.failCount++
		a.lastFail = time.Now()
		rl.mu.Unlock()

		next.ServeHTTP(w, r)
	})
}

func cooldownFor(failCount int) time.Duration {
	switch {
	case failCount >= 20:
		return 60 * time.Second
	case failCount >= 10:
		return 30 * time.Second
	default:
		return 5 * time.Second
	}
}

// RecordSuccess rolls back the pre-increment applied by Middleware. Called by
// handlers after a successful authentication.
func (rl *RateLimiter) RecordSuccess(r *http.Request) {
	ip := ClientIP(r, rl.trustedProxy)
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.attempts, ip)
}

// evictOldestLocked removes the entry with the oldest lastFail. Caller must
// hold rl.mu.
func (rl *RateLimiter) evictOldestLocked() {
	var oldestIP string
	var oldestTime time.Time
	first := true
	for ip, a := range rl.attempts {
		if first || a.lastFail.Before(oldestTime) {
			oldestIP = ip
			oldestTime = a.lastFail
			first = false
		}
	}
	if oldestIP != "" {
		delete(rl.attempts, oldestIP)
	}
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-time.Hour)
		for ip, a := range rl.attempts {
			if a.lastFail.Before(cutoff) {
				delete(rl.attempts, ip)
			}
		}
		rl.mu.Unlock()
	}
}
