package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/Elexation/onyx/internal/adapter/database"
	"github.com/Elexation/onyx/internal/domain"
	"github.com/Elexation/onyx/internal/port/http/middleware"
	"github.com/Elexation/onyx/internal/service"
)

func newChangesHandlerForTest(t *testing.T) (*ChangesHandler, *service.EventStore, *sql.DB) {
	t.Helper()
	tmp := t.TempDir()
	db, err := database.Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	es := service.NewEventStore(db)
	return NewChangesHandler(es), es, db
}

func insertBackdatedEvent(t *testing.T, db *sql.DB, eventType string, createdAt int64) int64 {
	t.Helper()
	res, err := db.Exec(
		"INSERT INTO events (type, payload, created_at) VALUES (?, ?, ?)",
		eventType, "{}", createdAt,
	)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("last id: %v", err)
	}
	return id
}

type changesBody struct {
	Cursor int64           `json:"cursor"`
	Events []service.Event `json:"events"`
	Behind bool            `json:"behind,omitempty"`
}

func decodeChanges(t *testing.T, rr *httptest.ResponseRecorder) changesBody {
	t.Helper()
	var body changesBody
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return body
}

func TestChanges_Bootstrap_NoSince_NonEmpty(t *testing.T) {
	h, es, _ := newChangesHandlerForTest(t)
	es.Record("a", struct{}{})
	es.Record("b", struct{}{})
	es.Record("c", struct{}{})
	latest, _ := es.LatestID()

	rr := httptest.NewRecorder()
	h.Get(rr, httptest.NewRequest(http.MethodGet, "/api/changes", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rr.Code)
	}
	body := decodeChanges(t, rr)
	if body.Cursor != latest {
		t.Errorf("cursor: want %d, got %d", latest, body.Cursor)
	}
	if len(body.Events) != 0 {
		t.Errorf("expected empty events on bootstrap, got %d", len(body.Events))
	}
	if body.Behind {
		t.Errorf("behind should be false on bootstrap")
	}
}

func TestChanges_Bootstrap_NoSince_EmptyDB(t *testing.T) {
	h, _, _ := newChangesHandlerForTest(t)

	rr := httptest.NewRecorder()
	h.Get(rr, httptest.NewRequest(http.MethodGet, "/api/changes", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rr.Code)
	}
	body := decodeChanges(t, rr)
	if body.Cursor != 0 {
		t.Errorf("cursor: want 0 on empty, got %d", body.Cursor)
	}
	if len(body.Events) != 0 {
		t.Errorf("expected empty events, got %d", len(body.Events))
	}
}

func TestChanges_InvalidSince_400(t *testing.T) {
	h, _, _ := newChangesHandlerForTest(t)

	cases := []string{"abc", "-1", "1.5", "0x10"}
	for _, q := range cases {
		t.Run(q, func(t *testing.T) {
			rr := httptest.NewRecorder()
			h.Get(rr, httptest.NewRequest(http.MethodGet, "/api/changes?since="+q, nil))
			if rr.Code != http.StatusBadRequest {
				t.Errorf("since=%q: want 400, got %d", q, rr.Code)
			}
		})
	}
}

func TestChanges_Since_ReturnsEvents_AdvancesCursor(t *testing.T) {
	h, es, _ := newChangesHandlerForTest(t)
	es.Record("a", struct{}{})
	es.Record("b", struct{}{})
	es.Record("c", struct{}{})

	rr := httptest.NewRecorder()
	h.Get(rr, httptest.NewRequest(http.MethodGet, "/api/changes?since=0", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	body := decodeChanges(t, rr)
	if len(body.Events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(body.Events))
	}
	for i := 1; i < len(body.Events); i++ {
		if body.Events[i].ID <= body.Events[i-1].ID {
			t.Errorf("expected id-asc, got %d then %d", body.Events[i-1].ID, body.Events[i].ID)
		}
	}
	last := body.Events[len(body.Events)-1].ID
	if body.Cursor != last {
		t.Errorf("cursor: want %d (last event), got %d", last, body.Cursor)
	}
}

func TestChanges_Since_IdleNoAdvance(t *testing.T) {
	h, es, _ := newChangesHandlerForTest(t)
	es.Record("a", struct{}{})
	es.Record("b", struct{}{})
	es.Record("c", struct{}{})
	latest, _ := es.LatestID()

	rr := httptest.NewRecorder()
	url := "/api/changes?since=" + strconv.FormatInt(latest, 10)
	h.Get(rr, httptest.NewRequest(http.MethodGet, url, nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	body := decodeChanges(t, rr)
	if len(body.Events) != 0 {
		t.Errorf("expected no new events, got %d", len(body.Events))
	}
	if body.Cursor != latest {
		t.Errorf("cursor must not roll backward: want %d, got %d", latest, body.Cursor)
	}
	if body.Behind {
		t.Errorf("behind should be false at exact cursor")
	}
}

func TestChanges_BehindFlag_AfterPrune(t *testing.T) {
	h, es, db := newChangesHandlerForTest(t)

	old := time.Now().Add(-2 * time.Hour).Unix()
	insertBackdatedEvent(t, db, "a", old)
	insertBackdatedEvent(t, db, "b", old)
	insertBackdatedEvent(t, db, "c", old)
	id4 := insertBackdatedEvent(t, db, "d", time.Now().Unix())
	id5 := insertBackdatedEvent(t, db, "e", time.Now().Unix())

	if _, err := es.Prune(1 * time.Hour); err != nil {
		t.Fatalf("prune: %v", err)
	}

	rr := httptest.NewRecorder()
	h.Get(rr, httptest.NewRequest(http.MethodGet, "/api/changes?since=2", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	body := decodeChanges(t, rr)
	if !body.Behind {
		t.Errorf("expected behind=true after prune past cursor")
	}
	if len(body.Events) != 2 {
		t.Fatalf("expected 2 surviving events, got %d", len(body.Events))
	}
	if body.Events[0].ID != id4 || body.Events[1].ID != id5 {
		t.Errorf("expected ids [%d, %d], got [%d, %d]", id4, id5, body.Events[0].ID, body.Events[1].ID)
	}
}

// stubTokenValidator approves any bearer token. CheckScope is permissive so
// the test isolates the handler-level bearer check (defense-in-depth) from
// the CheckScope block list.
type stubTokenValidator struct{}

func (stubTokenValidator) ValidateToken(token string) (*domain.PersonalAccessToken, error) {
	return &domain.PersonalAccessToken{ID: 1, Scope: "read"}, nil
}
func (stubTokenValidator) CheckScope(scope, method, path string) bool { return true }

type stubSessionValidator struct{}

func (stubSessionValidator) ValidateSession(id string) (*domain.Session, error) { return nil, nil }

func TestChanges_Bearer_403(t *testing.T) {
	h, _, _ := newChangesHandlerForTest(t)

	authed := middleware.Auth(stubSessionValidator{}, stubTokenValidator{})(http.HandlerFunc(h.Get))

	req := httptest.NewRequest(http.MethodGet, "/api/changes", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rr := httptest.NewRecorder()
	authed.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status: want 403, got %d", rr.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["error"] != "not available via bearer token" {
		t.Errorf("error message: got %q", body["error"])
	}
}
