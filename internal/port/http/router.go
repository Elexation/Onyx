package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Elexation/onyx/internal/port/http/handler"
	"github.com/Elexation/onyx/internal/port/http/middleware"
	"github.com/Elexation/onyx/internal/service"
	"github.com/Elexation/onyx/web"
)

func NewRouter(auth *service.AuthService, files *service.FileService) *chi.Mux {
	r := chi.NewRouter()
	rl := middleware.NewRateLimiter()
	authHandler := handler.NewAuthHandler(auth, rl)
	fileHandler := handler.NewFileHandler(files)
	fileOpsHandler := handler.NewFileOpsHandler(files)

	r.Use(middleware.Recovery)
	r.Use(middleware.Logging)
	r.Use(middleware.SecurityHeaders)

	r.Get("/api/health", healthHandler)

	// Auth routes (public, with optional session context for status)
	r.Route("/api/auth", func(r chi.Router) {
		r.Get("/status", optionalAuth(auth, authHandler.Status))
		r.With(rl.Middleware).Post("/login", authHandler.Login)
		r.Post("/setup", authHandler.Setup)
		r.With(middleware.Auth(auth), middleware.CSRF).Post("/logout", authHandler.Logout)
	})

	// Protected API routes
	r.Route("/api", func(r chi.Router) {
		r.Use(middleware.Auth(auth))
		r.Use(middleware.CSRF)

		r.Get("/files/*", fileHandler.List)
		r.Get("/download/*", fileHandler.Download)

		r.Post("/files/mkdir", fileOpsHandler.MakeDir)
		r.Post("/files/rename", fileOpsHandler.Rename)
		r.Post("/files/move", fileOpsHandler.Move)
		r.Post("/files/copy", fileOpsHandler.Copy)
		r.Delete("/files", fileOpsHandler.Delete)
	})

	r.NotFound(web.SPAHandler())

	return r
}

// optionalAuth tries to load the session but doesn't require it
func optionalAuth(validator middleware.SessionValidator, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err == nil {
			session, _ := validator.ValidateSession(cookie.Value)
			if session != nil {
				ctx := r.Context()
				ctx = middleware.ContextWithSession(ctx, session)
				r = r.WithContext(ctx)
			}
		}
		next(w, r)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}
