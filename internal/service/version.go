package service

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Elexation/onyx/internal/adapter/storage"
	"github.com/Elexation/onyx/internal/domain"
)

// Files larger than this keep at most largeFileMaxVersions versions.
const (
	largeFileThreshold   = 100 * 1024 * 1024
	largeFileMaxVersions = 5
)

type VersionRepo interface {
	Insert(v *domain.FileVersion) error
	GetByID(id int64) (*domain.FileVersion, error)
	ListByPath(filePath string) ([]domain.FileVersion, error)
	CountByPath(filePath string) (int, error)
	Delete(id int64) error
	ListAllPaths() ([]string, error)
	TotalSize() (int64, error)
	ListOldestFirst() ([]domain.FileVersion, error)
	RenameFile(oldPath, newPath string) error
	RenameDir(oldDir, newDir string) error
}

type VersionService struct {
	repo     VersionRepo
	store    *storage.VersionStore
	settings *SettingsService
	dataDir  string
}

func NewVersionService(repo VersionRepo, store *storage.VersionStore, settings *SettingsService, dataDir string) *VersionService {
	return &VersionService{
		repo:     repo,
		store:    store,
		settings: settings,
		dataDir:  dataDir,
	}
}

// CreateVersion stores a version of the file currently at filePath. filePath
// is the leading-slash data-relative path. Silently skips (returns nil) when
// versioning is disabled, the file exceeds the max size, or the file does
// not exist yet.
func (s *VersionService) CreateVersion(filePath string) error {
	enabled, err := s.settings.Get(domain.SettingVersionsEnabled)
	if err != nil {
		return fmt.Errorf("get versions enabled: %w", err)
	}
	if !domain.GetBool(enabled) {
		return nil
	}

	filePath = ensureLeadingSlash(filePath)
	relPath := strings.TrimPrefix(filePath, "/")
	srcAbs := filepath.Join(s.dataDir, filepath.FromSlash(relPath))

	info, err := os.Stat(srcAbs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat file for version: %w", err)
	}
	if info.IsDir() {
		return nil
	}

	maxSizeStr, err := s.settings.Get(domain.SettingVersionsMaxFileSize)
	if err != nil {
		return fmt.Errorf("get max file size: %w", err)
	}
	maxSize := domain.GetInt64(maxSizeStr)
	if maxSize > 0 && info.Size() > maxSize {
		slog.Debug("version skipped: file exceeds max size", "path", filePath, "size", info.Size())
		return nil
	}

	nowNs := time.Now().UnixNano()
	versionRel, err := s.store.StoreVersion(filePath, nowNs)
	if err != nil {
		return fmt.Errorf("store version: %w", err)
	}

	v := &domain.FileVersion{
		FilePath:    filePath,
		VersionPath: versionRel,
		CreatedAt:   nowNs / int64(time.Second),
		Size:        info.Size(),
	}
	if err := s.repo.Insert(v); err != nil {
		s.store.DeleteVersion(versionRel)
		return fmt.Errorf("insert version: %w", err)
	}

	// Hard cap for large files.
	if info.Size() > largeFileThreshold {
		s.enforceLargeFileCap(filePath)
	}
	return nil
}

func (s *VersionService) enforceLargeFileCap(filePath string) {
	versions, err := s.repo.ListByPath(filePath)
	if err != nil {
		slog.Warn("large-file cap: list versions", "path", filePath, "error", err)
		return
	}
	for i := largeFileMaxVersions; i < len(versions); i++ {
		if err := s.deleteVersionRow(versions[i]); err != nil {
			slog.Warn("large-file cap: delete version", "id", versions[i].ID, "error", err)
		}
	}
}

// ListVersions returns versions for a file, newest first.
func (s *VersionService) ListVersions(filePath string) ([]domain.FileVersion, error) {
	return s.repo.ListByPath(ensureLeadingSlash(filePath))
}

// RestoreVersion replaces the current file with the version identified by id,
// first creating a safety version of the current file.
func (s *VersionService) RestoreVersion(id int64) error {
	v, err := s.repo.GetByID(id)
	if err != nil {
		return fmt.Errorf("get version: %w", err)
	}
	if v == nil {
		return fmt.Errorf("version not found")
	}

	// Safety: version the current file before replacing. A hard failure
	// here aborts the restore so we don't clobber the current content with
	// no backup. CreateVersion returns nil when the current file doesn't
	// exist (legitimate: user deleted it and is restoring) or versioning
	// is disabled.
	if err := s.CreateVersion(v.FilePath); err != nil {
		return fmt.Errorf("create safety version before restore: %w", err)
	}

	if err := s.store.RestoreVersion(v.FilePath, v.VersionPath); err != nil {
		return fmt.Errorf("restore version file: %w", err)
	}
	return nil
}

// DeleteAllVersions removes every version for the given data-relative file
// path. Used when a file is permanently deleted (e.g. from trash) so its
// version history doesn't linger as orphan records.
func (s *VersionService) DeleteAllVersions(filePath string) error {
	filePath = ensureLeadingSlash(filePath)
	versions, err := s.repo.ListByPath(filePath)
	if err != nil {
		return fmt.Errorf("list versions for delete-all: %w", err)
	}
	var firstErr error
	for _, v := range versions {
		if err := s.deleteVersionRow(v); err != nil {
			slog.Warn("delete-all versions: failed", "id", v.ID, "error", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// DeleteVersion removes a single version.
func (s *VersionService) DeleteVersion(id int64) error {
	v, err := s.repo.GetByID(id)
	if err != nil {
		return fmt.Errorf("get version: %w", err)
	}
	if v == nil {
		return fmt.Errorf("version not found")
	}
	return s.deleteVersionRow(*v)
}

func (s *VersionService) deleteVersionRow(v domain.FileVersion) error {
	if err := s.store.DeleteVersion(v.VersionPath); err != nil {
		return fmt.Errorf("delete version file: %w", err)
	}
	if err := s.repo.Delete(v.ID); err != nil {
		return fmt.Errorf("delete version record: %w", err)
	}
	return nil
}

// RenameFileVersions updates version records and disk files when a file is
// renamed or moved. Both paths are leading-slash data-relative.
func (s *VersionService) RenameFileVersions(oldPath, newPath string) error {
	oldPath = ensureLeadingSlash(oldPath)
	newPath = ensureLeadingSlash(newPath)
	if err := s.store.RenameFile(oldPath, newPath); err != nil {
		return fmt.Errorf("rename version files: %w", err)
	}
	if err := s.repo.RenameFile(oldPath, newPath); err != nil {
		return fmt.Errorf("rename version records: %w", err)
	}
	return nil
}

// RenameDirVersions updates version records and disk files when a directory
// is renamed or moved. Affects all versions of files inside.
func (s *VersionService) RenameDirVersions(oldPath, newPath string) error {
	oldPath = ensureLeadingSlash(oldPath)
	newPath = ensureLeadingSlash(newPath)
	if err := s.store.RenameDir(oldPath, newPath); err != nil {
		return fmt.Errorf("rename version dir: %w", err)
	}
	if err := s.repo.RenameDir(oldPath, newPath); err != nil {
		return fmt.Errorf("rename version dir records: %w", err)
	}
	return nil
}

// ApplyRetention prunes versions according to MaxCount (per-file),
// MaxAge (global), and MaxStorageBytes (global).
func (s *VersionService) ApplyRetention() {
	maxCountStr, _ := s.settings.Get(domain.SettingVersionsMaxCount)
	maxAgeStr, _ := s.settings.Get(domain.SettingVersionsMaxAge)
	maxStorageStr, _ := s.settings.Get(domain.SettingVersionsMaxStorageBytes)

	maxCount := domain.GetInt(maxCountStr)
	maxAge := domain.GetDuration(maxAgeStr)
	maxStorage := domain.GetInt64(maxStorageStr)

	paths, err := s.repo.ListAllPaths()
	if err != nil {
		slog.Warn("retention: list paths", "error", err)
		return
	}

	cutoff := int64(0)
	if maxAge > 0 {
		cutoff = time.Now().Add(-maxAge).Unix()
	}

	perFileDeleted := 0
	for _, p := range paths {
		versions, err := s.repo.ListByPath(p)
		if err != nil {
			slog.Warn("retention: list versions", "path", p, "error", err)
			continue
		}
		for i, v := range versions {
			countExceeded := maxCount > 0 && i >= maxCount
			ageExceeded := cutoff > 0 && v.CreatedAt < cutoff
			if !countExceeded && !ageExceeded {
				continue
			}
			if err := s.deleteVersionRow(v); err != nil {
				slog.Warn("retention: delete version", "id", v.ID, "error", err)
				continue
			}
			perFileDeleted++
		}
	}
	if perFileDeleted > 0 {
		slog.Info("retention: deleted versions by per-file policy", "count", perFileDeleted)
	}

	// Global storage cap.
	if maxStorage <= 0 {
		return
	}
	total, err := s.repo.TotalSize()
	if err != nil {
		slog.Warn("retention: total size", "error", err)
		return
	}
	if total <= maxStorage {
		return
	}

	oldest, err := s.repo.ListOldestFirst()
	if err != nil {
		slog.Warn("retention: list oldest", "error", err)
		return
	}
	storageDeleted := 0
	for _, v := range oldest {
		if total <= maxStorage {
			break
		}
		if err := s.deleteVersionRow(v); err != nil {
			slog.Warn("retention: delete oldest version", "id", v.ID, "error", err)
			continue
		}
		total -= v.Size
		storageDeleted++
	}
	if storageDeleted > 0 {
		slog.Info("retention: deleted versions for storage cap", "count", storageDeleted)
	}
}

// StartRetention runs ApplyRetention on an interval in a background goroutine.
func (s *VersionService) StartRetention(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			s.ApplyRetention()
		}
	}()
}

func ensureLeadingSlash(p string) string {
	if strings.HasPrefix(p, "/") {
		return p
	}
	return "/" + p
}
