package service

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/Elexation/onyx/internal/adapter/database"
)

func newEventStoreForTest(t *testing.T) (*EventStore, *sql.DB) {
	t.Helper()
	tmp := t.TempDir()
	db, err := database.Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewEventStore(db), db
}

// insertBackdated writes an events row with a chosen created_at so prune /
// behind-detection tests don't depend on wall-clock sleeps.
func insertBackdated(t *testing.T, db *sql.DB, eventType, payload string, createdAt int64) int64 {
	t.Helper()
	res, err := db.Exec(
		"INSERT INTO events (type, payload, created_at) VALUES (?, ?, ?)",
		eventType, payload, createdAt,
	)
	if err != nil {
		t.Fatalf("insert backdated: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}
	return id
}

func TestEventStore_Record_PersistsAllFields(t *testing.T) {
	es, _ := newEventStoreForTest(t)

	es.Record("file.changed", FileChangedPayload{Path: "/a.txt", ParentPath: "/", Kind: "create"})

	events, _, _, err := es.Since(0, 100)
	if err != nil {
		t.Fatalf("since: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	e := events[0]
	if e.ID <= 0 {
		t.Errorf("expected id > 0, got %d", e.ID)
	}
	if e.Type != "file.changed" {
		t.Errorf("type: got %q", e.Type)
	}
	if e.CreatedAt <= 0 {
		t.Errorf("createdAt: got %d", e.CreatedAt)
	}
	var got FileChangedPayload
	if err := json.Unmarshal(e.Payload, &got); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if got.Path != "/a.txt" || got.ParentPath != "/" || got.Kind != "create" {
		t.Errorf("payload roundtrip mismatch: %+v", got)
	}
}

func TestEventStore_Since_FiltersByCursor(t *testing.T) {
	es, _ := newEventStoreForTest(t)

	es.Record("a", struct{}{})
	es.Record("b", struct{}{})
	es.Record("c", struct{}{})

	all, _, _, err := es.Since(0, 100)
	if err != nil {
		t.Fatalf("since(0): %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3, got %d", len(all))
	}

	rest, _, _, err := es.Since(all[0].ID, 100)
	if err != nil {
		t.Fatalf("since(first): %v", err)
	}
	if len(rest) != 2 {
		t.Fatalf("expected 2 after first, got %d", len(rest))
	}
	if rest[0].ID <= all[0].ID {
		t.Errorf("expected ids strictly greater than cursor")
	}
}

func TestEventStore_Since_RespectsLimit(t *testing.T) {
	es, _ := newEventStoreForTest(t)

	for i := 0; i < 5; i++ {
		es.Record("x", struct{}{})
	}

	got, _, _, err := es.Since(0, 3)
	if err != nil {
		t.Fatalf("since: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 (limit cap), got %d", len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i].ID <= got[i-1].ID {
			t.Errorf("expected id-asc order, got %d then %d", got[i-1].ID, got[i].ID)
		}
	}
}

func TestEventStore_LatestID_Empty_IsZero(t *testing.T) {
	es, _ := newEventStoreForTest(t)
	id, err := es.LatestID()
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if id != 0 {
		t.Errorf("expected 0 on empty, got %d", id)
	}
}

func TestEventStore_LatestID_AfterRecord(t *testing.T) {
	es, _ := newEventStoreForTest(t)

	es.Record("a", struct{}{})
	es.Record("b", struct{}{})
	es.Record("c", struct{}{})

	all, _, _, _ := es.Since(0, 100)
	want := all[len(all)-1].ID

	got, err := es.LatestID()
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if got != want {
		t.Errorf("latest: want %d, got %d", want, got)
	}
}

func TestEventStore_Prune_DeletesByCreatedAt(t *testing.T) {
	es, db := newEventStoreForTest(t)

	old := time.Now().Add(-2 * time.Hour).Unix()
	oldID := insertBackdated(t, db, "old", "{}", old)
	es.Record("fresh", struct{}{})

	n, err := es.Prune(1 * time.Hour)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 pruned, got %d", n)
	}

	all, _, _, _ := es.Since(0, 100)
	if len(all) != 1 {
		t.Fatalf("expected 1 surviving, got %d", len(all))
	}
	if all[0].ID == oldID {
		t.Errorf("old event survived prune")
	}
	if all[0].Type != "fresh" {
		t.Errorf("expected 'fresh' to survive, got %q", all[0].Type)
	}
}

func TestEventStore_Prune_AUTOINCREMENT_NoRowidReuse(t *testing.T) {
	es, db := newEventStoreForTest(t)

	old := time.Now().Add(-2 * time.Hour).Unix()
	insertBackdated(t, db, "a", "{}", old)
	insertBackdated(t, db, "b", "{}", old)
	prevMax := insertBackdated(t, db, "c", "{}", old)

	if _, err := es.Prune(1 * time.Hour); err != nil {
		t.Fatalf("prune: %v", err)
	}

	es.Record("after-prune", struct{}{})

	latest, err := es.LatestID()
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if latest <= prevMax {
		t.Fatalf("AUTOINCREMENT broken: new id %d not greater than pre-prune max %d (rowid was reused)", latest, prevMax)
	}
}

func TestEventStore_Since_BoundsAfterPrune(t *testing.T) {
	es, db := newEventStoreForTest(t)

	old := time.Now().Add(-2 * time.Hour).Unix()
	insertBackdated(t, db, "a", "{}", old)
	insertBackdated(t, db, "b", "{}", old)
	insertBackdated(t, db, "c", "{}", old)
	id4 := insertBackdated(t, db, "d", "{}", time.Now().Unix())
	id5 := insertBackdated(t, db, "e", "{}", time.Now().Unix())

	if _, err := es.Prune(1 * time.Hour); err != nil {
		t.Fatalf("prune: %v", err)
	}

	events, minID, maxID, err := es.Since(2, 100)
	if err != nil {
		t.Fatalf("since: %v", err)
	}
	if minID != id4 {
		t.Errorf("minID: want %d, got %d", id4, minID)
	}
	if maxID != id5 {
		t.Errorf("maxID: want %d, got %d", id5, maxID)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 surviving events, got %d", len(events))
	}
	// behind condition that the handler checks
	cursor := int64(2)
	if !(cursor < minID-1) {
		t.Errorf("expected behind condition (cursor=%d < minID-1=%d) to hold", cursor, minID-1)
	}
}
