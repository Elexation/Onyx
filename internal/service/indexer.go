package service

import (
	"io/fs"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/Elexation/onyx/internal/adapter/database"
	"github.com/Elexation/onyx/internal/adapter/storage"
	"github.com/Elexation/onyx/internal/domain"
)

type SearchRepo interface {
	Upsert(name, path string, isDir bool, size, modTime int64) error
	UpsertBatch(items []database.FileEntry) error
	Delete(path string) error
	DeleteTree(path string) error
	Search(query string, limit int) ([]domain.SearchResult, int, error)
	DeleteStale(olderThan int64) (int64, error)
	UpdatePath(oldPath, newPath, newName string) error
	UpdatePathPrefix(oldPrefix, newPrefix string) error
}

type Indexer struct {
	repo    SearchRepo
	storage *storage.LocalStorage
	mu      sync.Mutex
}

func NewIndexer(repo SearchRepo, st *storage.LocalStorage) *Indexer {
	return &Indexer{repo: repo, storage: st}
}

func (idx *Indexer) Start(interval time.Duration) {
	go func() {
		slog.Info("search indexer: starting full scan")
		start := time.Now()
		count := idx.scan()
		slog.Info("search indexer: full scan complete", "files", count, "duration", time.Since(start).Round(time.Millisecond))

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			idx.scan()
		}
	}()
}

func (idx *Indexer) scan() int {
	scanStart := time.Now().Unix()
	var batch []database.FileEntry
	var count int

	fsys := idx.storage.FS()
	_ = fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			slog.Warn("search indexer: walk error", "path", path, "error", err)
			return nil
		}
		if path == "." {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			slog.Warn("search indexer: stat error", "path", path, "error", err)
			return nil
		}

		// Normalize to forward-slash with leading slash (matches domain.FileInfo.Path)
		normalized := "/" + strings.ReplaceAll(path, "\\", "/")

		batch = append(batch, database.FileEntry{
			Name:    d.Name(),
			Path:    normalized,
			IsDir:   d.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().Unix(),
		})

		if len(batch) >= 500 {
			n := idx.flushBatch(fsys, batch)
			prev := count
			count += n
			batch = batch[:0]

			if count/10000 > prev/10000 {
				slog.Info("search indexer: progress", "files", count)
			}
		}

		return nil
	})

	if len(batch) > 0 {
		count += idx.flushBatch(fsys, batch)
	}

	// Remove entries for files that no longer exist
	idx.mu.Lock()
	removed, err := idx.repo.DeleteStale(scanStart)
	idx.mu.Unlock()
	if err != nil {
		slog.Warn("search indexer: stale cleanup error", "error", err)
	} else if removed > 0 {
		slog.Info("search indexer: removed stale entries", "count", removed)
	}

	return count
}

// flushBatch re-stats each queued entry under idx.mu and drops any that
// no longer exist on disk, so that a concurrent Notify* delete or rename
// cannot be undone by a stale observation buffered during the walk.
func (idx *Indexer) flushBatch(fsys fs.FS, batch []database.FileEntry) int {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	verified := make([]database.FileEntry, 0, len(batch))
	for _, entry := range batch {
		rel := strings.TrimPrefix(entry.Path, "/")
		info, err := fs.Stat(fsys, rel)
		if err != nil {
			continue
		}
		verified = append(verified, database.FileEntry{
			Name:    entry.Name,
			Path:    entry.Path,
			IsDir:   info.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().Unix(),
		})
	}

	if len(verified) == 0 {
		return 0
	}
	if err := idx.repo.UpsertBatch(verified); err != nil {
		slog.Warn("search indexer: batch upsert error", "error", err)
		return 0
	}
	return len(verified)
}

func (idx *Indexer) NotifyCreated(path string, isDir bool, size, modTime int64) {
	name := path[strings.LastIndex(path, "/")+1:]
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if err := idx.repo.Upsert(name, path, isDir, size, modTime); err != nil {
		slog.Warn("search indexer: notify created error", "path", path, "error", err)
	}
}

func (idx *Indexer) NotifyRenamed(oldPath, newPath string, isDir bool) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if isDir {
		if err := idx.repo.UpdatePathPrefix(oldPath, newPath); err != nil {
			slog.Warn("search indexer: notify renamed dir error", "old", oldPath, "new", newPath, "error", err)
		}
	} else {
		newName := newPath[strings.LastIndex(newPath, "/")+1:]
		if err := idx.repo.UpdatePath(oldPath, newPath, newName); err != nil {
			slog.Warn("search indexer: notify renamed error", "old", oldPath, "new", newPath, "error", err)
		}
	}
}

func (idx *Indexer) NotifyMoved(oldPath, newPath string, isDir bool) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if isDir {
		if err := idx.repo.UpdatePathPrefix(oldPath, newPath); err != nil {
			slog.Warn("search indexer: notify moved dir error", "old", oldPath, "new", newPath, "error", err)
		}
	} else {
		newName := newPath[strings.LastIndex(newPath, "/")+1:]
		if err := idx.repo.UpdatePath(oldPath, newPath, newName); err != nil {
			slog.Warn("search indexer: notify moved error", "old", oldPath, "new", newPath, "error", err)
		}
	}
}

func (idx *Indexer) NotifyDeleted(paths []string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	for _, p := range paths {
		if err := idx.repo.DeleteTree(p); err != nil {
			slog.Warn("search indexer: notify deleted error", "path", p, "error", err)
		}
	}
}

func (idx *Indexer) NotifyCopied(path string, isDir bool, size, modTime int64) {
	name := path[strings.LastIndex(path, "/")+1:]
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if err := idx.repo.Upsert(name, path, isDir, size, modTime); err != nil {
		slog.Warn("search indexer: notify copied error", "path", path, "error", err)
	}
}
