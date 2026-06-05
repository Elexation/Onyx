package middleware

import (
	"net/http"

	"github.com/Elexation/onyx/web"
)

func SecurityHeaders(trustedProxy, requireHTTPS bool) func(http.Handler) http.Handler {
	csp := "default-src 'self'; script-src 'self' " + web.ScriptHash + "; style-src 'self' 'unsafe-inline'; img-src 'self' blob: data:; media-src 'self' blob:; object-src 'none'; frame-ancestors 'self'; base-uri 'self'; form-action 'self'"
	permissions := "camera=(), microphone=(), geolocation=(), payment=(), usb=(), serial=(), midi=(), accelerometer=(), gyroscope=(), magnetometer=()"
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Security-Policy", csp)
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Permissions-Policy", permissions)
			w.Header().Set("X-Frame-Options", "SAMEORIGIN")
			w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
			w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
			if requireHTTPS || IsHTTPS(r, trustedProxy) {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}
