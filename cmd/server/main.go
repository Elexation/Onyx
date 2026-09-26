package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
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
	"github.com/Elexation/onyx/internal/domain"
	server "github.com/Elexation/onyx/internal/port/http"
	"github.com/Elexation/onyx/internal/port/http/middleware"
	"github.com/Elexation/onyx/internal/service"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

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

	eventStore := service.NewEventStore(db)
	eventStore.StartPruner(service.EventPrunerInterval, service.EventRetention)
	settingsService.SetEvents(eventStore)

	port, envOverrides := resolveListenPort(settingsService)

	tlsEnabled, certFile, keyFile, tlsOverrides := resolveTLS(settingsService, configDir)
	for k, v := range tlsOverrides {
		envOverrides[k] = v
	}
	manualCert := os.Getenv("ONYX_TLS_CERT") != ""

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
	fileService.SetEvents(eventStore)

	trashRepo := database.NewTrashRepo(db)
	trashService, err := service.NewTrashService(trashRepo, settingsService, dataDir, trashDir)
	if err != nil {
		slog.Error("trash service init failed", "error", err)
		os.Exit(1)
	}
	trashService.SetEvents(eventStore)
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
	versionService.SetEvents(eventStore)
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
	indexer.SetEvents(eventStore)
	searchService := service.NewSearchService(searchRepo)
	fileService.SetIndexer(indexer)
	indexer.Start(5 * time.Minute)

	shareRepo := database.NewShareRepo(db)
	shareService := service.NewShareService(shareRepo, settingsService, fileService)
	shareService.SetEvents(eventStore)
	fileService.SetShares(shareService)
	trashService.SetShares(shareService)
	if n, err := shareService.SweepOrphans(); err != nil {
		slog.Warn("share orphan sweep failed", "error", err)
	} else if n > 0 {
		slog.Info("share orphan sweep removed dead links", "count", n)
	}
	shareService.StartCleanup(24 * time.Hour)

	tokenRepo := database.NewTokenRepo(db)
	tokenService := service.NewTokenService(tokenRepo)
	tokenService.SetEvents(eventStore)
	tokenService.StartCleanup(24 * time.Hour)

	thumbsDir := filepath.Join(cacheDir, "thumbs")
	thumbService, err := service.NewThumbnailService(localStorage, dataDir, thumbsDir)
	if err != nil {
		slog.Error("thumbnail service init failed", "error", err)
		os.Exit(1)
	}
	thumbService.SetEvents(eventStore)
	thumbService.Start()
	thumbService.StartJanitor(6 * time.Hour)

	probeService, err := service.NewProbeService(localStorage, dataDir)
	if err != nil {
		slog.Error("probe service init failed", "error", err)
		os.Exit(1)
	}
	probeService.StartJanitor(10 * time.Minute)
	defer probeService.Shutdown()

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

	trustedProxy := os.Getenv("ONYX_TRUSTED_PROXY") == "true"
	requireHTTPS := os.Getenv("ONYX_REQUIRE_HTTPS") == "true"

	uploadStageDir := filepath.Join(cacheDir, "uploads")
	tusHandler, err := upload.NewTusHandler(
		uploadStageDir,
		"/api/upload/",
		fileService,
		settingsService,
		trustedProxy,
	)
	if err != nil {
		slog.Error("upload handler init failed", "error", err)
		os.Exit(1)
	}
	defer tusHandler.Close()

	// NewTusHandler has created the staging dir by now, so the probe can run.
	if err := localStorage.CheckHardlink(uploadStageDir); err != nil {
		slog.Warn("upload staging cannot hardlink into the data dir, so every upload will be copied in full on finalize; put the cache and data dirs under one mount to avoid it",
			"error", err, "cache", cacheDir, "data", dataDir)
	}

	canonicalDomain := os.Getenv("ONYX_DOMAIN")
	if canonicalDomain != "" {
		if strings.Contains(canonicalDomain, "://") || strings.Contains(canonicalDomain, "/") || strings.Contains(canonicalDomain, ":") {
			slog.Error("ONYX_DOMAIN must be a bare hostname (e.g. onyx.example.com)")
			os.Exit(1)
		}
	}

	httpsRedirect := os.Getenv("ONYX_HTTPS_REDIRECT") == "true"
	httpsRedirectPort := env("ONYX_HTTPS_REDIRECT_PORT", "80")
	if httpsRedirect {
		if !tlsEnabled {
			slog.Warn("ONYX_HTTPS_REDIRECT=true has no effect without HTTPS enabled (ONYX_HTTPS=true), skipping")
			httpsRedirect = false
		} else {
			n, err := strconv.Atoi(httpsRedirectPort)
			if err != nil || n < 1 || n > 65535 {
				slog.Error("ONYX_HTTPS_REDIRECT_PORT must be a valid port (1-65535)", "value", httpsRedirectPort)
				os.Exit(1)
			}
		}
	}

	router := server.NewRouter(authService, fileService, settingsService, trashService, versionService, tusHandler, searchService, shareService, tokenService, thumbService, probeService, transcodeService, eventStore, trustedProxy, requireHTTPS, tlsEnabled, port, envOverrides)

	var handler http.Handler = router
	if canonicalDomain != "" {
		handler = middleware.DomainCanon(canonicalDomain, tlsEnabled, trustedProxy, port)(handler)
	}

	slog.Info("starting server", "port", port, "tls", tlsEnabled, "domain", canonicalDomain, "envOverrides", envOverrides)
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	if httpsRedirect {
		go startHTTPRedirectListener(httpsRedirectPort, port, canonicalDomain, trustedProxy)
	}

	if tlsEnabled {
		if !manualCert {
			if err := ensureSelfSignedCert(certFile, keyFile); err != nil {
				slog.Error("TLS cert generation failed", "error", err)
				os.Exit(1)
			}
		}
		tlsCfg, err := buildTLSConfig(certFile, keyFile)
		if err != nil {
			slog.Error("TLS config failed", "error", err)
			os.Exit(1)
		}
		slog.Info("TLS enabled", "cert", certFile, "fingerprint", certFingerprint(certFile))
		ln, err := net.Listen("tcp", srv.Addr)
		if err != nil {
			slog.Error("listen failed", "error", err)
			os.Exit(1)
		}
		if err := srv.Serve(newTLSRedirectListener(ln, tlsCfg, port, canonicalDomain, trustedProxy)); err != nil {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	} else {
		if err := srv.ListenAndServe(); err != nil {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// resolveListenPort resolves the HTTP listen port via env -> DB -> default,
// and returns a map of env-overridden setting keys -> reason for the UI to
// lock fields the operator can't actually change at runtime.
//
// "docker" reason: in-container listen port is fixed by the docker-compose
// port mapping (host:container). Changing the in-container port would silently
// break access since compose maps to :8080 by default.
func resolveListenPort(settings *service.SettingsService) (string, map[string]string) {
	envOverrides := map[string]string{}

	if v := os.Getenv("ONYX_PORT"); v != "" {
		envOverrides[domain.SettingServerListenPort] = "ONYX_PORT"
		return v, envOverrides
	}

	if _, err := os.Stat("/.dockerenv"); err == nil {
		envOverrides[domain.SettingServerListenPort] = "docker"
		return "8080", envOverrides
	}

	stored, err := settings.Get(domain.SettingServerListenPort)
	if err != nil || stored == "" {
		return "8080", envOverrides
	}
	n, perr := strconv.Atoi(stored)
	if perr != nil || n < 1024 || n > 65535 {
		slog.Warn("invalid stored listen port, using default", "value", stored)
		return "8080", envOverrides
	}
	return stored, envOverrides
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
