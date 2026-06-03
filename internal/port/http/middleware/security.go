package middleware

import (
	"net/http"

	"github.com/Elexation/onyx/web"
)

func SecurityHeaders(next http.Handler) http.Handler {
	csp := "default-src 'self'; script-src 'self' " + web.ScriptHash + "; style-src 'self' 'unsafe-inline'; img-src 'self' blob: data:; media-src 'self' blob:; object-src 'none'; frame-ancestors 'self'"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", csp)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}
