package server

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Elexation/onyx/internal/port/http/middleware"
	"github.com/Elexation/onyx/web"
)

func NewRouter(logger *slog.Logger) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.Recovery)
	r.Use(middleware.Logging)
	r.Use(middleware.SecurityHeaders)

	r.Get("/api/health", healthHandler)

	r.NotFound(web.SPAHandler())

	return r
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}
