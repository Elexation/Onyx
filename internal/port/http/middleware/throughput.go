package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

var errTooSlow = errors.New("client reading too slowly")

// MinThroughput drops a response averaging under bytesPerSec per window, so a trickling reader can't hold a slot for days
func MinThroughput(bytesPerSec int64, window time.Duration) func(http.Handler) http.Handler {
	floor := bytesPerSec * int64(window) / int64(time.Second)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tw := &throughputWriter{ResponseWriter: w}
			rc := http.NewResponseController(w)
			stop := make(chan struct{})
			defer close(stop)
			go func() {
				t := time.NewTicker(window)
				defer t.Stop()
				for {
					select {
					case <-stop:
						return
					case <-t.C:
						if tw.sent.Swap(0) >= floor {
							continue
						}
						tw.dropped.Store(true)
						// Interrupt a write that is blocked right now; later writes fail on dropped
						_ = rc.SetWriteDeadline(time.Now())
						slog.Info("security_event", "event", "slow_download_dropped", "path", redactSharePath(r.URL.Path))
						return
					}
				}
			}()
			next.ServeHTTP(tw, r)
		})
	}
}

type throughputWriter struct {
	http.ResponseWriter
	sent    atomic.Int64
	dropped atomic.Bool
}

func (tw *throughputWriter) Write(p []byte) (int, error) {
	if tw.dropped.Load() {
		return 0, errTooSlow
	}
	n, err := tw.ResponseWriter.Write(p)
	tw.sent.Add(int64(n))
	return n, err
}

// Unwrap lets http.ResponseController reach the connection
func (tw *throughputWriter) Unwrap() http.ResponseWriter {
	return tw.ResponseWriter
}
