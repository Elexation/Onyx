package middleware

import (
	"log/slog"
	"net/http"
	"sync"
)

const concurrencyMaxEntries = 10000

// ConcurrencyLimiter caps concurrent in-flight requests per client IP.
// Distinct from RateLimiter (event-rate, lockout) and StreamRateLimiter
// (request-rate, token bucket): this counts overlapping in-flight requests
// and rejects past a per-IP cap. Sized for upload-style endpoints where
// each request holds resources (FDs, disk I/O, goroutines) for an extended
// period.
//
// Counters increment on entry under lock and decrement on exit via defer.
// Map entries are deleted when the per-IP count returns to zero, so the
// map only grows with currently-active IPs.
type ConcurrencyLimiter struct {
	mu           sync.Mutex
	counts       map[string]int
	max          int
	trustedProxy bool
}

func NewConcurrencyLimiter(trustedProxy bool, maxPerIP int) *ConcurrencyLimiter {
	return &ConcurrencyLimiter{
		counts:       make(map[string]int),
		max:          maxPerIP,
		trustedProxy: trustedProxy,
	}
}

func (l *ConcurrencyLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := ClientIP(r, l.trustedProxy)
		if ip == "" {
			ip = r.RemoteAddr
		}

		l.mu.Lock()
		if l.counts[ip] >= l.max {
			l.mu.Unlock()
			slog.Info("security_event", "event", "concurrency_limit_triggered", "ip", ip, "path", r.URL.Path)
			w.Header().Set("Retry-After", "1")
			http.Error(w, `{"error":"too many concurrent requests"}`, http.StatusTooManyRequests)
			return
		}
		// Reject new IPs when the map is full rather than evicting an
		// in-flight entry (eviction would corrupt the held counter).
		if _, present := l.counts[ip]; !present && len(l.counts) >= concurrencyMaxEntries {
			l.mu.Unlock()
			slog.Warn("concurrency limiter map full", "path", r.URL.Path)
			w.Header().Set("Retry-After", "1")
			http.Error(w, `{"error":"server overloaded"}`, http.StatusServiceUnavailable)
			return
		}
		l.counts[ip]++
		l.mu.Unlock()

		defer func() {
			l.mu.Lock()
			l.counts[ip]--
			if l.counts[ip] <= 0 {
				delete(l.counts, ip)
			}
			l.mu.Unlock()
		}()

		next.ServeHTTP(w, r)
	})
}
