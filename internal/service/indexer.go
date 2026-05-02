package service

import (
	"io/fs"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Elexation/onyx/internal/adapter/database"
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
	dataDir string
	mu      sync.Mutex
}

func NewIndexer(repo SearchRepo, dataDir string) *Indexer {
	return &Indexer{repo: repo, dataDir: dataDir}
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

	fsys := os.DirFS(idx.dataDir)
	fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
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
			idx.mu.Lock()
			if err := idx.repo.UpsertBatch(batch); err != nil {
				slog.Warn("search indexer: batch upsert error", "error", err)
			}
			idx.mu.Unlock()
			count += len(batch)
			batch = batch[:0]

			if count%10000 == 0 {
				slog.Info("search indexer: progress", "files", count)
			}
		}

		return nil
	})

	if len(batch) > 0 {
		idx.mu.Lock()
		if err := idx.repo.UpsertBatch(batch); err != nil {
			slog.Warn("search indexer: batch upsert error", "error", err)
		}
		idx.mu.Unlock()
		count += len(batch)
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
