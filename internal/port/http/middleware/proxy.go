package middleware

import (
	"net"
	"net/http"
	"strings"
)

// ClientIP returns the source IP of the request.
//
// When trustedProxy is true, X-Real-IP is preferred; otherwise the right-most
// X-Forwarded-For entry is used. Standard proxies append the real client IP to
// X-Forwarded-For, so earlier entries are client-controllable and must not be
// trusted for rate limiting. When trustedProxy is false, forwarded headers are
// ignored and RemoteAddr is used.
func ClientIP(r *http.Request, trustedProxy bool) string {
	if trustedProxy {
		if ip := r.Header.Get("X-Real-IP"); ip != "" {
			return strings.TrimSpace(ip)
		}
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if idx := strings.LastIndex(xff, ","); idx >= 0 {
				return strings.TrimSpace(xff[idx+1:])
			}
			return strings.TrimSpace(xff)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// IsHTTPS reports whether the request originated over HTTPS. X-Forwarded-Proto
// is honored only when trustedProxy is true.
func IsHTTPS(r *http.Request, trustedProxy bool) bool {
	if r.TLS != nil {
		return true
	}
	if trustedProxy && r.Header.Get("X-Forwarded-Proto") == "https" {
		return true
	}
	return false
}
