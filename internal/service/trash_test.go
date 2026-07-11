package service

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Elexation/onyx/internal/domain"
)

// fakeTrashRepo is a stateful in-memory TrashRepo for tests.
type fakeTrashRepo struct {
	mu    sync.Mutex
	items map[string]domain.TrashItem
}

func newFakeTrashRepo() *fakeTrashRepo {
	return &fakeTrashRepo{items: map[string]domain.TrashItem{}}
}

func (r *fakeTrashRepo) Insert(item *domain.TrashItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[item.ID] = *item
	return nil
}

func (r *fakeTrashRepo) GetByID(id string) (*domain.TrashItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if item, ok := r.items[id]; ok {
		copy := item
		return &copy, nil
	}
	return nil, nil
}

func (r *fakeTrashRepo) List() ([]domain.TrashItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.TrashItem, 0, len(r.items))
	for _, item := range r.items {
		out = append(out, item)
	}
	return out, nil
}

func (r *fakeTrashRepo) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.items, id)
	return nil
}

func (r *fakeTrashRepo) DeleteAll() ([]domain.TrashItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.TrashItem, 0, len(r.items))
	for _, item := range r.items {
		out = append(out, item)
	}
	r.items = map[string]domain.TrashItem{}
	return out, nil
}

func (r *fakeTrashRepo) Count() (int, error)                                  { r.mu.Lock(); defer r.mu.Unlock(); return len(r.items), nil }
func (r *fakeTrashRepo) TotalSize() (int64, error)                            { return 0, nil }
func (r *fakeTrashRepo) ListExpiredBefore(int64) ([]domain.TrashItem, error)  { return nil, nil }
func (r *fakeTrashRepo) ListOldestFirst() ([]domain.TrashItem, error)         { return nil, nil }

// fakeShareCleaner records share-cascade calls for assertion.
type fakeShareCleaner struct {
	mu    sync.Mutex
	calls []shareCleanCall
}

type shareCleanCall struct {
	Path  string
	IsDir bool
}

func (c *fakeShareCleaner) DeleteForPath(p string, isDir bool) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, shareCleanCall{Path: p, IsDir: isDir})
	return 1, nil
}

func (c *fakeShareCleaner) snapshot() []shareCleanCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]shareCleanCall, len(c.calls))
	copy(out, c.calls)
	return out
}

// recordingEvents captures emitted events for assertions.
type recordingEvents struct {
	mu     sync.Mutex
	events []recordedEvent
}

type recordedEvent struct {
	Type    string
	Payload any
}

func (r *recordingEvents) Record(eventType string, payload any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, recordedEvent{Type: eventType, Payload: payload})
}

func (r *recordingEvents) byType(t string) []recordedEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []recordedEvent
	for _, e := range r.events {
		if e.Type == t {
			out = append(out, e)
		}
	}
	return out
}

func setupTrashTest(t *testing.T) (*TrashService, string, string, *fakeTrashRepo, *recordingEvents) {
	t.Helper()
	tmp := t.TempDir()
	dataDir := filepath.Join(tmp, "data")
	trashDir := filepath.Join(tmp, ".trash")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}
	repo := newFakeTrashRepo()
	settings := NewSettingsService(&stubSettingsRepo{})
	trash, err := NewTrashService(repo, settings, dataDir, trashDir)
	if err != nil {
		t.Fatalf("new trash service: %v", err)
	}
	rec := &recordingEvents{}
	trash.SetEvents(rec)
	return trash, dataDir, trashDir, repo, rec
}

// trashFile writes content to dataDir/name then moves it to trash. Returns
// the resulting trash item id.
func trashFile(t *testing.T, trash *TrashService, dataDir, name string, content []byte) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dataDir, name), content, 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	results := trash.MoveToTrash([]string{"/" + name})
	if len(results) != 1 || !results[0].Success {
		t.Fatalf("move to trash failed: %+v", results)
	}
	items, _ := trash.List()
	for _, item := range items {
		if item.OriginalPath == "/"+name {
			return item.ID
		}
	}
	t.Fatalf("trashed item not found in repo")
	return ""
}

func TestRestore_KeepBoth_SingleConflict(t *testing.T) {
	trash, dataDir, _, _, _ := setupTrashTest(t)

	id := trashFile(t, trash, dataDir, "foo.txt", []byte("v1"))

	// Recreate conflicting file at the same path.
	if err := os.WriteFile(filepath.Join(dataDir, "foo.txt"), []byte("v2"), 0o644); err != nil {
		t.Fatalf("recreate: %v", err)
	}

	got, err := trash.Restore(id, "keepBoth")
	if err != nil {
		t.Fatalf("restore keepBoth: %v", err)
	}
	if got != "/foo (1).txt" {
		t.Errorf("expected restored path /foo (1).txt, got %q", got)
	}

	// Both files must exist with the right contents.
	if b, _ := os.ReadFile(filepath.Join(dataDir, "foo.txt")); string(b) != "v2" {
		t.Errorf("original foo.txt clobbered: got %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(dataDir, "foo (1).txt")); string(b) != "v1" {
		t.Errorf("restored foo (1).txt wrong content: got %q", b)
	}
}

func TestRestore_KeepBoth_Chained(t *testing.T) {
	trash, dataDir, _, _, _ := setupTrashTest(t)

	id := trashFile(t, trash, dataDir, "foo.txt", []byte("v1"))

	if err := os.WriteFile(filepath.Join(dataDir, "foo.txt"), []byte("v2"), 0o644); err != nil {
		t.Fatalf("recreate foo.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "foo (1).txt"), []byte("manual"), 0o644); err != nil {
		t.Fatalf("create foo (1).txt: %v", err)
	}

	got, err := trash.Restore(id, "keepBoth")
	if err != nil {
		t.Fatalf("restore keepBoth: %v", err)
	}
	if got != "/foo (2).txt" {
		t.Errorf("expected /foo (2).txt, got %q", got)
	}
	if b, _ := os.ReadFile(filepath.Join(dataDir, "foo (2).txt")); string(b) != "v1" {
		t.Errorf("restored foo (2).txt wrong content: got %q", b)
	}
}

func TestRestore_Replace_DisplacesExistingToTrash(t *testing.T) {
	trash, dataDir, _, repo, rec := setupTrashTest(t)

	id := trashFile(t, trash, dataDir, "foo.txt", []byte("trashed-v1"))

	if err := os.WriteFile(filepath.Join(dataDir, "foo.txt"), []byte("existing-v2"), 0o644); err != nil {
		t.Fatalf("recreate: %v", err)
	}

	got, err := trash.Restore(id, "replace")
	if err != nil {
		t.Fatalf("restore replace: %v", err)
	}
	if got != "/foo.txt" {
		t.Errorf("expected /foo.txt, got %q", got)
	}

	// On disk: trashed content is now at the original path.
	if b, _ := os.ReadFile(filepath.Join(dataDir, "foo.txt")); string(b) != "trashed-v1" {
		t.Errorf("expected restored content at /foo.txt, got %q", b)
	}

	// Repo: original id deleted, displaced item inserted with same OriginalPath.
	items, _ := repo.List()
	if len(items) != 1 {
		t.Fatalf("expected 1 trash item after replace, got %d", len(items))
	}
	if items[0].OriginalPath != "/foo.txt" {
		t.Errorf("displaced item OriginalPath = %q, want /foo.txt", items[0].OriginalPath)
	}
	if items[0].ID == id {
		t.Errorf("displaced item kept original id; should have new id")
	}

	// Events: trash.changed:add (from displaced moveOne), trash.changed:restore (from successful restore),
	// file.changed:create (final restored path).
	addEvents := rec.byType("trash.changed")
	hasAdd := false
	hasRestore := false
	for _, e := range addEvents {
		p := e.Payload.(TrashChangedPayload)
		if p.Kind == "add" {
			hasAdd = true
		}
		if p.Kind == "restore" {
			hasRestore = true
		}
	}
	if !hasAdd {
		t.Error("expected trash.changed:add for displaced item")
	}
	if !hasRestore {
		t.Error("expected trash.changed:restore for restored item")
	}
}

func TestRestore_Skip_NoOp(t *testing.T) {
	trash, dataDir, _, repo, _ := setupTrashTest(t)

	id := trashFile(t, trash, dataDir, "foo.txt", []byte("v1"))

	if err := os.WriteFile(filepath.Join(dataDir, "foo.txt"), []byte("v2"), 0o644); err != nil {
		t.Fatalf("recreate: %v", err)
	}

	got, err := trash.Restore(id, "skip")
	if err != nil {
		t.Fatalf("restore skip: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty path on skip, got %q", got)
	}

	// Disk and repo unchanged.
	if b, _ := os.ReadFile(filepath.Join(dataDir, "foo.txt")); string(b) != "v2" {
		t.Errorf("dataDir/foo.txt mutated by skip: got %q", b)
	}
	if item, _ := repo.GetByID(id); item == nil {
		t.Error("trash record removed despite skip")
	}
}

func TestRestore_DefaultStrategy_ReturnsConflictError(t *testing.T) {
	trash, dataDir, _, _, _ := setupTrashTest(t)

	id := trashFile(t, trash, dataDir, "foo.txt", []byte("v1"))
	if err := os.WriteFile(filepath.Join(dataDir, "foo.txt"), []byte("v2"), 0o644); err != nil {
		t.Fatalf("recreate: %v", err)
	}

	_, err := trash.Restore(id, "")
	if err == nil {
		t.Fatal("expected error on default-strategy conflict")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected back-compat error message, got %q", err.Error())
	}
}

func TestMoveToTrash_CascadesShareDelete_File(t *testing.T) {
	trash, dataDir, _, _, _ := setupTrashTest(t)
	cleaner := &fakeShareCleaner{}
	trash.SetShares(cleaner)

	if err := os.WriteFile(filepath.Join(dataDir, "shared.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	results := trash.MoveToTrash([]string{"/shared.txt"})
	if !results[0].Success {
		t.Fatalf("trash move failed: %+v", results[0])
	}

	calls := cleaner.snapshot()
	if len(calls) != 1 {
		t.Fatalf("expected 1 share cleanup call, got %d", len(calls))
	}
	if calls[0].Path != "/shared.txt" || calls[0].IsDir {
		t.Errorf("call = %+v, want {/shared.txt false}", calls[0])
	}
}

func TestMoveToTrash_CascadesShareDelete_Dir(t *testing.T) {
	trash, dataDir, _, _, _ := setupTrashTest(t)
	cleaner := &fakeShareCleaner{}
	trash.SetShares(cleaner)

	if err := os.MkdirAll(filepath.Join(dataDir, "shared-dir"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	results := trash.MoveToTrash([]string{"/shared-dir"})
	if !results[0].Success {
		t.Fatalf("trash move failed: %+v", results[0])
	}

	calls := cleaner.snapshot()
	if len(calls) != 1 || calls[0].Path != "/shared-dir" || !calls[0].IsDir {
		t.Errorf("calls = %+v, want one {/shared-dir true}", calls)
	}
}

func TestMoveToTrash_NoSharesWired_NoOp(t *testing.T) {
	// SetShares(nil) is the default; the cascade path must be nil-safe.
	trash, dataDir, _, _, _ := setupTrashTest(t)

	if err := os.WriteFile(filepath.Join(dataDir, "ok.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	results := trash.MoveToTrash([]string{"/ok.txt"})
	if !results[0].Success {
		t.Fatalf("trash move failed: %+v", results[0])
	}
}

func TestEmptyTrash_CascadesShareDelete(t *testing.T) {
	trash, dataDir, _, _, _ := setupTrashTest(t)
	cleaner := &fakeShareCleaner{}
	trash.SetShares(cleaner)

	_ = trashFile(t, trash, dataDir, "a.txt", []byte("a"))
	_ = trashFile(t, trash, dataDir, "b.txt", []byte("b"))
	// Reset call log so we only assert against the EmptyTrash path.
	cleaner.calls = nil

	if err := trash.EmptyTrash(); err != nil {
		t.Fatalf("empty trash: %v", err)
	}

	calls := cleaner.snapshot()
	if len(calls) != 2 {
		t.Fatalf("expected 2 cascade calls during empty, got %d (%+v)", len(calls), calls)
	}
	gotPaths := map[string]bool{calls[0].Path: true, calls[1].Path: true}
	if !gotPaths["/a.txt"] || !gotPaths["/b.txt"] {
		t.Errorf("expected cascade for /a.txt and /b.txt, got %+v", calls)
	}
}

func TestPermanentDelete_CascadesShareDelete(t *testing.T) {
	trash, dataDir, _, _, _ := setupTrashTest(t)
	cleaner := &fakeShareCleaner{}
	trash.SetShares(cleaner)

	id := trashFile(t, trash, dataDir, "perm.txt", []byte("x"))
	cleaner.calls = nil

	if err := trash.PermanentDelete(id); err != nil {
		t.Fatalf("permanent delete: %v", err)
	}
	calls := cleaner.snapshot()
	if len(calls) != 1 || calls[0].Path != "/perm.txt" {
		t.Errorf("expected one cascade for /perm.txt, got %+v", calls)
	}
}

func TestCheckRestoreConflicts_FiltersNonConflicts(t *testing.T) {
	trash, dataDir, _, _, _ := setupTrashTest(t)

	conflictID := trashFile(t, trash, dataDir, "conflict.txt", []byte("v1"))
	clearID := trashFile(t, trash, dataDir, "clear.txt", []byte("v1"))

	// Recreate only the conflicting one.
	if err := os.WriteFile(filepath.Join(dataDir, "conflict.txt"), []byte("v2-much-larger-content"), 0o644); err != nil {
		t.Fatalf("recreate conflict: %v", err)
	}

	conflicts, err := trash.CheckRestoreConflicts([]string{conflictID, clearID})
	if err != nil {
		t.Fatalf("check conflicts: %v", err)
	}
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	c := conflicts[0]
	if c.ID != conflictID {
		t.Errorf("wrong conflict id: got %q, want %q", c.ID, conflictID)
	}
	if c.Path != "/conflict.txt" {
		t.Errorf("wrong path: got %q", c.Path)
	}
	if c.Existing.Size <= c.Restoring.Size {
		t.Errorf("expected existing.size > restoring.size, got existing=%d restoring=%d", c.Existing.Size, c.Restoring.Size)
	}
	if c.Restoring.ModTime == 0 {
		t.Error("restoring.modTime not populated from trash blob")
	}
}
