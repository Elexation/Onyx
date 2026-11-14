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
	mu     sync.Mutex
	counts map[string]int
	max    int
	key    func(*http.Request) string
}

func NewConcurrencyLimiter(trustedProxy bool, maxPerIP int) *ConcurrencyLimiter {
	return NewConcurrencyLimiterBy(maxPerIP, func(r *http.Request) string { return ClientKey(r, trustedProxy) })
}

// NewConcurrencyLimiterBy caps in-flight requests per key. The key is logged, so it must never be a secret.
func NewConcurrencyLimiterBy(limit int, key func(*http.Request) string) *ConcurrencyLimiter {
	return &ConcurrencyLimiter{counts: make(map[string]int), max: limit, key: key}
}

func (l *ConcurrencyLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := l.key(r)

		l.mu.Lock()
		if l.counts[ip] >= l.max {
			l.mu.Unlock()
			slog.Info("security_event", "event", "concurrency_limit_triggered", "ip", ip, "path", redactSharePath(r.URL.Path))
			w.Header().Set("Retry-After", "1")
			http.Error(w, `{"error":"too many concurrent requests"}`, http.StatusTooManyRequests)
			return
		}
		// Reject new IPs when the map is full rather than evicting an
		// in-flight entry (eviction would corrupt the held counter).
		if _, present := l.counts[ip]; !present && len(l.counts) >= concurrencyMaxEntries {
			l.mu.Unlock()
			slog.Warn("concurrency limiter map full", "path", redactSharePath(r.URL.Path))
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

// GlobalLimiter caps in-flight requests across every client. When full it answers 503, never queues.
type GlobalLimiter struct {
	slots chan struct{}
}

func NewGlobalLimiter(max int) *GlobalLimiter {
	return &GlobalLimiter{slots: make(chan struct{}, max)}
}

func (l *GlobalLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case l.slots <- struct{}{}:
		default:
			slog.Info("security_event", "event", "global_limit_triggered", "path", redactSharePath(r.URL.Path))
			w.Header().Set("Retry-After", "1")
			http.Error(w, `{"error":"server busy, try again shortly"}`, http.StatusServiceUnavailable)
			return
		}
		defer func() { <-l.slots }()
		next.ServeHTTP(w, r)
	})
}
