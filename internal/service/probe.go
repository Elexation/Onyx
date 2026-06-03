package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Elexation/onyx/internal/adapter/media"
	"github.com/Elexation/onyx/internal/adapter/storage"
)

// ErrNoVideoStream is re-exported from the media package so handlers can
// distinguish "file exists but has no video" from other probe failures.
var ErrNoVideoStream = media.ErrNoVideoStream

const probeCacheTTL = 1 * time.Hour

type probeEntry struct {
	info      *media.ProbeInfo
	mtime     int64
	expiresAt time.Time
}

type probeInflight struct {
	done chan struct{}
	info *media.ProbeInfo
	err  error
}

// ProbeService caches ffprobe results keyed by absolute path and mtime.
// Concurrent calls for the same file coalesce via single-flight.
type ProbeService struct {
	storage  *storage.LocalStorage
	ffmpeg   *media.FFmpeg
	dataDir  string
	realRoot string
	sema     chan struct{}

	cache    sync.Map // absPath → *probeEntry
	inflight sync.Map // absPath → *probeInflight
}

// NewProbeService wires the service. ffprobe is probed here; if missing,
// HasFFprobe returns false and Probe returns an error.
func NewProbeService(s *storage.LocalStorage, dataDir string) (*ProbeService, error) {
	realRoot, err := filepath.EvalSymlinks(dataDir)
	if err != nil {
		return nil, fmt.Errorf("resolve data dir: %w", err)
	}
	limit := runtime.NumCPU()
	if limit < 1 {
		limit = 1
	}
	return &ProbeService{
		storage:  s,
		ffmpeg:   media.Detect(),
		dataDir:  dataDir,
		realRoot: realRoot,
		sema:     make(chan struct{}, limit),
	}, nil
}

// HasFFprobe reports whether ffprobe is available on PATH.
func (ps *ProbeService) HasFFprobe() bool { return ps.ffmpeg.Available() }

// StartJanitor periodically evicts expired cache entries.
func (ps *ProbeService) StartJanitor(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			ps.sweep()
		}
	}()
}

// Probe returns video metadata for relPath. Results are cached for 1h
// and invalidated on mtime change.
func (ps *ProbeService) Probe(ctx context.Context, relPath string) (*media.ProbeInfo, error) {
	info, err := ps.storage.Stat(relPath)
	if err != nil {
		return nil, err
	}
	if info.IsDir {
		return nil, fmt.Errorf("not a file")
	}

	absPath, err := ps.resolveSafePath(relPath)
	if err != nil {
		return nil, err
	}

	if cached, ok := ps.cache.Load(absPath); ok {
		e := cached.(*probeEntry)
		if e.mtime == info.ModTime && time.Now().Before(e.expiresAt) {
			slog.Debug("probe cache hit", "path", relPath)
			return e.info, nil
		}
		ps.cache.Delete(absPath)
	}

	return ps.probeSingleFlight(ctx, absPath, info.ModTime)
}

// probeSingleFlight coalesces concurrent probes of the same file.
func (ps *ProbeService) probeSingleFlight(ctx context.Context, absPath string, mtime int64) (*media.ProbeInfo, error) {
	entry := &probeInflight{done: make(chan struct{})}
	if existing, loaded := ps.inflight.LoadOrStore(absPath, entry); loaded {
		e := existing.(*probeInflight)
		select {
		case <-e.done:
			return e.info, e.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	defer func() {
		close(entry.done)
		ps.inflight.Delete(absPath)
	}()

	select {
	case ps.sema <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-ps.sema }()

	entry.info, entry.err = ps.ffmpeg.ProbeVideo(ctx, absPath)
	if entry.err == nil {
		ps.cache.Store(absPath, &probeEntry{
			info:      entry.info,
			mtime:     mtime,
			expiresAt: time.Now().Add(probeCacheTTL),
		})
	}
	return entry.info, entry.err
}

// resolveSafePath validates relPath via the storage layer and returns an
// absolute OS path confined to the data directory.
func (ps *ProbeService) resolveSafePath(relPath string) (string, error) {
	if _, err := ps.storage.Stat(relPath); err != nil {
		return "", err
	}
	clean := strings.TrimLeft(relPath, "/")
	clean = path.Clean(clean)
	if clean == ".." || strings.HasPrefix(clean, "../") || clean == "." {
		return "", errors.New("invalid path")
	}
	osPath := filepath.Join(ps.dataDir, filepath.FromSlash(clean))
	resolved, err := filepath.EvalSymlinks(osPath)
	if err != nil {
		return "", err
	}
	root := ps.realRoot + string(filepath.Separator)
	if resolved != ps.realRoot && !strings.HasPrefix(resolved, root) {
		return "", errors.New("path escapes data directory")
	}
	return resolved, nil
}

func (ps *ProbeService) sweep() {
	var removed int
	now := time.Now()
	ps.cache.Range(func(key, value any) bool {
		e := value.(*probeEntry)
		if now.After(e.expiresAt) {
			ps.cache.Delete(key)
			removed++
		}
		return true
	})
	if removed > 0 {
		slog.Debug("probe janitor: swept", "entries", removed)
	}
}
