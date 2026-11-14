package storage

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func writeZipFixture(t *testing.T, dangling bool) *LocalStorage {
	t.Helper()
	dir := t.TempDir()
	folder := filepath.Join(dir, "folder")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.txt", "c.txt"} {
		if err := os.WriteFile(filepath.Join(folder, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if dangling {
		// Sorts between the two files, so the archive fails partway through
		if err := os.Symlink(filepath.Join(dir, "missing"), filepath.Join(folder, "b-dangling")); err != nil {
			t.Skipf("symlinks not available here: %v", err)
		}
	}
	ls, err := NewLocalStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ls.Close() })
	return ls
}

func TestWriteZipCompleteArchive(t *testing.T) {
	ls := writeZipFixture(t, false)
	var buf bytes.Buffer
	if err := ls.WriteZip(&buf, []string{"/folder"}, false); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("archive does not parse: %v", err)
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	if len(names) != 3 {
		t.Fatalf("entries %v, want the folder and both files", names)
	}
}

// A failure midway must not leave a valid-looking archive that silently lacks files
func TestWriteZipFailureLeavesArchiveUnsealed(t *testing.T) {
	ls := writeZipFixture(t, true)
	var buf bytes.Buffer
	if err := ls.WriteZip(&buf, []string{"/folder"}, false); err == nil {
		t.Fatal("unreadable entry did not fail the zip")
	}
	if zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len())); err == nil {
		t.Fatalf("partial archive still parses, with %d entries", len(zr.File))
	}
}
