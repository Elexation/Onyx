package middleware

import (
	"net"
	"net/http"
	"strings"
)

// DomainCanon redirects other hosts to canonicalDomain, keeping the client's scheme and port
func DomainCanon(canonicalDomain string, trustedProxy bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host, port := r.Host, ""
			if h, p, err := net.SplitHostPort(host); err == nil {
				host, port = h, p
			}

			if host == "localhost" || host == "127.0.0.1" || host == "::1" {
				next.ServeHTTP(w, r)
				return
			}

			if strings.EqualFold(host, canonicalDomain) {
				next.ServeHTTP(w, r)
				return
			}

			scheme := "http"
			if IsHTTPS(r, trustedProxy) {
				scheme = "https"
			}
			hostPort := canonicalDomain
			if !trustedProxy && port != "" {
				hostPort = net.JoinHostPort(canonicalDomain, port)
			}
			http.Redirect(w, r, scheme+"://"+hostPort+r.URL.RequestURI(), http.StatusMovedPermanently)
		})
	}
}
