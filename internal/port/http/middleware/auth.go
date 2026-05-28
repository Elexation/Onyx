package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/Elexation/onyx/internal/domain"
)

type contextKey string

const (
	sessionKey    contextKey = "session"
	authMethodKey contextKey = "authMethod"
)

const (
	AuthMethodCookie = "cookie"
	AuthMethodBearer = "bearer"
)

type SessionValidator interface {
	ValidateSession(sessionID string) (*domain.Session, error)
}

type TokenValidator interface {
	ValidateToken(token string) (*domain.PersonalAccessToken, error)
	CheckScope(scope, method, path string) bool
}

func Auth(sessions SessionValidator, tokens TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if bearer, ok := extractBearer(r); ok {
				tok, err := tokens.ValidateToken(bearer)
				if err != nil || tok == nil {
					http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
					return
				}
				if !tokens.CheckScope(tok.Scope, r.Method, r.URL.Path) {
					http.Error(w, `{"error":"forbidden: token scope does not allow this action"}`, http.StatusForbidden)
					return
				}
				// Synthetic session for downstream handlers. UserID=0 and
				// empty ID/CSRFToken are intentional — bearer-authed requests
				// have no session identity, and no handler currently reads
				// UserID. Handlers that require a real session (ChangePassword)
				// are guarded by the admin-endpoint block in CheckScope.
				synthetic := &domain.Session{}
				ctx := ContextWithSession(r.Context(), synthetic)
				ctx = context.WithValue(ctx, authMethodKey, AuthMethodBearer)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			cookie, err := r.Cookie("session")
			if err != nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			session, err := sessions.ValidateSession(cookie.Value)
			if err != nil {
				http.Error(w, `{"error":"internal server error"}`, http.StatusServiceUnavailable)
				return
			}
			if session == nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			ctx := ContextWithSession(r.Context(), session)
			ctx = context.WithValue(ctx, authMethodKey, AuthMethodCookie)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractBearer(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", false
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return "", false
	}
	token := strings.TrimSpace(h[len(prefix):])
	if token == "" {
		return "", false
	}
	return token, true
}

func ContextWithSession(ctx context.Context, s *domain.Session) context.Context {
	return context.WithValue(ctx, sessionKey, s)
}

func SessionFromContext(ctx context.Context) *domain.Session {
	s, _ := ctx.Value(sessionKey).(*domain.Session)
	return s
}

// IsBearerAuth reports whether the current request was authenticated via a
// Personal Access Token (Authorization: Bearer) rather than a session cookie.
func IsBearerAuth(ctx context.Context) bool {
	m, _ := ctx.Value(authMethodKey).(string)
	return m == AuthMethodBearer
}
