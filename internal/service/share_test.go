package service

import (
	"path/filepath"
	"testing"

	"github.com/Elexation/onyx/internal/adapter/database"
	"github.com/Elexation/onyx/internal/domain"
)

// stubSharePathChecker implements SharePathChecker against an in-memory
// set of "existing" paths. Used so share tests don't need a real storage.
type stubSharePathChecker struct {
	exists map[string]bool
}

func (s *stubSharePathChecker) GetFileInfo(p string) (*domain.FileInfo, error) {
	if s.exists[p] {
		return &domain.FileInfo{Path: p}, nil
	}
	// Match the shape ShareService.Validate's defensive sweep checks for.
	// os.PathError wrapping ErrNotExist would be more realistic, but
	// errors.Is on os.ErrNotExist passes for the bare sentinel too.
	return nil, fsNotExistErr{}
}

type fsNotExistErr struct{}

func (fsNotExistErr) Error() string { return "file does not exist" }

// Is matches errors.Is(err, os.ErrNotExist).
func (fsNotExistErr) Is(target error) bool {
	return target.Error() == "file does not exist"
}

func newShareServiceForTest(t *testing.T, exists map[string]bool) (*ShareService, *recordingEvents) {
	t.Helper()
	tmp := t.TempDir()
	db, err := database.Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	settings := NewSettingsService(&stubSettingsRepo{value: "true", found: true})
	repo := database.NewShareRepo(db)
	checker := &stubSharePathChecker{exists: exists}
	svc := NewShareService(repo, settings, checker)
	rec := &recordingEvents{}
	svc.SetEvents(rec)
	return svc, rec
}

// makeShare inserts a share row directly via the repo so tests don't need
// the create-by-token flow.
func makeShare(t *testing.T, svc *ShareService, filePath string, isDir bool) int64 {
	t.Helper()
	id, err := svc.repo.Create("hash-"+filePath, "last8xxx", filePath, isDir, 0, nil, nil)
	if err != nil {
		t.Fatalf("seed share %s: %v", filePath, err)
	}
	return id
}

func TestShare_DeleteForPath_File(t *testing.T) {
	svc, _ := newShareServiceForTest(t, nil)
	makeShare(t, svc, "/foo.txt", false)
	makeShare(t, svc, "/bar.txt", false)

	n, err := svc.DeleteForPath("/foo.txt", false)
	if err != nil {
		t.Fatalf("DeleteForPath: %v", err)
	}
	if n != 1 {
		t.Errorf("removed = %d, want 1", n)
	}

	all, _ := svc.List()
	if len(all) != 1 || all[0].FilePath != "/bar.txt" {
		t.Errorf("remaining shares = %+v, want only /bar.txt", all)
	}
}

func TestShare_DeleteForPath_DirCascadesDescendants(t *testing.T) {
	svc, _ := newShareServiceForTest(t, nil)
	makeShare(t, svc, "/proj", true)
	makeShare(t, svc, "/proj/inner.txt", false)
	makeShare(t, svc, "/proj/sub/deep.txt", false)
	makeShare(t, svc, "/other.txt", false)
	// A path that starts with "/proj" but is NOT inside it (e.g. /projector).
	// Must NOT be cascade-deleted.
	makeShare(t, svc, "/projector.txt", false)

	n, err := svc.DeleteForPath("/proj", true)
	if err != nil {
		t.Fatalf("DeleteForPath dir: %v", err)
	}
	if n != 3 {
		t.Errorf("removed = %d, want 3 (dir + 2 descendants)", n)
	}

	all, _ := svc.List()
	got := map[string]bool{}
	for _, l := range all {
		got[l.FilePath] = true
	}
	if !got["/other.txt"] || !got["/projector.txt"] || got["/proj"] {
		t.Errorf("remaining = %+v, want /other.txt and /projector.txt only", got)
	}
}

func TestShare_RewritePath_File(t *testing.T) {
	svc, _ := newShareServiceForTest(t, nil)
	makeShare(t, svc, "/old.txt", false)
	makeShare(t, svc, "/keep.txt", false)

	n, err := svc.RewritePath("/old.txt", "/new.txt", false)
	if err != nil {
		t.Fatalf("RewritePath: %v", err)
	}
	if n != 1 {
		t.Errorf("rewritten = %d, want 1", n)
	}
	link, _ := svc.repo.GetByPath("/new.txt")
	if link == nil {
		t.Fatal("expected share at /new.txt after rewrite")
	}
	if link, _ := svc.repo.GetByPath("/old.txt"); link != nil {
		t.Errorf("share still at /old.txt after rewrite")
	}
}

func TestShare_RewritePath_DirCascadesDescendants(t *testing.T) {
	svc, _ := newShareServiceForTest(t, nil)
	makeShare(t, svc, "/old", true)
	makeShare(t, svc, "/old/a.txt", false)
	makeShare(t, svc, "/old/sub/b.txt", false)
	makeShare(t, svc, "/other.txt", false)

	n, err := svc.RewritePath("/old", "/new", true)
	if err != nil {
		t.Fatalf("RewritePath dir: %v", err)
	}
	if n != 3 {
		t.Errorf("rewritten = %d, want 3", n)
	}

	all, _ := svc.List()
	paths := map[string]bool{}
	for _, l := range all {
		paths[l.FilePath] = true
	}
	want := []string{"/new", "/new/a.txt", "/new/sub/b.txt", "/other.txt"}
	for _, p := range want {
		if !paths[p] {
			t.Errorf("missing %q after dir rewrite, got %+v", p, paths)
		}
	}
	if paths["/old"] || paths["/old/a.txt"] {
		t.Errorf("old paths survived rewrite: %+v", paths)
	}
}

func TestShare_SweepOrphans_RemovesMissingFiles(t *testing.T) {
	svc, _ := newShareServiceForTest(t, map[string]bool{
		"/alive.txt":  true,
		"/alive2.txt": true,
		// /dead.txt and /also-dead.txt are NOT in the existence set.
	})
	makeShare(t, svc, "/alive.txt", false)
	makeShare(t, svc, "/dead.txt", false)
	makeShare(t, svc, "/alive2.txt", false)
	makeShare(t, svc, "/also-dead.txt", false)

	removed, err := svc.SweepOrphans()
	if err != nil {
		t.Fatalf("SweepOrphans: %v", err)
	}
	if removed != 2 {
		t.Errorf("removed = %d, want 2", removed)
	}

	all, _ := svc.List()
	if len(all) != 2 {
		t.Fatalf("expected 2 surviving shares, got %d (%+v)", len(all), all)
	}
	for _, l := range all {
		if l.FilePath != "/alive.txt" && l.FilePath != "/alive2.txt" {
			t.Errorf("unexpected survivor %q", l.FilePath)
		}
	}
}

func TestShare_Validate_DeletesOrphanAndReturnsNil(t *testing.T) {
	checker := &stubSharePathChecker{exists: map[string]bool{"/gone.txt": true}}
	svc, _ := newShareServiceForTest(t, checker.exists)
	svc.files = checker // share the same checker so we can mutate existence

	_, fullToken, err := svc.Create("/gone.txt", false, nil, "")
	if err != nil {
		t.Fatalf("create share: %v", err)
	}

	// Simulate the file being deleted out-of-band (SFTP, OS rm, etc.).
	delete(checker.exists, "/gone.txt")

	link, _, err := svc.Validate(fullToken)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if link != nil {
		t.Errorf("expected nil link for orphan share, got %+v", link)
	}

	// Row must have been deleted as a side effect.
	if l, _ := svc.repo.GetByPath("/gone.txt"); l != nil {
		t.Errorf("orphan share row still exists after Validate")
	}
}
