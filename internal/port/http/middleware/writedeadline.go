package middleware

import (
	"net/http"
	"sync/atomic"
	"time"
)

// WriteDeadline drops a client that stops reading: every write must finish within d.
// Unlike http.Server.WriteTimeout it never caps how long a whole download may take.
func WriteDeadline(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			dw := &deadlineWriter{ResponseWriter: w, rc: http.NewResponseController(w), d: d}
			next.ServeHTTP(dw, r)
			// The server flushes the buffered tail after the handler returns
			dw.extend()
		})
	}
}

type deadlineWriter struct {
	http.ResponseWriter
	rc      *http.ResponseController
	d       time.Duration
	managed atomic.Bool // set from other goroutines too, by MinThroughput
}

func (dw *deadlineWriter) extend() {
	if !dw.managed.Load() {
		_ = dw.rc.SetWriteDeadline(time.Now().Add(dw.d))
	}
}

func (dw *deadlineWriter) Write(p []byte) (int, error) {
	dw.extend()
	return dw.ResponseWriter.Write(p)
}

func (dw *deadlineWriter) Flush() {
	dw.extend()
	_ = dw.rc.Flush()
}

// SetWriteDeadline hands deadlines to a handler that sets its own, like the SSE feed
func (dw *deadlineWriter) SetWriteDeadline(t time.Time) error {
	dw.managed.Store(true)
	return dw.rc.SetWriteDeadline(t)
}

// Unwrap lets http.ResponseController reach the connection for everything else
func (dw *deadlineWriter) Unwrap() http.ResponseWriter {
	return dw.ResponseWriter
}
