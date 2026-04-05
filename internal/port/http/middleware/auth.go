package middleware

import (
	"context"
	"net/http"

	"github.com/Elexation/onyx/internal/domain"
)

type contextKey string

const sessionKey contextKey = "session"

type SessionValidator interface {
	ValidateSession(sessionID string) (*domain.Session, error)
}

func Auth(validator SessionValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("session")
			if err != nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			session, err := validator.ValidateSession(cookie.Value)
			if err != nil || session == nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			ctx := ContextWithSession(r.Context(), session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func ContextWithSession(ctx context.Context, s *domain.Session) context.Context {
	return context.WithValue(ctx, sessionKey, s)
}

func SessionFromContext(ctx context.Context) *domain.Session {
	s, _ := ctx.Value(sessionKey).(*domain.Session)
	return s
}
