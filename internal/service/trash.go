package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Elexation/onyx/internal/domain"
)

type TrashRepo interface {
	Insert(item *domain.TrashItem) error
	GetByID(id string) (*domain.TrashItem, error)
	List() ([]domain.TrashItem, error)
	Delete(id string) error
	DeleteAll() ([]domain.TrashItem, error)
	Count() (int, error)
	TotalSize() (int64, error)
	ListExpiredBefore(unixTime int64) ([]domain.TrashItem, error)
	ListOldestFirst() ([]domain.TrashItem, error)
}

type TrashService struct {
	repo     TrashRepo
	settings *SettingsService
	dataDir  string
	trashDir string
	versions *VersionService
}

func NewTrashService(repo TrashRepo, settings *SettingsService, dataDir, trashDir string) (*TrashService, error) {
	if err := os.MkdirAll(trashDir, 0755); err != nil {
		return nil, fmt.Errorf("create trash directory: %w", err)
	}
	return &TrashService{
		repo:     repo,
		settings: settings,
		dataDir:  dataDir,
		trashDir: trashDir,
	}, nil
}

// SetVersioning wires the version service in after construction to avoid a
// circular init dependency with VersionService (constructed after trash).
func (s *TrashService) SetVersioning(v *VersionService) {
	s.versions = v
}

func (s *TrashService) MoveToTrash(paths []string) []MoveToTrashResult {
	results := make([]MoveToTrashResult, len(paths))
	for i, p := range paths {
		results[i] = s.moveOne(p)
	}
	return results
}

type MoveToTrashResult struct {
	Path    string `json:"path"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

func (s *TrashService) moveOne(filePath string) MoveToTrashResult {
	clean := strings.TrimLeft(filePath, "/")
	if clean == "" || clean == "." {
		return MoveToTrashResult{Path: filePath, Error: "invalid path"}
	}

	srcAbs := filepath.Join(s.dataDir, filepath.FromSlash(clean))
	info, err := os.Stat(srcAbs)
	if err != nil {
		return MoveToTrashResult{Path: filePath, Error: err.Error()}
	}

	var size int64
	if info.IsDir() {
		size, err = dirSize(srcAbs)
		if err != nil {
			return MoveToTrashResult{Path: filePath, Error: fmt.Sprintf("calculate size: %s", err)}
		}
	} else {
		size = info.Size()
	}

	id, err := generateID()
	if err != nil {
		return MoveToTrashResult{Path: filePath, Error: fmt.Sprintf("generate id: %s", err)}
	}

	trashName := id + "-" + filepath.Base(clean)
	dstAbs := filepath.Join(s.trashDir, trashName)

	if err := os.Rename(srcAbs, dstAbs); err != nil {
		if !isCrossDevice(err) {
			return MoveToTrashResult{Path: filePath, Error: fmt.Sprintf("move to trash: %s", err)}
		}
		// Cross-device: copy then delete original
		if err := copyTree(srcAbs, dstAbs); err != nil {
			os.RemoveAll(dstAbs)
			return MoveToTrashResult{Path: filePath, Error: fmt.Sprintf("copy to trash: %s", err)}
		}
		if err := os.RemoveAll(srcAbs); err != nil {
			return MoveToTrashResult{Path: filePath, Error: fmt.Sprintf("remove original after copy: %s", err)}
		}
	}

	item := &domain.TrashItem{
		ID:           id,
		OriginalPath: "/" + clean,
		TrashPath:    trashName,
		DeletedAt:    time.Now().Unix(),
		Size:         size,
		IsDir:        info.IsDir(),
	}
	if err := s.repo.Insert(item); err != nil {
		// Move back on DB failure
		os.Rename(dstAbs, srcAbs)
		return MoveToTrashResult{Path: filePath, Error: fmt.Sprintf("record trash item: %s", err)}
	}

	return MoveToTrashResult{Path: filePath, Success: true}
}

func (s *TrashService) Restore(id string) error {
	item, err := s.repo.GetByID(id)
	if err != nil {
		return fmt.Errorf("get trash item: %w", err)
	}
	if item == nil {
		return fmt.Errorf("trash item not found")
	}

	clean := strings.TrimLeft(item.OriginalPath, "/")
	dstAbs := filepath.Join(s.dataDir, filepath.FromSlash(clean))

	// Check for conflict at original path
	if _, err := os.Stat(dstAbs); err == nil {
		return fmt.Errorf("cannot restore: a file or directory already exists at %s", item.OriginalPath)
	}

	// Ensure parent directory exists
	parentDir := filepath.Dir(dstAbs)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}

	srcAbs := filepath.Join(s.trashDir, item.TrashPath)
	if err := os.Rename(srcAbs, dstAbs); err != nil {
		return fmt.Errorf("restore file: %w", err)
	}

	if err := s.repo.Delete(id); err != nil {
		// Move back to trash on DB failure
		os.Rename(dstAbs, srcAbs)
		return fmt.Errorf("remove trash record: %w", err)
	}

	return nil
}

func (s *TrashService) PermanentDelete(id string) error {
	item, err := s.repo.GetByID(id)
	if err != nil {
		return fmt.Errorf("get trash item: %w", err)
	}
	if item == nil {
		return fmt.Errorf("trash item not found")
	}

	trashAbs := filepath.Join(s.trashDir, item.TrashPath)
	if err := os.RemoveAll(trashAbs); err != nil {
		return fmt.Errorf("delete from trash: %w", err)
	}

	if err := s.repo.Delete(id); err != nil {
		return fmt.Errorf("remove trash record: %w", err)
	}

	// Cascade: drop any version history attached to this file's original
	// path. Dir items don't have per-path versions of their own, but files
	// inside the dir would — we don't walk those here because trash stores
	// the dir as an opaque blob, so version cleanup for dir contents is
	// left to retention.
	if s.versions != nil && !item.IsDir {
		if err := s.versions.DeleteAllVersions(item.OriginalPath); err != nil {
			slog.Warn("trash permanent delete: cleanup versions", "path", item.OriginalPath, "error", err)
		}
	}

	return nil
}

func (s *TrashService) EmptyTrash() error {
	items, err := s.repo.DeleteAll()
	if err != nil {
		return fmt.Errorf("empty trash records: %w", err)
	}

	for _, item := range items {
		trashAbs := filepath.Join(s.trashDir, item.TrashPath)
		if err := os.RemoveAll(trashAbs); err != nil {
			slog.Warn("failed to delete trash file", "path", item.TrashPath, "error", err)
		}
		if s.versions != nil && !item.IsDir {
			if err := s.versions.DeleteAllVersions(item.OriginalPath); err != nil {
				slog.Warn("empty trash: cleanup versions", "path", item.OriginalPath, "error", err)
			}
		}
	}

	return nil
}

func (s *TrashService) List() ([]domain.TrashItem, error) {
	return s.repo.List()
}

func (s *TrashService) Count() (int, error) {
	return s.repo.Count()
}

func (s *TrashService) AutoPurge() {
	// Age-based purge
	purgeAgeStr, err := s.settings.Get(domain.SettingTrashPurgeAge)
	if err != nil {
		slog.Warn("trash auto-purge: failed to get purge age", "error", err)
		return
	}
	purgeAge := domain.GetDuration(purgeAgeStr)
	if purgeAge > 0 {
		cutoff := time.Now().Add(-purgeAge).Unix()
		expired, err := s.repo.ListExpiredBefore(cutoff)
		if err != nil {
			slog.Warn("trash auto-purge: failed to list expired", "error", err)
		} else {
			for _, item := range expired {
				if err := s.PermanentDelete(item.ID); err != nil {
					slog.Warn("trash auto-purge: failed to delete", "id", item.ID, "error", err)
				}
			}
			if len(expired) > 0 {
				slog.Info("trash auto-purge: deleted expired items", "count", len(expired))
			}
		}
	}

	// Size-based purge
	maxSizeStr, err := s.settings.Get(domain.SettingTrashMaxSize)
	if err != nil {
		slog.Warn("trash auto-purge: failed to get max size", "error", err)
		return
	}
	maxSize := domain.GetInt64(maxSizeStr)
	if maxSize <= 0 {
		return
	}

	totalSize, err := s.repo.TotalSize()
	if err != nil {
		slog.Warn("trash auto-purge: failed to get total size", "error", err)
		return
	}
	if totalSize <= maxSize {
		return
	}

	oldest, err := s.repo.ListOldestFirst()
	if err != nil {
		slog.Warn("trash auto-purge: failed to list oldest", "error", err)
		return
	}

	deleted := 0
	for _, item := range oldest {
		if totalSize <= maxSize {
			break
		}
		if err := s.PermanentDelete(item.ID); err != nil {
			slog.Warn("trash auto-purge: failed to delete for size", "id", item.ID, "error", err)
			continue
		}
		totalSize -= item.Size
		deleted++
	}
	if deleted > 0 {
		slog.Info("trash auto-purge: deleted items for size limit", "count", deleted)
	}
}

func (s *TrashService) StartAutoPurge(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			s.AutoPurge()
		}
	}()
}

func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func isCrossDevice(err error) bool {
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return errors.Is(linkErr.Err, syscall.EXDEV)
	}
	return false
}

func copyTree(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(src, dst)
	}
	return copyFile(src, dst)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcChild := filepath.Join(src, entry.Name())
		dstChild := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(srcChild, dstChild); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcChild, dstChild); err != nil {
				return err
			}
		}
	}
	return nil
}

func dirSize(path string) (int64, error) {
	var total int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}
