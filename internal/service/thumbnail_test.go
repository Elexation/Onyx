package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Elexation/onyx/internal/adapter/storage"
)

func newTestThumbService(t *testing.T) (*ThumbnailService, *storage.ThumbStore) {
	t.Helper()
	store, err := storage.NewThumbStore(t.TempDir())
	if err != nil {
		t.Fatalf("new thumb store: %v", err)
	}
	return &ThumbnailService{
		store:   store,
		failTTL: time.Minute,
		lruTTL:  time.Minute,
	}, store
}

func writeCacheFile(t *testing.T, dir, name string, mtime time.Time) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	if err := os.Chtimes(p, mtime, mtime); err != nil {
		t.Fatalf("chtimes %s: %v", p, err)
	}
	return p
}

func TestSweep_RemovesStaleFailMarkers(t *testing.T) {
	ts, store := newTestThumbService(t)
	shard := filepath.Join(store.Root(), "ab")

	stale := writeCacheFile(t, shard, "stale.fail", time.Now().Add(-time.Hour))
	fresh := writeCacheFile(t, shard, "fresh.fail", time.Now())

	ts.sweep()

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale fail marker still exists: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("fresh fail marker was incorrectly removed: %v", err)
	}
}

func TestSweep_RemovesStaleThumbnails(t *testing.T) {
	ts, store := newTestThumbService(t)
	shard := filepath.Join(store.Root(), "cd")

	stale := writeCacheFile(t, shard, "stale.jpg", time.Now().Add(-time.Hour))
	fresh := writeCacheFile(t, shard, "fresh.jpg", time.Now())

	ts.sweep()

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale thumbnail still exists: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("fresh thumbnail was incorrectly removed: %v", err)
	}
}

// TestLookup_QueueOverflowKeepsInflightClean covers case 45: when the jobs
// channel is full, Lookup must still return StatusQueued (graceful
// degradation) and must not leave a dangling inflight marker; otherwise
// subsequent retries could never re-enqueue the work.
func TestLookup_QueueOverflowKeepsInflightClean(t *testing.T) {
	tmp := t.TempDir()
	dataDir := filepath.Join(tmp, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}
	// Write a tiny valid JPEG so storage.Stat returns image/jpeg.
	jpegBytes := []byte{
		0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00,
		0x01, 0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00,
		0xFF, 0xD9,
	}
	if err := os.WriteFile(filepath.Join(dataDir, "photo.jpg"), jpegBytes, 0o644); err != nil {
		t.Fatalf("write jpg: %v", err)
	}

	ls, err := storage.NewLocalStorage(dataDir)
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	t.Cleanup(func() { _ = ls.Close() })
	store, err := storage.NewThumbStore(filepath.Join(tmp, "cache"))
	if err != nil {
		t.Fatalf("new thumb store: %v", err)
	}
	realRoot, err := filepath.EvalSymlinks(dataDir)
	if err != nil {
		t.Fatalf("eval symlinks: %v", err)
	}

	ts := &ThumbnailService{
		storage:  ls,
		store:    store,
		dataDir:  dataDir,
		realRoot: realRoot,
		jobs:     make(chan thumbJob, 1), // deliberately tiny
		workers:  0,                       // no worker pool, channel stays full
		failTTL:  time.Minute,
		lruTTL:   time.Minute,
	}

	// Fill the jobs channel so the next enqueue hits the default branch.
	ts.jobs <- thumbJob{relPath: "/filler", size: ThumbMedium}

	result, err := ts.Lookup("/photo.jpg", ThumbMedium)
	if err != nil {
		t.Fatalf("Lookup returned error: %v", err)
	}
	if result.Status != StatusQueued {
		t.Errorf("expected StatusQueued under queue pressure, got %v", result.Status)
	}

	// The default branch must delete the inflight marker for this key so a
	// retry can try again. Count inflight entries: should be zero.
	inflightCount := 0
	ts.inflight.Range(func(_, _ any) bool {
		inflightCount++
		return true
	})
	if inflightCount != 0 {
		t.Errorf("expected inflight map to be clean after overflow, got %d entries", inflightCount)
	}
}
