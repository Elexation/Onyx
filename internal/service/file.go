package service

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Elexation/onyx/internal/adapter/storage"
	"github.com/Elexation/onyx/internal/domain"
)

// ConflictInfo describes an existing file that collides with an incoming upload.
type ConflictInfo struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"modTime"`
}

// ShareRewriter is the share-service surface FileService needs for cascade
// hooks on rename, move, and permanent delete. Wired post-construction via
// SetShares to break the circular init order with ShareService.
type ShareRewriter interface {
	DeleteForPath(p string, isDir bool) (int64, error)
	RewritePath(oldPath, newPath string, isDir bool) (int64, error)
}

type FileService struct {
	storage  *storage.LocalStorage
	trash    *TrashService
	versions *VersionService
	settings *SettingsService
	indexer  *Indexer
	shares   ShareRewriter
	events   EventRecorder
	// Per-destination finalize locks. Finalize runs concurrently (one tus
	// request goroutine per file) and the Exists→resolve-conflict→WriteFile
	// sequence is non-atomic, so two finalizes of the SAME destination could
	// both pass the exists check (or pick the same keepBoth name) and clobber
	// each other. Keying the lock by destination keeps distinct paths — the
	// common folder-upload case — fully parallel.
	finalizeMu    sync.Mutex
	finalizeLocks map[string]*finalizeLock
}

type finalizeLock struct {
	mu   sync.Mutex
	refs int
}

// ErrUploadConflict is returned by CompleteUpload when the destination already
// exists and the client supplied no conflict strategy. Exposed so the upload
// handler can map it to a sanitized client-facing error.
var ErrUploadConflict = errors.New("file already exists")

// lockFinalize acquires the finalize lock for dest, creating it on first use.
func (s *FileService) lockFinalize(dest string) *finalizeLock {
	s.finalizeMu.Lock()
	l := s.finalizeLocks[dest]
	if l == nil {
		l = &finalizeLock{}
		s.finalizeLocks[dest] = l
	}
	l.refs++
	s.finalizeMu.Unlock()
	l.mu.Lock()
	return l
}

func (s *FileService) unlockFinalize(dest string, l *finalizeLock) {
	l.mu.Unlock()
	s.finalizeMu.Lock()
	l.refs--
	if l.refs == 0 {
		delete(s.finalizeLocks, dest)
	}
	s.finalizeMu.Unlock()
}

func NewFileService(storage *storage.LocalStorage) *FileService {
	return &FileService{storage: storage, finalizeLocks: make(map[string]*finalizeLock)}
}

func (s *FileService) SetTrash(trash *TrashService, settings *SettingsService) {
	s.trash = trash
	s.settings = settings
}

// DiskUsage reports used and total bytes for the filesystem hosting the
// data directory.
func (s *FileService) DiskUsage() (used, total uint64, err error) {
	return s.storage.DiskUsage()
}

func (s *FileService) SetVersioning(versions *VersionService) {
	s.versions = versions
}

func (s *FileService) SetIndexer(indexer *Indexer) {
	s.indexer = indexer
}

func (s *FileService) SetEvents(r EventRecorder) {
	s.events = r
}

// SetShares wires the share rewriter in after construction so cascade
// cleanup/rewrite happens on rename, move, and permanent delete.
func (s *FileService) SetShares(c ShareRewriter) {
	s.shares = c
}

// cascadeShareDelete drops shares pointing at p. Best-effort — failures
// are logged but do not propagate (the underlying delete already succeeded).
func (s *FileService) cascadeShareDelete(p string, isDir bool) {
	if s.shares == nil {
		return
	}
	if _, err := s.shares.DeleteForPath(p, isDir); err != nil {
		slog.Warn("file: cascade share delete failed", "path", p, "isDir", isDir, "error", err)
	}
}

// cascadeShareRewrite updates shares from oldPath to newPath on rename/move.
// Best-effort — failures are logged but do not propagate (the underlying
// rename already succeeded; a stale share row is preferable to a transaction
// rollback that would leave the FS state ahead of the DB).
func (s *FileService) cascadeShareRewrite(oldPath, newPath string, isDir bool) {
	if s.shares == nil {
		return
	}
	if _, err := s.shares.RewritePath(oldPath, newPath, isDir); err != nil {
		slog.Warn("file: cascade share rewrite failed", "old", oldPath, "new", newPath, "isDir", isDir, "error", err)
	}
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

// ListDirectoriesOnly returns only directories, skipping files and MIME detection.
func (s *FileService) ListDirectoriesOnly(dirPath string, showHidden bool) ([]domain.FileInfo, error) {
	items, err := s.storage.ListDirs(dirPath)
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
// MaxZipBytes caps the total bytes streamed by any single WriteZip response.
// Without a cap, a single request like paths=["/large/dir"] streams the entire
// subtree, draining bandwidth and disk I/O indefinitely. Applied to both
// admin and public-share zip endpoints.
const MaxZipBytes int64 = 50 << 30 // 50 GB

// ErrZipSizeExceeded is returned by WriteZip when MaxZipBytes is reached
// mid-stream. The response is necessarily already partially written; handlers
// should suppress generic error logging for this case (the service warns).
var ErrZipSizeExceeded = errors.New("zip size cap exceeded")

type maxBytesWriter struct {
	w   io.Writer
	n   int64
	max int64
}

func (cw *maxBytesWriter) Write(p []byte) (int, error) {
	if cw.n+int64(len(p)) > cw.max {
		return 0, ErrZipSizeExceeded
	}
	n, err := cw.w.Write(p)
	cw.n += int64(n)
	return n, err
}

func (s *FileService) WriteZip(w io.Writer, paths []string) error {
	cw := &maxBytesWriter{w: w, max: MaxZipBytes}
	err := s.storage.WriteZip(cw, paths)
	if errors.Is(err, ErrZipSizeExceeded) {
		slog.Warn("zip aborted: size cap exceeded", "max_bytes", MaxZipBytes, "written", cw.n)
		return ErrZipSizeExceeded
	}
	return err
}

// MakeDir creates a directory. The parent must exist and the target must not.
func (s *FileService) MakeDir(dirPath string) error {
	if dirPath == "" || dirPath == "/" {
		return fmt.Errorf("invalid directory path")
	}
	if err := s.storage.MakeDir(dirPath); err != nil {
		return err
	}
	full := ensureSlashPrefix(dirPath)
	if s.indexer != nil {
		s.indexer.NotifyCreated(full, true, 0, time.Now().Unix())
	}
	recordIf(s.events, "file.changed", FileChangedPayload{
		Path:       full,
		ParentPath: parentOf(full),
		Kind:       "create",
	})
	return nil
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
		// Allow case-only renames (same file on case-insensitive FS)
		if !s.storage.SameFile(filePath, targetPath) {
			return &ConflictError{Path: targetPath}
		}
	}

	if err := s.storage.Rename(filePath, newName); err != nil {
		return err
	}

	if s.indexer != nil {
		s.indexer.NotifyRenamed(ensureSlashPrefix(filePath), ensureSlashPrefix(targetPath), info.IsDir)
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
	s.cascadeShareRewrite(ensureSlashPrefix(filePath), ensureSlashPrefix(targetPath), info.IsDir)
	newFull := ensureSlashPrefix(targetPath)
	recordIf(s.events, "file.changed", FileChangedPayload{
		Path:       newFull,
		ParentPath: parentOf(newFull),
		OldPath:    ensureSlashPrefix(filePath),
		Kind:       "rename",
	})
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

	// Pre-stat each path so we can update version/index/share records after
	// a successful rename (source is gone by then).
	isDir := make(map[string]bool, len(paths))
	if s.versions != nil || s.indexer != nil || s.shares != nil {
		for _, p := range paths {
			if pi, err := s.storage.Stat(p); err == nil {
				isDir[p] = pi.IsDir
			}
		}
	}

	results := s.storage.Move(paths, destination)

	for i, r := range results {
		if !r.Success {
			continue
		}
		oldPath := ensureSlashPrefix(paths[i])
		base := oldPath[strings.LastIndex(oldPath, "/")+1:]
		newPath := strings.TrimRight(ensureSlashPrefix(destination), "/") + "/" + base
		if s.indexer != nil {
			s.indexer.NotifyMoved(oldPath, newPath, isDir[paths[i]])
		}
		if s.versions != nil {
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
		s.cascadeShareRewrite(oldPath, newPath, isDir[paths[i]])
		recordIf(s.events, "file.changed", FileChangedPayload{
			Path:       newPath,
			ParentPath: parentOf(newPath),
			OldPath:    oldPath,
			Kind:       "move",
		})
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

	results := s.storage.Copy(paths, destination)
	for i, r := range results {
		if !r.Success {
			continue
		}
		base := paths[i]
		if idx := strings.LastIndex(base, "/"); idx >= 0 {
			base = base[idx+1:]
		}
		newPath := ensureSlashPrefix(strings.TrimRight(ensureSlashPrefix(destination), "/") + "/" + base)
		if s.indexer != nil {
			if info, err := s.storage.Stat(newPath); err == nil {
				s.indexer.NotifyCopied(newPath, info.IsDir, info.Size, info.ModTime)
			}
		}
		recordIf(s.events, "file.changed", FileChangedPayload{
			Path:       newPath,
			ParentPath: parentOf(newPath),
			Kind:       "create",
		})
	}
	return results, nil
}

// Delete removes files and directories. When trash is enabled and permanent
// is false, files are moved to the trash directory instead of being deleted.
func (s *FileService) Delete(paths []string, permanent bool) []storage.OpResult {
	var results []storage.OpResult
	wentToTrash := false
	if !permanent && s.trash != nil && s.settings != nil {
		enabled, err := s.settings.Get(domain.SettingTrashEnabled)
		if err != nil || domain.GetBool(enabled) {
			if err != nil {
				slog.Warn("trash setting read failed, defaulting to trash enabled", "error", err)
			}
			trashResults := s.trash.MoveToTrash(paths)
			results = make([]storage.OpResult, len(trashResults))
			for i, tr := range trashResults {
				results[i] = storage.OpResult{
					Path:    tr.Path,
					Success: tr.Success,
					Error:   tr.Error,
				}
			}
			wentToTrash = true
		}
	}
	// Pre-stat for share cascade — only needed when bypassing trash, since
	// MoveToTrash already cascades. Source is gone by the time we cascade,
	// so isDir must be captured before the storage delete.
	var preStatIsDir map[string]bool
	if !wentToTrash && s.shares != nil {
		preStatIsDir = make(map[string]bool, len(paths))
		for _, p := range paths {
			if pi, err := s.storage.Stat(p); err == nil {
				preStatIsDir[p] = pi.IsDir
			}
		}
	}
	if results == nil {
		results = s.storage.Delete(paths)
	}
	var deleted []string
	for i, r := range results {
		if !r.Success {
			continue
		}
		full := ensureSlashPrefix(paths[i])
		deleted = append(deleted, full)
		if !wentToTrash {
			s.cascadeShareDelete(full, preStatIsDir[paths[i]])
		}
	}
	if s.indexer != nil && len(deleted) > 0 {
		s.indexer.NotifyDeleted(deleted)
	}
	for _, p := range deleted {
		recordIf(s.events, "file.changed", FileChangedPayload{
			Path:       p,
			ParentPath: parentOf(p),
			Kind:       "delete",
		})
	}
	return results
}

// CheckConflicts returns metadata for the subset of paths that already exist in targetDir.
func (s *FileService) CheckConflicts(targetDir string, relativePaths []string) ([]ConflictInfo, error) {
	conflicts := make([]ConflictInfo, 0)
	for _, rp := range relativePaths {
		fpClean := filepath.Clean(filepath.FromSlash(strings.TrimLeft(rp, "/")))
		sep := string(filepath.Separator)
		if fpClean == "" || fpClean == "." || fpClean == ".." || strings.HasPrefix(fpClean, ".."+sep) {
			return nil, fmt.Errorf("invalid relative path: %q", rp)
		}
		clean := filepath.ToSlash(fpClean)
		fullPath := path.Join(targetDir, clean)
		info, err := s.storage.Lstat(fullPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		conflicts = append(conflicts, ConflictInfo{
			Path:    rp,
			Size:    info.Size,
			ModTime: info.ModTime,
		})
	}
	return conflicts, nil
}

// CompleteUpload moves an uploaded file into the data root.
// conflictStrategy: "replace" overwrites, "keepBoth" auto-renames.
// relativePath is the path relative to targetDir (supports nested dirs for folder uploads).
func (s *FileService) CompleteUpload(targetDir, relativePath, conflictStrategy string, src io.Reader) (string, error) {
	// Reject traversal in either component before os.Root gets a chance to.
	// Bad uploads would otherwise linger in the tus store after a 500.
	fpTarget := filepath.Clean(filepath.FromSlash(strings.TrimLeft(targetDir, "/")))
	sep := string(filepath.Separator)
	if fpTarget == ".." || strings.HasPrefix(fpTarget, ".."+sep) {
		return "", fmt.Errorf("invalid target directory")
	}
	cleanTarget := filepath.ToSlash(fpTarget)
	fpRel := filepath.Clean(filepath.FromSlash(strings.TrimLeft(relativePath, "/")))
	if fpRel == "" || fpRel == "." || fpRel == ".." || strings.HasPrefix(fpRel, ".."+sep) {
		return "", fmt.Errorf("invalid upload path")
	}
	cleanRel := filepath.ToSlash(fpRel)

	destPath := path.Join(cleanTarget, cleanRel)

	// Serialize same-destination finalizes for the whole check→resolve→write
	// window. Lock key is the original destPath (evaluated now, before any
	// keepBoth rename), so the defer releases the right entry.
	l := s.lockFinalize(destPath)
	defer s.unlockFinalize(destPath, l)

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
			destPath, err = s.storage.UniqueName(destPath)
			if err != nil {
				return "", fmt.Errorf("unique name: %w", err)
			}
		default:
			return "", ErrUploadConflict
		}
	}

	if err := s.storage.WriteFile(destPath, src); err != nil {
		return "", fmt.Errorf("write upload: %w", err)
	}

	finalPath := "/" + destPath
	if s.indexer != nil {
		if info, err := s.storage.Stat(finalPath); err == nil {
			s.indexer.NotifyCreated(finalPath, false, info.Size, info.ModTime)
		}
	}
	recordIf(s.events, "file.changed", FileChangedPayload{
		Path:       finalPath,
		ParentPath: parentOf(finalPath),
		Kind:       "create",
	})
	return finalPath, nil
}

// ConflictError indicates a name collision.
type ConflictError struct {
	Path string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("a file or directory already exists at %s", e.Path)
}
