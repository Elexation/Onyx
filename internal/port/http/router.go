package server

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/Elexation/onyx/internal/adapter/upload"
	"github.com/Elexation/onyx/internal/port/http/handler"
	"github.com/Elexation/onyx/internal/port/http/middleware"
	"github.com/Elexation/onyx/internal/service"
	"github.com/Elexation/onyx/web"
)

func NewRouter(auth *service.AuthService, files *service.FileService, settings *service.SettingsService, trash *service.TrashService, versions *service.VersionService, tus *upload.TusHandler) http.Handler {
	r := chi.NewRouter()
	rl := middleware.NewRateLimiter()
	authHandler := handler.NewAuthHandler(auth, rl)
	fileHandler := handler.NewFileHandler(files)
	fileOpsHandler := handler.NewFileOpsHandler(files)
	uploadHandler := handler.NewUploadHandler(files)
	settingsHandler := handler.NewSettingsHandler(settings, auth)
	trashHandler := handler.NewTrashHandler(trash)
	versionHandler := handler.NewVersionHandler(versions)

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
		r.With(middleware.Auth(auth), middleware.CSRF).Post("/change-password", settingsHandler.ChangePassword)
	})

	// Protected API routes
	r.Route("/api", func(r chi.Router) {
		r.Use(middleware.Auth(auth))
		r.Use(middleware.CSRF)

		r.Route("/files", func(r chi.Router) {
			r.Post("/mkdir", fileOpsHandler.MakeDir)
			r.Post("/rename", fileOpsHandler.Rename)
			r.Post("/move", fileOpsHandler.Move)
			r.Post("/copy", fileOpsHandler.Copy)
			r.Post("/check-conflicts", uploadHandler.CheckConflicts)
			r.Delete("/", fileOpsHandler.Delete)
			r.Get("/*", fileHandler.List)
		})
		r.Get("/download/zip", fileHandler.DownloadZip)
		r.Get("/download/*", fileHandler.Download)
		r.Get("/preview/*", fileHandler.Preview)

		r.Route("/trash", func(r chi.Router) {
			r.Get("/", trashHandler.List)
			r.Get("/count", trashHandler.Count)
			r.Post("/{id}/restore", trashHandler.Restore)
			r.Delete("/{id}", trashHandler.PermanentDelete)
			r.Delete("/", trashHandler.EmptyTrash)
		})

		r.Route("/versions", func(r chi.Router) {
			r.Get("/", versionHandler.List)
			r.Post("/{id}/restore", versionHandler.Restore)
			r.Delete("/{id}", versionHandler.Delete)
		})

		r.Get("/settings", settingsHandler.GetAll)
		r.Patch("/settings", settingsHandler.Update)
	})

	r.NotFound(web.SPAHandler())

	// Intercept /api/upload before Chi to avoid path mangling.
	// OPTIONS pass through without auth (tus CORS preflight).
	return uploadInterceptor(auth, tus, r)
}

// uploadInterceptor routes /api/upload requests directly to tusd,
// bypassing Chi's routing which modifies URL paths.
func uploadInterceptor(auth middleware.SessionValidator, tus http.Handler, next http.Handler) http.Handler {
	stripped := http.StripPrefix("/api/upload/", tus)
	authed := middleware.Auth(auth)(stripped)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/upload") {
			next.ServeHTTP(w, r)
			return
		}
		slog.Info("upload interceptor", "method", r.Method, "path", r.URL.Path)
		if r.Method == http.MethodOptions {
			stripped.ServeHTTP(w, r)
			return
		}
		authed.ServeHTTP(w, r)
	})
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
