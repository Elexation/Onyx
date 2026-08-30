package upload

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tus/tusd/v2/pkg/filelocker"
	"github.com/tus/tusd/v2/pkg/filestore"
	tusd "github.com/tus/tusd/v2/pkg/handler"

	"github.com/Elexation/onyx/internal/domain"
	"github.com/Elexation/onyx/internal/service"
)

// Client-facing finalize errors. tusd writes the error message verbatim into
// the response body, so these stay generic — details go to slog only.
var (
	errUploadConflict = tusd.NewError("ERR_UPLOAD_CONFLICT", "file already exists", http.StatusUnprocessableEntity)
	errFinalizeFailed = tusd.NewError("ERR_FINALIZE_FAILED", "could not finalize upload", http.StatusInternalServerError)
)

// TusHandler wraps tusd to provide resumable file uploads.
type TusHandler struct {
	handler  *tusd.Handler
	storedir string
	settings *service.SettingsService
}

// NewTusHandler creates a tusd handler backed by local disk storage.
// storeDir is the directory for incomplete uploads (e.g. /cache/uploads).
func NewTusHandler(storeDir string, basePath string, files *service.FileService, settings *service.SettingsService) (*TusHandler, error) {
	if err := os.MkdirAll(storeDir, 0755); err != nil {
		return nil, fmt.Errorf("create upload store dir: %w", err)
	}

	store := filestore.New(storeDir)
	locker := filelocker.New(storeDir)

	composer := tusd.NewStoreComposer()
	store.UseIn(composer)
	locker.UseIn(composer)

	h, err := tusd.NewHandler(tusd.Config{
		BasePath:      basePath,
		StoreComposer: composer,
		// Finalize synchronously, in the request goroutine, before the 204 is
		// returned. This runs up to the client's concurrency wide (not serialized
		// through one channel consumer) and lets a finalize failure propagate to
		// the client as an upload error instead of being silently swallowed.
		PreFinishResponseCallback: func(hook tusd.HookEvent) (tusd.HTTPResponse, error) {
			meta := hook.Upload.MetaData
			uploadID := hook.Upload.ID
			filename := meta["name"]
			targetDir := meta["targetDir"]
			strategy := meta["conflictStrategy"]

			// For folder uploads, relativePath includes subdirectory structure.
			relativePath := meta["relativePath"]
			if relativePath == "" {
				relativePath = filename
			}

			tusFile := filepath.Join(storeDir, uploadID)
			src, err := os.Open(tusFile)
			if err != nil {
				slog.Error("open completed upload", "id", uploadID, "error", err)
				return tusd.HTTPResponse{}, errFinalizeFailed
			}

			finalPath, ferr := files.CompleteUpload(targetDir, relativePath, strategy, src)
			src.Close()

			// Cleanup of the tus temp file is left to the client's terminate
			// (DELETE), which @uppy/tus always sends once it sees the 204 and
			// removes the file from its queue. Removing it here too would race
			// that DELETE and make it 404. The 24h sweep (cleanupStaleUploads)
			// is the backstop if the client never sends it.
			if ferr != nil {
				slog.Error("finalize upload", "id", uploadID, "file", filename, "error", ferr)
				// Sanitized errors only — raw ferr text (paths, syscall detail)
				// would be written verbatim into the response body by tusd.
				// Conflict gets a non-retryable 422 (NOT 409/5xx — tus clients
				// auto-retry those, and a conflict never resolves by retrying).
				if errors.Is(ferr, service.ErrUploadConflict) {
					return tusd.HTTPResponse{}, errUploadConflict
				}
				return tusd.HTTPResponse{}, errFinalizeFailed
			}

			slog.Info("upload complete", "file", finalPath)
			return tusd.HTTPResponse{}, nil
		},
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
			if hook.Upload.SizeIsDeferred {
				return tusd.HTTPResponse{}, tusd.FileInfoChanges{},
					tusd.NewError("ERR_UPLOAD_SIZE_UNKNOWN", "deferred-length uploads are not allowed", http.StatusBadRequest)
			}
			maxStr, _ := settings.Get(domain.SettingUploadMaxSize)
			maxSize := domain.GetInt64(maxStr)
			if maxSize > 0 && hook.Upload.Size > maxSize {
				return tusd.HTTPResponse{}, tusd.FileInfoChanges{},
					tusd.NewError("ERR_FILE_TOO_LARGE", "file exceeds maximum upload size", http.StatusRequestEntityTooLarge)
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
		settings: settings,
	}

	go th.cleanupStaleUploads()

	return th, nil
}

func (t *TusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t.handler.ServeHTTP(w, r)
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
		if !info.ModTime().Before(cutoff) {
			continue
		}
		uploadID := strings.TrimSuffix(entry.Name(), ".info")
		// Remove data and .info as a pair. Remove() on a missing file is harmless.
		os.Remove(filepath.Join(t.storedir, uploadID))
		os.Remove(filepath.Join(t.storedir, uploadID+".info"))
	}
}

// Close is a no-op placeholder for graceful shutdown.
func (t *TusHandler) Close() {
}
