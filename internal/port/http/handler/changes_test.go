package handler

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
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

type sseMsg struct {
	ID   string
	Data string
}

// readSSEMessages reads SSE messages from a response body, returning up to
// count data-bearing messages or stopping when the body closes/errors.
func readSSEMessages(t *testing.T, resp *http.Response, count int) []sseMsg {
	t.Helper()
	type result struct {
		msgs []sseMsg
	}
	ch := make(chan result, 1)
	go func() {
		var msgs []sseMsg
		var current sseMsg
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				if current.Data != "" {
					msgs = append(msgs, current)
					current = sseMsg{}
					if len(msgs) >= count {
						ch <- result{msgs}
						return
					}
				}
				continue
			}
			if after, ok := strings.CutPrefix(line, "id: "); ok {
				current.ID = after
			} else if after, ok := strings.CutPrefix(line, "data: "); ok {
				current.Data = after
			}
		}
		ch <- result{msgs}
	}()

	select {
	case r := <-ch:
		return r.msgs
	case <-time.After(3 * time.Second):
		t.Fatal("timeout reading SSE messages")
		return nil
	}
}

// sseGet makes a GET request to the test server with optional Last-Event-ID
// and a context timeout. Caller must close resp.Body.
func sseGet(t *testing.T, srv *httptest.Server, lastEventID string, timeout time.Duration) *http.Response {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)

	req, err := http.NewRequestWithContext(ctx, "GET", srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if lastEventID != "" {
		req.Header.Set("Last-Event-ID", lastEventID)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestChanges_SSE_ContentType(t *testing.T) {
	h, _, _ := newChangesHandlerForTest(t)
	srv := httptest.NewServer(http.HandlerFunc(h.Get))
	defer srv.Close()

	resp := sseGet(t, srv, "", 2*time.Second)
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type: want text/event-stream, got %q", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control: want no-cache, got %q", cc)
	}
	if xa := resp.Header.Get("X-Accel-Buffering"); xa != "no" {
		t.Errorf("X-Accel-Buffering: want no, got %q", xa)
	}
}

func TestChanges_SSE_StreamsEvents(t *testing.T) {
	h, es, _ := newChangesHandlerForTest(t)
	es.Record("file.changed", map[string]string{"path": "/a.txt", "parentPath": "/", "kind": "create"})
	es.Record("file.changed", map[string]string{"path": "/b.txt", "parentPath": "/", "kind": "create"})
	es.Record("thumb.ready", map[string]string{"path": "/a.txt"})

	srv := httptest.NewServer(http.HandlerFunc(h.Get))
	defer srv.Close()

	resp := sseGet(t, srv, "0", 3*time.Second)
	defer resp.Body.Close()

	msgs := readSSEMessages(t, resp, 3)
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}

	for _, m := range msgs {
		if m.ID == "" {
			t.Error("expected id on event message")
		}
		var ev sseEvent
		if err := json.Unmarshal([]byte(m.Data), &ev); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if ev.Type != "file.changed" && ev.Type != "thumb.ready" {
			t.Errorf("unexpected type: %q", ev.Type)
		}
	}

	if msgs[2].ID <= msgs[0].ID {
		t.Errorf("ids should be ascending: first=%s last=%s", msgs[0].ID, msgs[2].ID)
	}
}

func TestChanges_SSE_CursorResume(t *testing.T) {
	h, es, _ := newChangesHandlerForTest(t)
	es.Record("a", struct{}{})
	es.Record("b", struct{}{})
	es.Record("c", struct{}{})
	latest, _ := es.LatestID()

	srv := httptest.NewServer(http.HandlerFunc(h.Get))
	defer srv.Close()

	// Resume from second-to-last: should only get the last event.
	resumeFrom := latest - 1
	resp := sseGet(t, srv, strconv.FormatInt(resumeFrom, 10), 3*time.Second)
	defer resp.Body.Close()

	msgs := readSSEMessages(t, resp, 1)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	var ev sseEvent
	if err := json.Unmarshal([]byte(msgs[0].Data), &ev); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ev.Type != "c" {
		t.Errorf("expected type 'c', got %q", ev.Type)
	}
}

func TestChanges_SSE_Bootstrap_SkipsExisting(t *testing.T) {
	h, es, db := newChangesHandlerForTest(t)
	es.Record("old", struct{}{})
	es.Record("old", struct{}{})

	srv := httptest.NewServer(http.HandlerFunc(h.Get))
	defer srv.Close()

	// No Last-Event-ID: bootstrap anchors at latest. Insert a new event
	// after connection to verify only new events stream.
	resp := sseGet(t, srv, "", 3*time.Second)
	defer resp.Body.Close()

	// Give the handler time to bootstrap and start its loop.
	time.Sleep(200 * time.Millisecond)
	insertBackdatedEvent(t, db, "new", time.Now().Unix())

	msgs := readSSEMessages(t, resp, 1)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message (only the new event), got %d", len(msgs))
	}
	var ev sseEvent
	if err := json.Unmarshal([]byte(msgs[0].Data), &ev); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ev.Type != "new" {
		t.Errorf("expected type 'new', got %q", ev.Type)
	}
}

func TestChanges_SSE_Behind(t *testing.T) {
	h, es, db := newChangesHandlerForTest(t)

	old := time.Now().Add(-2 * time.Hour).Unix()
	insertBackdatedEvent(t, db, "a", old)
	insertBackdatedEvent(t, db, "b", old)
	insertBackdatedEvent(t, db, "c", old)
	insertBackdatedEvent(t, db, "d", time.Now().Unix())
	insertBackdatedEvent(t, db, "e", time.Now().Unix())

	if _, err := es.Prune(1 * time.Hour); err != nil {
		t.Fatalf("prune: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(h.Get))
	defer srv.Close()

	// Cursor at 2: events 1-3 are pruned, so cursor is behind.
	resp := sseGet(t, srv, "2", 3*time.Second)
	defer resp.Body.Close()

	// Expect: 1 behind message + 2 event messages (d and e).
	msgs := readSSEMessages(t, resp, 3)
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages (behind + 2 events), got %d", len(msgs))
	}

	// First message should be the behind signal (no id).
	if msgs[0].ID != "" {
		t.Errorf("behind message should have no id, got %q", msgs[0].ID)
	}
	var behind sseEvent
	if err := json.Unmarshal([]byte(msgs[0].Data), &behind); err != nil {
		t.Fatalf("unmarshal behind: %v", err)
	}
	if behind.Type != "behind" {
		t.Errorf("first message type: want 'behind', got %q", behind.Type)
	}

	// Remaining messages should be the surviving events.
	for _, m := range msgs[1:] {
		if m.ID == "" {
			t.Error("event message should have an id")
		}
	}
}

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
