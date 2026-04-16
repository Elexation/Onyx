package upload

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/tus/tusd/v2/pkg/filelocker"
	"github.com/tus/tusd/v2/pkg/filestore"
	tusd "github.com/tus/tusd/v2/pkg/handler"

	"github.com/Elexation/onyx/internal/service"
)

// TusHandler wraps tusd to provide resumable file uploads.
type TusHandler struct {
	handler  *tusd.Handler
	storedir string
	files    *service.FileService
}

// NewTusHandler creates a tusd handler backed by local disk storage.
// storeDir is the directory for incomplete uploads (e.g. /cache/uploads).
func NewTusHandler(storeDir string, basePath string, files *service.FileService) (*TusHandler, error) {
	if err := os.MkdirAll(storeDir, 0755); err != nil {
		return nil, fmt.Errorf("create upload store dir: %w", err)
	}

	store := filestore.New(storeDir)
	locker := filelocker.New(storeDir)

	composer := tusd.NewStoreComposer()
	store.UseIn(composer)
	locker.UseIn(composer)

	h, err := tusd.NewHandler(tusd.Config{
		BasePath:              basePath,
		StoreComposer:         composer,
		NotifyCompleteUploads: true,
		PreUploadCreateCallback: func(hook tusd.HookEvent) (tusd.HTTPResponse, tusd.FileInfoChanges, error) {
			meta := hook.Upload.MetaData
			if meta["name"] == "" {
				return tusd.HTTPResponse{}, tusd.FileInfoChanges{},
					tusd.NewError("ERR_FILENAME_REQUIRED", "filename metadata is required", http.StatusBadRequest)
			}
			if meta["targetDir"] == "" {
				return tusd.HTTPResponse{}, tusd.FileInfoChanges{},
					tusd.NewError("ERR_TARGET_REQUIRED", "targetDir metadata is required", http.StatusBadRequest)
			}
			return tusd.HTTPResponse{}, tusd.FileInfoChanges{}, nil
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create tusd handler: %w", err)
	}

	th := &TusHandler{
		handler:  h,
		storedir: storeDir,
		files:    files,
	}

	go th.processCompletedUploads()
	go th.cleanupStaleUploads()

	return th, nil
}

func (t *TusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t.handler.ServeHTTP(w, r)
}

// processCompletedUploads drains the CompleteUploads channel and moves
// finished files to their target directory in the data root.
func (t *TusHandler) processCompletedUploads() {
	for event := range t.handler.CompleteUploads {
		meta := event.Upload.MetaData
		uploadID := event.Upload.ID
		filename := meta["name"]
		targetDir := meta["targetDir"]
		strategy := meta["conflictStrategy"]

		// For folder uploads, relativePath includes subdirectory structure
		relativePath := meta["relativePath"]
		if relativePath == "" {
			relativePath = filename
		}

		tusFile := filepath.Join(t.storedir, uploadID)
		src, err := os.Open(tusFile)
		if err != nil {
			slog.Error("open completed upload", "id", uploadID, "error", err)
			continue
		}

		finalPath, err := t.files.CompleteUpload(targetDir, relativePath, strategy, src)
		src.Close()
		if err != nil {
			slog.Error("finalize upload", "id", uploadID, "file", filename, "error", err)
			continue
		}

		// Clean up tus files (.info and data)
		os.Remove(tusFile)
		os.Remove(tusFile + ".info")

		slog.Info("upload complete", "file", finalPath)
	}
}

// cleanupStaleUploads removes incomplete uploads older than 24 hours.
// Runs on startup and every hour.
func (t *TusHandler) cleanupStaleUploads() {
	t.doCleanup()
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		t.doCleanup()
	}
}

func (t *TusHandler) doCleanup() {
	cutoff := time.Now().Add(-24 * time.Hour)
	entries, err := os.ReadDir(t.storedir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			path := filepath.Join(t.storedir, entry.Name())
			if err := os.Remove(path); err == nil {
				slog.Debug("cleaned stale upload", "file", entry.Name())
			}
		}
	}
}

// Close is a no-op placeholder for graceful shutdown.
func (t *TusHandler) Close() {
}
