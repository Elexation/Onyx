package middleware

import (
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// StreamRateLimiter is a per-IP token-bucket limiter sized for streaming
// workloads. Burst tolerates hls.js startup (master + variant playlist +
// init + several prefetched segments); sustained refill comfortably
// exceeds real-time playback (~0.25 rps for 4s segments).
//
// Distinct from RateLimiter (which throttles auth-event failures and
// requires handler-side RecordSuccess rollback): this limiter is purely
// request-rate-based, never requires handler participation, and recovers
// continuously rather than imposing lockout cooldowns.
type StreamRateLimiter struct {
	mu           sync.Mutex
	buckets      map[string]*streamBucket
	trustedProxy bool
}

type streamBucket struct {
	tokens     float64
	lastRefill time.Time
}

const (
	streamMaxEntries       = 10000
	streamBurstCapacity    = 20.0
	streamRefillRate       = 5.0 // tokens per second
	streamBucketCleanupAge = time.Hour
)

func NewStreamRateLimiter(trustedProxy bool) *StreamRateLimiter {
	l := &StreamRateLimiter{
		buckets:      make(map[string]*streamBucket),
		trustedProxy: trustedProxy,
	}
	go l.cleanup()
	return l
}

// Middleware refills the caller's bucket by elapsed time, then either
// deducts one token and forwards the request, or responds 429 with a
// Retry-After seconds-until-next-token hint.
func (l *StreamRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := ClientIP(r, l.trustedProxy)
		if ip == "" {
			ip = r.RemoteAddr
		}
		now := time.Now()

		l.mu.Lock()
		b, ok := l.buckets[ip]
		if !ok {
			if len(l.buckets) >= streamMaxEntries {
				l.evictOldestLocked()
			}
			b = &streamBucket{tokens: streamBurstCapacity, lastRefill: now}
			l.buckets[ip] = b
		} else {
			elapsed := now.Sub(b.lastRefill).Seconds()
			b.tokens += elapsed * streamRefillRate
			if b.tokens > streamBurstCapacity {
				b.tokens = streamBurstCapacity
			}
			b.lastRefill = now
		}
		if b.tokens < 1 {
			wait := (1 - b.tokens) / streamRefillRate
			l.mu.Unlock()
			slog.Info("security_event", "event", "stream_rate_limit_triggered", "ip", ip, "path", r.URL.Path)
			w.Header().Set("Retry-After", strconv.Itoa(int(wait)+1))
			http.Error(w, `{"error":"too many requests"}`, http.StatusTooManyRequests)
			return
		}
		b.tokens--
		l.mu.Unlock()

		next.ServeHTTP(w, r)
	})
}

func (l *StreamRateLimiter) evictOldestLocked() {
	var oldestIP string
	var oldestTime time.Time
	first := true
	for ip, b := range l.buckets {
		if first || b.lastRefill.Before(oldestTime) {
			oldestIP = ip
			oldestTime = b.lastRefill
			first = false
		}
	}
	if oldestIP != "" {
		delete(l.buckets, oldestIP)
	}
}

func (l *StreamRateLimiter) cleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		l.mu.Lock()
		cutoff := time.Now().Add(-streamBucketCleanupAge)
		for ip, b := range l.buckets {
			if b.lastRefill.Before(cutoff) {
				delete(l.buckets, ip)
			}
		}
		l.mu.Unlock()
	}
}
