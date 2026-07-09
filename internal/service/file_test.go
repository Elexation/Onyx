package service

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Elexation/onyx/internal/adapter/storage"
	"github.com/Elexation/onyx/internal/domain"
)

// stubSettingsRepo implements SettingsRepo for testing.
type stubSettingsRepo struct {
	value string
	found bool
	err   error
}

func (s *stubSettingsRepo) Get(key string) (string, bool, error) {
	return s.value, s.found, s.err
}
func (s *stubSettingsRepo) Set(key, value string) error { return nil }
func (s *stubSettingsRepo) GetAll() (map[string]string, error) {
	return map[string]string{}, nil
}

// stubTrashRepo implements TrashRepo with no-op persistence.
type stubTrashRepo struct{}

func (s *stubTrashRepo) Insert(item *domain.TrashItem) error              { return nil }
func (s *stubTrashRepo) GetByID(id string) (*domain.TrashItem, error)     { return nil, nil }
func (s *stubTrashRepo) List() ([]domain.TrashItem, error)                { return nil, nil }
func (s *stubTrashRepo) Delete(id string) error                           { return nil }
func (s *stubTrashRepo) DeleteAll() ([]domain.TrashItem, error)           { return nil, nil }
func (s *stubTrashRepo) Count() (int, error)                              { return 0, nil }
func (s *stubTrashRepo) TotalSize() (int64, error)                        { return 0, nil }
func (s *stubTrashRepo) ListExpiredBefore(int64) ([]domain.TrashItem, error) { return nil, nil }
func (s *stubTrashRepo) ListOldestFirst() ([]domain.TrashItem, error)     { return nil, nil }

func setupDeleteTest(t *testing.T, settingsRepo *stubSettingsRepo) (*FileService, string, string) {
	t.Helper()
	tmp := t.TempDir()
	dataDir := filepath.Join(tmp, "data")
	trashDir := filepath.Join(tmp, ".trash")

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}

	ls, err := storage.NewLocalStorage(dataDir)
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	t.Cleanup(func() { _ = ls.Close() })

	settings := NewSettingsService(settingsRepo)
	trash, err := NewTrashService(&stubTrashRepo{}, settings, dataDir, trashDir)
	if err != nil {
		t.Fatalf("new trash service: %v", err)
	}

	fs := NewFileService(ls)
	fs.SetTrash(trash, settings)

	return fs, dataDir, trashDir
}

func TestDelete_SettingsError_DefaultsToTrash(t *testing.T) {
	repo := &stubSettingsRepo{err: errors.New("db connection lost")}
	fs, dataDir, trashDir := setupDeleteTest(t, repo)

	// Create a test file
	if err := os.WriteFile(filepath.Join(dataDir, "keep-me.txt"), []byte("precious"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	results := fs.Delete([]string{"/keep-me.txt"}, false)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Success {
		t.Fatalf("delete failed: %s", results[0].Error)
	}

	// File should be gone from data dir
	if _, err := os.Stat(filepath.Join(dataDir, "keep-me.txt")); !os.IsNotExist(err) {
		t.Error("file still exists in data dir after delete")
	}

	// File should be in trash dir (moved, not permanently deleted)
	entries, err := os.ReadDir(trashDir)
	if err != nil {
		t.Fatalf("read trash dir: %v", err)
	}
	if len(entries) == 0 {
		t.Error("trash dir is empty — file was permanently deleted instead of trashed")
	}
}

func TestDelete_TrashEnabled_MovesToTrash(t *testing.T) {
	repo := &stubSettingsRepo{value: "true", found: true}
	fs, dataDir, trashDir := setupDeleteTest(t, repo)

	if err := os.WriteFile(filepath.Join(dataDir, "file.txt"), []byte("data"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	results := fs.Delete([]string{"/file.txt"}, false)
	if len(results) != 1 || !results[0].Success {
		t.Fatalf("delete failed: %v", results)
	}

	if _, err := os.Stat(filepath.Join(dataDir, "file.txt")); !os.IsNotExist(err) {
		t.Error("file still in data dir")
	}

	entries, err := os.ReadDir(trashDir)
	if err != nil {
		t.Fatalf("read trash: %v", err)
	}
	if len(entries) == 0 {
		t.Error("file not found in trash")
	}
}

func TestDelete_TrashDisabled_PermanentlyDeletes(t *testing.T) {
	repo := &stubSettingsRepo{value: "false", found: true}
	fs, dataDir, _ := setupDeleteTest(t, repo)

	if err := os.WriteFile(filepath.Join(dataDir, "file.txt"), []byte("data"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	results := fs.Delete([]string{"/file.txt"}, false)
	if len(results) != 1 || !results[0].Success {
		t.Fatalf("delete failed: %v", results)
	}

	if _, err := os.Stat(filepath.Join(dataDir, "file.txt")); !os.IsNotExist(err) {
		t.Error("file still exists — should have been permanently deleted")
	}
}

func TestDelete_Permanent_IgnoresTrashSetting(t *testing.T) {
	repo := &stubSettingsRepo{value: "true", found: true}
	fs, dataDir, trashDir := setupDeleteTest(t, repo)

	if err := os.WriteFile(filepath.Join(dataDir, "file.txt"), []byte("data"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	results := fs.Delete([]string{"/file.txt"}, true)
	if len(results) != 1 || !results[0].Success {
		t.Fatalf("delete failed: %v", results)
	}

	if _, err := os.Stat(filepath.Join(dataDir, "file.txt")); !os.IsNotExist(err) {
		t.Error("file still exists")
	}

	entries, _ := os.ReadDir(trashDir)
	if len(entries) > 0 {
		t.Error("file ended up in trash despite permanent=true")
	}
}
