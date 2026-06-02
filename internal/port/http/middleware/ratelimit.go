package middleware

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type loginAttempts struct {
	failCount int
	lastFail  time.Time
}

type RateLimiter struct {
	mu       sync.Mutex
	attempts map[string]*loginAttempts
}

func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		attempts: make(map[string]*loginAttempts),
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)

		rl.mu.Lock()
		a := rl.attempts[ip]
		if a != nil && a.failCount >= 5 {
			cooldown := 5 * time.Second
			if a.failCount >= 20 {
				cooldown = 60 * time.Second
			} else if a.failCount >= 10 {
				cooldown = 30 * time.Second
			}
			elapsed := time.Since(a.lastFail)
			if elapsed < cooldown {
				rl.mu.Unlock()
				remaining := int((cooldown - elapsed).Seconds()) + 1
				w.Header().Set("Retry-After", strconv.Itoa(remaining))
				http.Error(w, `{"error":"too many attempts, try again later"}`, http.StatusTooManyRequests)
				return
			}
		}
		rl.mu.Unlock()

		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) RecordFailure(r *http.Request) {
	ip := extractIP(r)
	rl.mu.Lock()
	defer rl.mu.Unlock()

	a, ok := rl.attempts[ip]
	if !ok {
		a = &loginAttempts{}
		rl.attempts[ip] = a
	}
	a.failCount++
	a.lastFail = time.Now()
}

func (rl *RateLimiter) RecordSuccess(r *http.Request) {
	ip := extractIP(r)
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.attempts, ip)
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

func extractIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
