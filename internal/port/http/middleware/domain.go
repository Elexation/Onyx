package middleware

import (
	"net"
	"net/http"
	"strings"
)

func DomainCanon(canonicalDomain string, tlsEnabled, trustedProxy bool, listenPort string) func(http.Handler) http.Handler {
	scheme := "http"
	defaultPort := "80"
	if tlsEnabled {
		scheme = "https"
		defaultPort = "443"
	}

	var hostPort string
	if trustedProxy {
		hostPort = canonicalDomain
	} else if listenPort != defaultPort {
		hostPort = net.JoinHostPort(canonicalDomain, listenPort)
	} else {
		hostPort = canonicalDomain
	}
	baseURL := scheme + "://" + hostPort

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := r.Host
			if h, _, err := net.SplitHostPort(host); err == nil {
				host = h
			}

			if host == "localhost" || host == "127.0.0.1" || host == "::1" {
				next.ServeHTTP(w, r)
				return
			}

			if strings.EqualFold(host, canonicalDomain) {
				next.ServeHTTP(w, r)
				return
			}

			http.Redirect(w, r, baseURL+r.URL.RequestURI(), http.StatusMovedPermanently)
		})
	}
}
