package upload

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// doCleanup splits retention on whether any bytes ever arrived, so every age is
// paired with both a zero-byte and a written data file. partial-fresh is the
// case that fails if the shorter cutoff leaks onto uploads with progress.
func TestDoCleanupRetentionSplit(t *testing.T) {
	dir := t.TempDir()

	write := func(name string, size int, age time.Duration) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, make([]byte, size), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		stamp := time.Now().Add(-age)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatalf("chtimes %s: %v", name, err)
		}
	}

	write("empty-fresh", 0, 10*time.Minute)
	write("empty-stale", 0, 2*time.Hour)
	write("empty-stale.info", 64, 2*time.Hour)
	write("partial-fresh", 8, 2*time.Hour)
	write("partial-stale", 8, 25*time.Hour)
	write("partial-stale.info", 64, 25*time.Hour)
	write("orphan.info", 64, 25*time.Hour)
	write("live", 8, 10*time.Minute)
	write("live.info", 64, 25*time.Hour)

	(&TusHandler{storedir: dir}).doCleanup()

	want := map[string]bool{
		"empty-fresh":        true,
		"empty-stale":        false,
		"empty-stale.info":   false,
		"partial-fresh":      true,
		"partial-stale":      false,
		"partial-stale.info": false,
		"orphan.info":        false,
		// A frozen .info mtime must not outweigh a live data sibling.
		"live":      true,
		"live.info": true,
	}
	for name, present := range want {
		_, err := os.Lstat(filepath.Join(dir, name))
		if got := err == nil; got != present {
			t.Errorf("%s: present=%v, want %v", name, got, present)
		}
	}
}
