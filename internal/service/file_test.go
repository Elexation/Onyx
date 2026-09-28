package service

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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

func (s *stubTrashRepo) Insert(item *domain.TrashItem) error                 { return nil }
func (s *stubTrashRepo) GetByID(id string) (*domain.TrashItem, error)        { return nil, nil }
func (s *stubTrashRepo) List() ([]domain.TrashItem, error)                   { return nil, nil }
func (s *stubTrashRepo) Delete(id string) error                              { return nil }
func (s *stubTrashRepo) DeleteAll() ([]domain.TrashItem, error)              { return nil, nil }
func (s *stubTrashRepo) Count() (int, error)                                 { return 0, nil }
func (s *stubTrashRepo) TotalSize() (int64, error)                           { return 0, nil }
func (s *stubTrashRepo) ListExpiredBefore(int64) ([]domain.TrashItem, error) { return nil, nil }
func (s *stubTrashRepo) ListOldestFirst() ([]domain.TrashItem, error)        { return nil, nil }

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
		t.Error("trash dir is empty: file was permanently deleted instead of trashed")
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
		t.Error("file still exists: should have been permanently deleted")
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

func setupUploadTest(t *testing.T) (*FileService, string) {
	t.Helper()
	dataDir := filepath.Join(t.TempDir(), "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}
	ls, err := storage.NewLocalStorage(dataDir)
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	t.Cleanup(func() { _ = ls.Close() })
	return NewFileService(ls), dataDir
}

// uploadSrc writes an out-of-root source file standing in for a byte-complete
// tus store entry, as CompleteUpload now consumes a path, not a reader.
func uploadSrc(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "upload.bin")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write upload source: %v", err)
	}
	return p
}

func TestCompleteUpload_ReplaceOverDirectory_Fails(t *testing.T) {
	fs, dataDir := setupUploadTest(t)
	if err := os.Mkdir(filepath.Join(dataDir, "target"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	_, err := fs.CompleteUpload("/", "target", "replace", uploadSrc(t, "body"))
	if !errors.Is(err, ErrUploadIsDir) {
		t.Fatalf("want ErrUploadIsDir, got %v", err)
	}

	info, err := os.Stat(filepath.Join(dataDir, "target"))
	if err != nil || !info.IsDir() {
		t.Errorf("existing directory was clobbered: %v", err)
	}
}

func TestCompleteUpload_KeepBothOverDirectory_AutoRenames(t *testing.T) {
	fs, dataDir := setupUploadTest(t)
	if err := os.Mkdir(filepath.Join(dataDir, "target"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	got, err := fs.CompleteUpload("/", "target", "keepBoth", uploadSrc(t, "body"))
	if err != nil {
		t.Fatalf("keepBoth over directory failed: %v", err)
	}
	if got != "/target (1)" {
		t.Errorf("want /target (1), got %s", got)
	}

	data, err := os.ReadFile(filepath.Join(dataDir, "target (1)"))
	if err != nil || string(data) != "body" {
		t.Errorf("renamed file content wrong: %q, %v", data, err)
	}
}

func TestMakeDir_OverExistingFile_Fails(t *testing.T) {
	fs, dataDir := setupUploadTest(t)
	if err := os.WriteFile(filepath.Join(dataDir, "test"), []byte("keep"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	err := fs.MakeDir("/test")
	if !errors.Is(err, ErrDirBlockedByFile) {
		t.Fatalf("want ErrDirBlockedByFile, got %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dataDir, "test"))
	if err != nil || string(data) != "keep" {
		t.Errorf("blocking file was damaged: %q, %v", data, err)
	}
}

func TestMakeDir_OverExistingDir_ReportsExist(t *testing.T) {
	fs, dataDir := setupUploadTest(t)
	if err := os.Mkdir(filepath.Join(dataDir, "test"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	err := fs.MakeDir("/test")
	if errors.Is(err, ErrDirBlockedByFile) {
		t.Fatal("directory blocker misreported as a file")
	}
	if !os.IsExist(err) {
		t.Fatalf("want an exists error, got %v", err)
	}
}

func TestCheckConflicts_ReportsIsDir(t *testing.T) {
	fs, dataDir := setupUploadTest(t)
	if err := os.Mkdir(filepath.Join(dataDir, "testfolder"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "test"), []byte("body"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	conflicts, err := fs.CheckConflicts("/", []string{"testfolder", "test", "absent"})
	if err != nil {
		t.Fatalf("check conflicts: %v", err)
	}
	if len(conflicts) != 2 {
		t.Fatalf("want 2 conflicts, got %d", len(conflicts))
	}
	if !conflicts[0].IsDir {
		t.Error("directory conflict reported as a file")
	}
	if conflicts[1].IsDir {
		t.Error("file conflict reported as a directory")
	}
}

func TestCompleteUpload_ParentSegmentIsFile_Fails(t *testing.T) {
	fs, dataDir := setupUploadTest(t)
	if err := os.WriteFile(filepath.Join(dataDir, "blocker"), []byte("keep"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := fs.CompleteUpload("/", "blocker/inner.txt", "", uploadSrc(t, "body"))
	if !errors.Is(err, ErrUploadBlockedByFile) {
		t.Fatalf("want ErrUploadBlockedByFile, got %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dataDir, "blocker"))
	if err != nil || string(data) != "keep" {
		t.Errorf("blocking file was damaged: %q, %v", data, err)
	}
}

// Folder uploads land here first: the parent does not exist yet. Windows reports
// that as ERROR_PATH_NOT_FOUND, which also matches ENOTDIR, so a blocker-first
// classification rejects the whole folder. ParentSegmentIsFile above cannot catch
// it, since that case still fails once mkdirAll runs.
func TestCompleteUpload_CreatesMissingParent(t *testing.T) {
	fs, dataDir := setupUploadTest(t)

	got, err := fs.CompleteUpload("/", "newdir/inner.txt", "", uploadSrc(t, "body"))
	if err != nil {
		t.Fatalf("upload into a missing parent: %v", err)
	}
	if got != "/newdir/inner.txt" {
		t.Fatalf("path = %q, want /newdir/inner.txt", got)
	}

	data, err := os.ReadFile(filepath.Join(dataDir, "newdir", "inner.txt"))
	if err != nil || string(data) != "body" {
		t.Fatalf("file on disk = %q, err %v", data, err)
	}
}

func TestCleanUploadPaths(t *testing.T) {
	deep := strings.Repeat("a/", maxUploadPathDepth+1) + "f"

	cases := []struct {
		name      string
		targetDir string
		rel       string
		wantErr   error
	}{
		{"plain", "/docs", "notes.txt", nil},
		{"nested", "/", "photos/2024/pic.jpg", nil},
		{"root target", "/", "f.txt", nil},
		{"traversal in rel", "/", "../evil.bin", ErrUploadInvalidPath},
		{"traversal in target", "../outside", "f.txt", ErrUploadInvalidTarget},
		{"absolute rel", "/", "///etc/passwd", nil},
		{"rel is dotdot", "/", "..", ErrUploadInvalidPath},
		{"rel empty", "/", "", ErrUploadInvalidPath},
		// mkdirAll issues one syscall per segment and os.Root resolves
		// handle-relative, so nothing else bounds these.
		{"too deep", "/", deep, ErrUploadInvalidPath},
		{"segment too long", "/", strings.Repeat("x", maxUploadSegmentLen+1), ErrUploadInvalidPath},
		{"path too long", "/", strings.Repeat("ab/", maxUploadPathLen) + "f", ErrUploadInvalidPath},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := CleanUploadPaths(tc.targetDir, tc.rel)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

// Windows treats "\" as a separator and path.Clean does not, so a backslash
// traversal must not survive canonicalization into a joined filesystem path.
func TestCleanUploadPathsRejectsBackslashTraversal(t *testing.T) {
	if filepath.Separator != '\\' {
		t.Skip("backslash is a legal filename character off Windows")
	}
	if _, _, err := CleanUploadPaths("/", `..\..\evil.bin`); !errors.Is(err, ErrUploadInvalidPath) {
		t.Fatalf("err = %v, want ErrUploadInvalidPath", err)
	}
	if _, _, err := CleanUploadPaths(`..\outside`, "f.txt"); !errors.Is(err, ErrUploadInvalidTarget) {
		t.Fatalf("err = %v, want ErrUploadInvalidTarget", err)
	}
}
