package main

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Elexation/onyx/internal/adapter/database"
	"github.com/Elexation/onyx/internal/adapter/storage"
	"github.com/Elexation/onyx/internal/adapter/upload"
	server "github.com/Elexation/onyx/internal/port/http"
	"github.com/Elexation/onyx/internal/service"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	port := env("ONYX_PORT", "8080")
	dataDir := env("ONYX_DATA", "data")
	configDir := env("ONYX_CONFIG", "config")
	cacheDir := env("ONYX_CACHE", ".cache")

	db, err := database.Open(filepath.Join(configDir, "onyx.db"))
	if err != nil {
		slog.Error("database init failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	settingsRepo := database.NewSettingsRepo(db)
	settingsService := service.NewSettingsService(settingsRepo)

	userRepo := database.NewUserRepo(db)
	sessionRepo := database.NewSessionRepo(db)
	authService := service.NewAuthService(userRepo, sessionRepo, settingsService)
	authService.StartCleanup(10 * time.Minute)

	localStorage, err := storage.NewLocalStorage(dataDir)
	if err != nil {
		slog.Error("storage init failed", "error", err)
		os.Exit(1)
	}
	defer localStorage.Close()
	fileService := service.NewFileService(localStorage)

	trashDir := ".trash"
	trashRepo := database.NewTrashRepo(db)
	trashService, err := service.NewTrashService(trashRepo, settingsService, dataDir, trashDir)
	if err != nil {
		slog.Error("trash service init failed", "error", err)
		os.Exit(1)
	}
	fileService.SetTrash(trashService, settingsService)
	trashService.StartAutoPurge(1 * time.Hour)

	tusHandler, err := upload.NewTusHandler(
		filepath.Join(cacheDir, "uploads"),
		"/api/upload/",
		fileService,
	)
	if err != nil {
		slog.Error("upload handler init failed", "error", err)
		os.Exit(1)
	}
	defer tusHandler.Close()

	router := server.NewRouter(authService, fileService, settingsService, trashService, tusHandler)

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
