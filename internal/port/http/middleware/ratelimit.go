package middleware

import (
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	maxAttemptEntries = 10000
	lockoutAfter      = 5         // failures before the cooldown starts
	attemptRetention  = time.Hour // cleanup forgets a key this long after its last failure
)

type loginAttempts struct {
	failCount int
	lastFail  time.Time
}

// heldAt reports whether a key has failed enough that forgetting it would hand back free attempts
func (a *loginAttempts) heldAt(now time.Time) bool {
	return a.failCount >= lockoutAfter && now.Sub(a.lastFail) < attemptRetention
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
// request as a failure before dispatching. Handlers MUST call RecordSuccess (admin
// auth) or UndoAttempt (share verify) on success; failures require no handler action. The
// pre-increment closes the check-then-act race where concurrent requests all
// read the same low count and pass the gate in parallel.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := ClientKey(r, rl.trustedProxy)

		rl.mu.Lock()
		a, ok := rl.attempts[key]
		if !ok {
			if len(rl.attempts) >= maxAttemptEntries {
				rl.evictOldestLocked()
			}
			a = &loginAttempts{}
			rl.attempts[key] = a
		}
		if a.failCount >= lockoutAfter {
			cooldown := cooldownFor(a.failCount)
			elapsed := time.Since(a.lastFail)
			if elapsed < cooldown {
				rl.mu.Unlock()
				slog.Info("security_event", "event", "rate_limit_triggered", "ip", key, "path", redactSharePath(r.URL.Path), "fail_count", a.failCount)
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

// RecordSuccess deletes the key's whole failure history, not just this request's increment.
func (rl *RateLimiter) RecordSuccess(r *http.Request) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.attempts, ClientKey(r, rl.trustedProxy))
}

// UndoAttempt removes only this request's pre-increment, for share verify
func (rl *RateLimiter) UndoAttempt(r *http.Request) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	key := ClientKey(r, rl.trustedProxy)
	a, ok := rl.attempts[key]
	if !ok {
		return
	}
	a.failCount--
	if a.failCount <= 0 {
		delete(rl.attempts, key)
	}
}

// evictOldestLocked drops the oldest key with few failures, so a flood of new keys can't reset a guesser. Caller holds rl.mu.
func (rl *RateLimiter) evictOldestLocked() {
	now := time.Now()
	var victim string
	var victimFail time.Time
	found, victimHeld := false, false
	for key, a := range rl.attempts {
		held := a.heldAt(now)
		// Only held keys left? Take the oldest anyway: refusing new keys would lock the admin out too.
		if !found || (victimHeld && !held) || (held == victimHeld && a.lastFail.Before(victimFail)) {
			victim, victimFail, victimHeld, found = key, a.lastFail, held, true
		}
	}
	if found {
		delete(rl.attempts, victim)
	}
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-attemptRetention)
		for ip, a := range rl.attempts {
			if a.lastFail.Before(cutoff) {
				delete(rl.attempts, ip)
			}
		}
		rl.mu.Unlock()
	}
}
