package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Elexation/onyx/internal/adapter/database"
	"github.com/Elexation/onyx/internal/adapter/media"
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

	// Trash and versions dirs are CWD-relative and hardcoded; refuse to run
	// if an operator's data dir would overlap with them (e.g. ONYX_DATA=".").
	// Overlap would leak trash/version state via showHidden listings.
	trashDir := ".trash"
	versionsDir := ".versions"
	if err := ensureNoDirOverlap(dataDir, trashDir, versionsDir); err != nil {
		slog.Error("invalid directory layout", "error", err)
		os.Exit(1)
	}

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

	trashRepo := database.NewTrashRepo(db)
	trashService, err := service.NewTrashService(trashRepo, settingsService, dataDir, trashDir)
	if err != nil {
		slog.Error("trash service init failed", "error", err)
		os.Exit(1)
	}
	fileService.SetTrash(trashService, settingsService)
	trashService.StartAutoPurge(1 * time.Hour)

	versionRepo := database.NewVersionRepo(db)
	versionStore, err := storage.NewVersionStore(dataDir, versionsDir)
	if err != nil {
		slog.Error("version store init failed", "error", err)
		os.Exit(1)
	}
	versionStore.TestReflink()
	versionService := service.NewVersionService(versionRepo, versionStore, settingsService, dataDir)
	fileService.SetVersioning(versionService)
	trashService.SetVersioning(versionService)

	retentionInterval := 24 * time.Hour
	if v := os.Getenv("ONYX_VERSION_RETENTION_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			slog.Error("invalid ONYX_VERSION_RETENTION_INTERVAL", "value", v, "error", err)
			os.Exit(1)
		}
		retentionInterval = d
	}
	versionService.StartRetention(retentionInterval)

	searchRepo := database.NewSearchRepo(db)
	indexer := service.NewIndexer(searchRepo, localStorage)
	searchService := service.NewSearchService(searchRepo)
	fileService.SetIndexer(indexer)
	indexer.Start(5 * time.Minute)

	shareRepo := database.NewShareRepo(db)
	shareService := service.NewShareService(shareRepo, settingsService, fileService)
	shareService.StartCleanup(24 * time.Hour)

	tokenRepo := database.NewTokenRepo(db)
	tokenService := service.NewTokenService(tokenRepo)
	tokenService.StartCleanup(24 * time.Hour)

	thumbsDir := filepath.Join(cacheDir, "thumbs")
	thumbService, err := service.NewThumbnailService(localStorage, dataDir, thumbsDir)
	if err != nil {
		slog.Error("thumbnail service init failed", "error", err)
		os.Exit(1)
	}
	thumbService.Start()
	thumbService.StartJanitor(6 * time.Hour)

	probeService, err := service.NewProbeService(localStorage, dataDir)
	if err != nil {
		slog.Error("probe service init failed", "error", err)
		os.Exit(1)
	}
	probeService.StartJanitor(10 * time.Minute)

	hwaccelPref := env("ONYX_HWACCEL", "auto")
	maxHeight := 2160
	if v := os.Getenv("ONYX_MAX_TRANSCODE_HEIGHT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			slog.Error("invalid ONYX_MAX_TRANSCODE_HEIGHT", "value", v, "error", err)
			os.Exit(1)
		}
		switch n {
		case 0, 480, 720, 1080, 1440, 2160:
			maxHeight = n
		default:
			slog.Error("ONYX_MAX_TRANSCODE_HEIGHT must be one of 0 (unlimited), 480, 720, 1080, 1440, 2160", "value", v)
			os.Exit(1)
		}
	}

	probeCtx, probeCancel := context.WithTimeout(context.Background(), 60*time.Second)
	hwProbe := media.RunStartupProbe(probeCtx, media.Detect())
	probeCancel()

	transcodeService, err := service.NewTranscodeService(localStorage, probeService, dataDir, cacheDir, hwProbe, hwaccelPref, maxHeight)
	if err != nil {
		slog.Error("transcode service init failed", "error", err)
		os.Exit(1)
	}
	defer transcodeService.Shutdown()

	tusHandler, err := upload.NewTusHandler(
		filepath.Join(cacheDir, "uploads"),
		"/api/upload/",
		fileService,
		settingsService,
	)
	if err != nil {
		slog.Error("upload handler init failed", "error", err)
		os.Exit(1)
	}
	defer tusHandler.Close()

	trustedProxy := os.Getenv("ONYX_TRUSTED_PROXY") == "true"
	requireHTTPS := os.Getenv("ONYX_REQUIRE_HTTPS") == "true"
	router := server.NewRouter(authService, fileService, settingsService, trashService, versionService, tusHandler, searchService, shareService, tokenService, thumbService, probeService, transcodeService, trustedProxy, requireHTTPS)

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

// ensureNoDirOverlap rejects configurations where dataDir overlaps with any
// of the sibling dirs (trash, versions). Overlap includes equality, nesting,
// or path-prefix relationships after absolute-path resolution.
func ensureNoDirOverlap(dataDir string, siblings ...string) error {
	dataAbs, err := filepath.Abs(dataDir)
	if err != nil {
		return fmt.Errorf("resolve data dir: %w", err)
	}
	for _, sibling := range siblings {
		sibAbs, err := filepath.Abs(sibling)
		if err != nil {
			return fmt.Errorf("resolve %s: %w", sibling, err)
		}
		if overlaps(dataAbs, sibAbs) {
			return fmt.Errorf("data dir %q overlaps with %q", dataDir, sibling)
		}
	}
	return nil
}

func overlaps(a, b string) bool {
	if a == b {
		return true
	}
	sep := string(filepath.Separator)
	return strings.HasPrefix(a, b+sep) || strings.HasPrefix(b, a+sep)
}
