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

func NewRouter(auth *service.AuthService, files *service.FileService, settings *service.SettingsService, trash *service.TrashService, versions *service.VersionService, tus *upload.TusHandler, search *service.SearchService, shares *service.ShareService, tokens *service.TokenService, thumbs *service.ThumbnailService, probe *service.ProbeService, transcode *service.TranscodeService) http.Handler {
	r := chi.NewRouter()
	rl := middleware.NewRateLimiter()
	shareRL := middleware.NewRateLimiter()
	authHandler := handler.NewAuthHandler(auth, rl)
	fileHandler := handler.NewFileHandler(files)
	fileOpsHandler := handler.NewFileOpsHandler(files)
	uploadHandler := handler.NewUploadHandler(files)
	settingsHandler := handler.NewSettingsHandler(settings, auth, shares, versions)
	trashHandler := handler.NewTrashHandler(trash)
	versionHandler := handler.NewVersionHandler(versions)
	searchHandler := handler.NewSearchHandler(search)
	shareHandler := handler.NewShareHandler(shares)
	publicHandler := handler.NewPublicHandler(shares, files, shareRL)
	tokenHandler := handler.NewTokenHandler(tokens)
	thumbsHandler := handler.NewThumbsHandler(thumbs)
	streamHandler := handler.NewStreamHandler(probe, transcode)

	r.Use(middleware.Recovery)
	r.Use(middleware.Logging)
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.BodyLimit(1 << 20))

	r.Get("/api/health", healthHandler)

	// Auth routes (public, with optional session context for status)
	r.Route("/api/auth", func(r chi.Router) {
		r.Get("/status", optionalAuth(auth, authHandler.Status))
		r.With(rl.Middleware).Post("/login", authHandler.Login)
		r.With(rl.Middleware).Post("/setup", authHandler.Setup)
		r.With(middleware.Auth(auth, tokens), middleware.CSRF).Post("/logout", authHandler.Logout)
		r.With(middleware.Auth(auth, tokens), middleware.CSRF).Post("/change-password", settingsHandler.ChangePassword)
	})

	// Protected API routes
	r.Route("/api", func(r chi.Router) {
		r.Use(middleware.Auth(auth, tokens))
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
		r.Get("/thumbs/*", thumbsHandler.Get)
		r.Get("/stream/info/*", streamHandler.Info)
		r.Get("/stream/master/*", streamHandler.Master)
		r.Get("/stream/playlist/{v}/*", streamHandler.Playlist)
		r.Get("/stream/init/{v}/*", streamHandler.Init)
		r.Get("/stream/segment/{v}/{n}/*", streamHandler.Segment)

		r.Route("/trash", func(r chi.Router) {
			r.Get("/", trashHandler.List)
			r.Get("/count", trashHandler.Count)
			r.Post("/{id}/restore", trashHandler.Restore)
			r.Delete("/{id}", trashHandler.PermanentDelete)
			r.Delete("/", trashHandler.EmptyTrash)
		})

		r.Route("/versions", func(r chi.Router) {
			r.Get("/", versionHandler.List)
			r.Get("/count", versionHandler.Count)
			r.Post("/{id}/restore", versionHandler.Restore)
			r.Delete("/{id}", versionHandler.Delete)
		})

		r.Get("/search", searchHandler.Search)

		r.Route("/shares", func(r chi.Router) {
			r.Post("/", shareHandler.Create)
			r.Get("/", shareHandler.List)
			r.Get("/by-path", shareHandler.GetByPath)
			r.Get("/count", shareHandler.Count)
			r.Delete("/{id}", shareHandler.Delete)
		})

		r.Get("/settings", settingsHandler.GetAll)
		r.Patch("/settings", settingsHandler.Update)

		r.Route("/tokens", func(r chi.Router) {
			r.Post("/", tokenHandler.Create)
			r.Get("/", tokenHandler.List)
			r.Delete("/{id}", tokenHandler.Delete)
		})
	})

	// Public share API routes (no auth)
	r.Get("/api/public/s/{token}", publicHandler.Info)
	r.With(shareRL.Middleware).Post("/api/public/s/{token}/verify", publicHandler.Verify)
	r.Get("/api/public/s/{token}/zip", publicHandler.DownloadZip)
	r.Get("/api/public/s/{token}/raw", publicHandler.Raw)
	r.Get("/api/public/s/{token}/raw/*", publicHandler.Raw)
	r.Get("/api/public/s/{token}/dl", publicHandler.Download)
	r.Get("/api/public/s/{token}/dl/*", publicHandler.Download)

	r.NotFound(web.SPAHandler())

	// Intercept /api/upload before Chi to avoid path mangling.
	// OPTIONS pass through without auth (tus CORS preflight).
	return uploadInterceptor(auth, tokens, tus, r)
}

// uploadInterceptor routes /api/upload requests directly to tusd,
// bypassing Chi's routing which modifies URL paths.
func uploadInterceptor(auth middleware.SessionValidator, tokens middleware.TokenValidator, tus http.Handler, next http.Handler) http.Handler {
	stripped := http.StripPrefix("/api/upload/", tus)
	authed := middleware.Auth(auth, tokens)(stripped)
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
