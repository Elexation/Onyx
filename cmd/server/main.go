package main

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Elexation/onyx/internal/adapter/database"
	server "github.com/Elexation/onyx/internal/port/http"
	"github.com/Elexation/onyx/internal/service"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	port := env("ONYX_PORT", "8080")
	configDir := env("ONYX_CONFIG", "config")

	db, err := database.Open(filepath.Join(configDir, "onyx.db"))
	if err != nil {
		slog.Error("database init failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	settingsRepo := database.NewSettingsRepo(db)
	_ = service.NewSettingsService(settingsRepo)

	router := server.NewRouter(logger)

	slog.Info("starting server", "port", port)
	if err := http.ListenAndServe(":"+port, router); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
