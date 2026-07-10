package service

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"path"
	"time"
)

const (
	EventPrunerInterval = 10 * time.Minute
	EventRetention      = 1 * time.Hour
	MaxEventsPerPoll    = 500
)

// Event is a single mutation entry in the change feed.
type Event struct {
	ID        int64           `json:"id"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt int64           `json:"createdAt"`
}

// EventRecorder is the minimal write interface services depend on.
// recordIf nil-checks before invoking so unwired services no-op safely.
type EventRecorder interface {
	Record(eventType string, payload any)
}

// EventStore appends mutation events and serves cursor-based reads.
type EventStore struct {
	db *sql.DB
}

func NewEventStore(db *sql.DB) *EventStore {
	return &EventStore{db: db}
}

// Record appends a single event. Best-effort: logs and swallows errors so a
// failed record never fails the parent mutation. Matches Onyx's
// compensation-not-transactions pattern.
func (s *EventStore) Record(eventType string, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		slog.Warn("events: marshal payload", "type", eventType, "error", err)
		return
	}
	if _, err := s.db.Exec(
		"INSERT INTO events (type, payload, created_at) VALUES (?, ?, ?)",
		eventType, string(body), time.Now().Unix(),
	); err != nil {
		slog.Warn("events: insert", "type", eventType, "error", err)
	}
}

// Since returns events with id > cursor (oldest first), capped at limit,
// alongside the current min/max event ids. Callers detect cursor staleness
// when cursor < minID-1 (events pruned past the client's position).
func (s *EventStore) Since(cursor int64, limit int) (events []Event, minID, maxID int64, err error) {
	if err := s.db.QueryRow("SELECT COALESCE(MIN(id), 0), COALESCE(MAX(id), 0) FROM events").Scan(&minID, &maxID); err != nil {
		return nil, 0, 0, fmt.Errorf("events: bounds: %w", err)
	}

	rows, err := s.db.Query(
		"SELECT id, type, payload, created_at FROM events WHERE id > ? ORDER BY id ASC LIMIT ?",
		cursor, limit,
	)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("events: query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var e Event
		var payload string
		if err := rows.Scan(&e.ID, &e.Type, &payload, &e.CreatedAt); err != nil {
			return nil, 0, 0, fmt.Errorf("events: scan: %w", err)
		}
		e.Payload = json.RawMessage(payload)
		events = append(events, e)
	}
	return events, minID, maxID, rows.Err()
}

// LatestID returns the current max event id (0 if empty). Used by the
// bootstrap path to anchor a fresh client without dumping history.
func (s *EventStore) LatestID() (int64, error) {
	var id int64
	if err := s.db.QueryRow("SELECT COALESCE(MAX(id), 0) FROM events").Scan(&id); err != nil {
		return 0, fmt.Errorf("events: latest: %w", err)
	}
	return id, nil
}

// Prune deletes events older than retainFor. AUTOINCREMENT on events.id
// prevents freed ids from being reissued, preserving cursor semantics
// across sweeps.
func (s *EventStore) Prune(retainFor time.Duration) (int64, error) {
	cutoff := time.Now().Add(-retainFor).Unix()
	res, err := s.db.Exec("DELETE FROM events WHERE created_at < ?", cutoff)
	if err != nil {
		return 0, fmt.Errorf("events: prune: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// StartPruner runs a background sweep every interval. Mirrors
// ThumbnailService.StartJanitor.
func (s *EventStore) StartPruner(interval, retainFor time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			n, err := s.Prune(retainFor)
			if err != nil {
				slog.Warn("events: prune", "error", err)
				continue
			}
			if n > 0 {
				slog.Info("events: pruned", "count", n)
			}
		}
	}()
}

// recordIf is a nil-safe Record. Services holding an EventRecorder that may
// remain nil (tests, partial wiring) get a silent no-op rather than a panic.
func recordIf(rec EventRecorder, eventType string, payload any) {
	if rec == nil {
		return
	}
	rec.Record(eventType, payload)
}

// Payload shapes — inner JSON written to events.payload. Field tags are
// lowerCamelCase to match the frontend's typed taxonomy.

type FileChangedPayload struct {
	Path       string `json:"path"`
	ParentPath string `json:"parentPath"`
	OldPath    string `json:"oldPath,omitempty"`
	Kind       string `json:"kind"` // create | update | delete | move | rename
}

type TrashChangedPayload struct {
	Kind string `json:"kind"` // add | restore | purge | auto-purge | empty | expired
	ID   string `json:"id,omitempty"`
}

type VersionChangedPayload struct {
	Kind   string `json:"kind"` // add | restore | delete | retention
	FileID int64  `json:"fileId,omitempty"`
	ID     int64  `json:"id,omitempty"`
}

type ShareChangedPayload struct {
	Kind string `json:"kind"` // create | revoke | expired
	ID   int64  `json:"id,omitempty"`
}

type TokenChangedPayload struct {
	Kind string `json:"kind"` // create | revoke | expired
	ID   int64  `json:"id,omitempty"`
}

type SettingsChangedPayload struct {
	Keys []string `json:"keys"`
}

type ThumbReadyPayload struct {
	Path string `json:"path"`
}

type SearchReindexedPayload struct{}

// parentOf returns the directory portion of p with a leading slash. Root and
// empty inputs collapse to "/".
func parentOf(p string) string {
	if p == "" || p == "/" {
		return "/"
	}
	return path.Dir(p)
}
