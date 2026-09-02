package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"
)

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(ww, r)
		slog.Info("request",
			"method", r.Method,
			"path", redactSharePath(r.URL.Path),
			"status", ww.status,
			"duration", time.Since(start),
		)
	})
}

// redactSharePath masks the token segment in share URLs; share tokens are
// bearer credentials and must not land in access logs.
func redactSharePath(p string) string {
	if strings.HasPrefix(p, "/api/public/s/") {
		return redactTokenSegment(p, len("/api/public/s/"))
	}
	if strings.HasPrefix(p, "/s/") {
		return redactTokenSegment(p, len("/s/"))
	}
	return p
}

func redactTokenSegment(p string, start int) string {
	end := strings.IndexByte(p[start:], '/')
	if end < 0 {
		return p[:start] + "REDACTED"
	}
	return p[:start] + "REDACTED" + p[start+end:]
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
