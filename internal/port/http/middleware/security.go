package middleware

import (
	"net/http"

	"github.com/Elexation/onyx/web"
)

func SecurityHeaders(next http.Handler) http.Handler {
	csp := "default-src 'self'; script-src 'self' " + web.ScriptHash + "; style-src 'self' 'unsafe-inline'; object-src 'none'; frame-ancestors 'self'"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", csp)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}
