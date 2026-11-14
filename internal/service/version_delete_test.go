package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Elexation/onyx/internal/adapter/database"
	"github.com/Elexation/onyx/internal/adapter/storage"
)

// A permanent delete must take the path's versions with it, or restoring one brings the content back
func TestPermanentDeleteDropsVersions(t *testing.T) {
	tmp := t.TempDir()
	dataDir := filepath.Join(tmp, "data")
	for _, d := range []string{"dir/sub", "dir2"} {
		if err := os.MkdirAll(filepath.Join(dataDir, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	db, err := database.Open(filepath.Join(tmp, "onyx.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ls, err := storage.NewLocalStorage(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ls.Close() })
	store, err := storage.NewVersionStore(dataDir, filepath.Join(tmp, ".versions"))
	if err != nil {
		t.Fatal(err)
	}
	versions := NewVersionService(database.NewVersionRepo(db), store, NewSettingsService(&stubSettingsRepo{}), dataDir)
	fs := NewFileService(ls)
	fs.SetVersioning(versions)

	files := []string{"/keys.txt", "/dir/sub/a.txt", "/dir2/b.txt"}
	for _, p := range files {
		if err := os.WriteFile(filepath.Join(dataDir, filepath.FromSlash(p)), []byte("secret"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := versions.CreateVersion(p); err != nil {
			t.Fatalf("create version of %s: %v", p, err)
		}
	}

	for _, r := range fs.Delete([]string{"/keys.txt", "/dir"}, true) {
		if !r.Success {
			t.Fatalf("delete %s: %s", r.Path, r.Error)
		}
	}

	want := map[string]int{"/keys.txt": 0, "/dir/sub/a.txt": 0, "/dir2/b.txt": 1}
	for p, n := range want {
		got, err := versions.ListVersions(p)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != n {
			t.Errorf("%s has %d versions after the delete, want %d", p, len(got), n)
		}
	}
}
