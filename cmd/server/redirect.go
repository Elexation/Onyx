package main

import (
	"log/slog"
	"net"
	"net/http"
	"time"
)

func startHTTPRedirectListener(redirectPort, mainPort, canonicalDomain string, trustedProxy bool) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if host == "" {
			http.Error(w, "missing Host header", http.StatusBadRequest)
			return
		}
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		if canonicalDomain != "" {
			host = canonicalDomain
		}

		var target string
		if trustedProxy || mainPort == "443" {
			target = "https://" + host + r.URL.RequestURI()
		} else {
			target = "https://" + net.JoinHostPort(host, mainPort) + r.URL.RequestURI()
		}

		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})

	srv := &http.Server{
		Addr:              ":" + redirectPort,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		slog.Error("HTTPS redirect listener failed to bind", "port", redirectPort, "error", err)
		return
	}
	slog.Info("HTTPS redirect listener started", "port", redirectPort)
	if err := srv.Serve(ln); err != nil {
		slog.Error("HTTPS redirect listener failed", "error", err)
	}
}
