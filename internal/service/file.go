package service

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Elexation/onyx/internal/adapter/storage"
	"github.com/Elexation/onyx/internal/domain"
)

type FileService struct {
	storage  *storage.LocalStorage
	trash    *TrashService
	versions *VersionService
	settings *SettingsService
}

func NewFileService(storage *storage.LocalStorage) *FileService {
	return &FileService{storage: storage}
}

func (s *FileService) SetTrash(trash *TrashService, settings *SettingsService) {
	s.trash = trash
	s.settings = settings
}

func (s *FileService) SetVersioning(versions *VersionService) {
	s.versions = versions
}

// ListDirectory returns the contents of a directory, optionally filtering
// hidden files and sorting directories first then by name.
func (s *FileService) ListDirectory(dirPath string, showHidden bool) ([]domain.FileInfo, error) {
	items, err := s.storage.ListDir(dirPath)
	if err != nil {
		return nil, err
	}

	if !showHidden {
		filtered := items[:0]
		for _, item := range items {
			if !strings.HasPrefix(item.Name, ".") {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})

	return items, nil
}

// GetFileInfo returns metadata for a single path.
func (s *FileService) GetFileInfo(filePath string) (*domain.FileInfo, error) {
	return s.storage.Stat(filePath)
}

// OpenFile returns a reader, mod time, and size for serving a file.
func (s *FileService) OpenFile(filePath string) (io.ReadSeekCloser, time.Time, int64, error) {
	return s.storage.Open(filePath)
}

// WriteZip streams a zip archive of the given paths to w.
func (s *FileService) WriteZip(w io.Writer, paths []string) error {
	return s.storage.WriteZip(w, paths)
}

// MakeDir creates a directory. The parent must exist and the target must not.
func (s *FileService) MakeDir(dirPath string) error {
	if dirPath == "" || dirPath == "/" {
		return fmt.Errorf("invalid directory path")
	}
	return s.storage.MakeDir(dirPath)
}

// Rename changes the name of a file or directory.
// newName must be a bare name with no path separators.
func (s *FileService) Rename(filePath, newName string) error {
	if newName == "" {
		return fmt.Errorf("new name must not be empty")
	}
	if strings.ContainsAny(newName, "/\\") {
		return fmt.Errorf("new name must not contain path separators")
	}

	// Check source exists
	info, err := s.storage.Stat(filePath)
	if err != nil {
		return err
	}

	// Check target doesn't exist
	parent := filePath[:strings.LastIndex(filePath, "/")+1]
	targetPath := parent + newName
	if _, err := s.storage.Stat(targetPath); err == nil {
		return &ConflictError{Path: targetPath}
	}

	if err := s.storage.Rename(filePath, newName); err != nil {
		return err
	}

	if s.versions != nil {
		if info.IsDir {
			if err := s.versions.RenameDirVersions(filePath, targetPath); err != nil {
				slog.Warn("rename versions for directory", "path", filePath, "error", err)
			}
		} else {
			if err := s.versions.RenameFileVersions(filePath, targetPath); err != nil {
				slog.Warn("rename versions for file", "path", filePath, "error", err)
			}
		}
	}
	return nil
}

// Move relocates paths into a destination directory.
func (s *FileService) Move(paths []string, destination string) ([]storage.OpResult, error) {
	// Validate destination is a directory
	info, err := s.storage.Stat(destination)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("destination not found: %s", destination)
		}
		return nil, err
	}
	if !info.IsDir {
		return nil, fmt.Errorf("destination is not a directory: %s", destination)
	}

	// Pre-stat each path so we can update version records after a successful
	// rename (source is gone by then).
	isDir := make(map[string]bool, len(paths))
	if s.versions != nil {
		for _, p := range paths {
			if pi, err := s.storage.Stat(p); err == nil {
				isDir[p] = pi.IsDir
			}
		}
	}

	results := s.storage.Move(paths, destination)

	if s.versions != nil {
		for i, r := range results {
			if !r.Success {
				continue
			}
			oldPath := ensureSlashPrefix(paths[i])
			base := oldPath[strings.LastIndex(oldPath, "/")+1:]
			newPath := strings.TrimRight(ensureSlashPrefix(destination), "/") + "/" + base
			if isDir[paths[i]] {
				if err := s.versions.RenameDirVersions(oldPath, newPath); err != nil {
					slog.Warn("move versions for directory", "path", oldPath, "error", err)
				}
			} else {
				if err := s.versions.RenameFileVersions(oldPath, newPath); err != nil {
					slog.Warn("move versions for file", "path", oldPath, "error", err)
				}
			}
		}
	}

	return results, nil
}

func ensureSlashPrefix(p string) string {
	if strings.HasPrefix(p, "/") {
		return p
	}
	return "/" + p
}

// Copy duplicates paths into a destination directory.
func (s *FileService) Copy(paths []string, destination string) ([]storage.OpResult, error) {
	info, err := s.storage.Stat(destination)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("destination not found: %s", destination)
		}
		return nil, err
	}
	if !info.IsDir {
		return nil, fmt.Errorf("destination is not a directory: %s", destination)
	}

	return s.storage.Copy(paths, destination), nil
}

// Delete removes files and directories. When trash is enabled and permanent
// is false, files are moved to the trash directory instead of being deleted.
func (s *FileService) Delete(paths []string, permanent bool) []storage.OpResult {
	if !permanent && s.trash != nil && s.settings != nil {
		enabled, err := s.settings.Get(domain.SettingTrashEnabled)
		if err == nil && domain.GetBool(enabled) {
			trashResults := s.trash.MoveToTrash(paths)
			results := make([]storage.OpResult, len(trashResults))
			for i, tr := range trashResults {
				results[i] = storage.OpResult{
					Path:    tr.Path,
					Success: tr.Success,
					Error:   tr.Error,
				}
			}
			return results
		}
	}
	return s.storage.Delete(paths)
}

// CheckConflicts returns the subset of paths that already exist in targetDir.
func (s *FileService) CheckConflicts(targetDir string, relativePaths []string) ([]string, error) {
	var conflicts []string
	for _, rp := range relativePaths {
		fullPath := targetDir + "/" + rp
		exists, err := s.storage.Exists(fullPath)
		if err != nil {
			return nil, err
		}
		if exists {
			conflicts = append(conflicts, rp)
		}
	}
	return conflicts, nil
}

// CompleteUpload moves an uploaded file into the data root.
// conflictStrategy: "replace" overwrites, "keepBoth" auto-renames.
// relativePath is the path relative to targetDir (supports nested dirs for folder uploads).
func (s *FileService) CompleteUpload(targetDir, relativePath, conflictStrategy string, src io.Reader) (string, error) {
	destPath := strings.TrimPrefix(targetDir+"/"+relativePath, "/")

	exists, err := s.storage.Exists(destPath)
	if err != nil {
		return "", fmt.Errorf("check existing: %w", err)
	}

	if exists {
		switch conflictStrategy {
		case "replace":
			// Version the current file before overwriting. A hard failure
			// here aborts the upload so we don't silently destroy the prior
			// content (e.g. disk full). CreateVersion returns nil for legit
			// skip cases (disabled, missing, oversized).
			if s.versions != nil {
				if err := s.versions.CreateVersion("/" + destPath); err != nil {
					return "", fmt.Errorf("create version before replace: %w", err)
				}
			}
		case "keepBoth":
			destPath = s.storage.UniqueName(destPath)
		default:
			return "", fmt.Errorf("file already exists: %s", destPath)
		}
	}

	if err := s.storage.WriteFile(destPath, src); err != nil {
		return "", fmt.Errorf("write upload: %w", err)
	}

	return "/" + destPath, nil
}

// ConflictError indicates a name collision.
type ConflictError struct {
	Path string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("a file or directory already exists at %s", e.Path)
}
