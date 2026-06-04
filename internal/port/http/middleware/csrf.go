package middleware

import (
	"crypto/subtle"
	"net/http"
)

func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}

		// Bearer (PAT) auth is immune to CSRF: tokens are not auto-sent by
		// browsers, so the cross-origin attack vector doesn't apply.
		if IsBearerAuth(r.Context()) {
			next.ServeHTTP(w, r)
			return
		}

		session := SessionFromContext(r.Context())
		if session == nil {
			// Defense in depth: Auth middleware should always run before CSRF
			// on mutating routes, which leaves either a real session or a
			// synthetic bearer session (handled above). A nil session at this
			// point means the route was misconfigured — fail closed.
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}

		token := r.Header.Get("X-CSRF-Token")
		if subtle.ConstantTimeCompare([]byte(token), []byte(session.CSRFToken)) != 1 {
			http.Error(w, `{"error":"invalid csrf token"}`, http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
