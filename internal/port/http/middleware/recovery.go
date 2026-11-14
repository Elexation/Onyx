package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// Deliberate abort mid-response: let net/http drop the connection, never append a 500
				if err == http.ErrAbortHandler {
					panic(err)
				}
				slog.Error("panic recovered", "error", err, "path", redactSharePath(r.URL.Path), "stack", string(debug.Stack()))
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
