package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Elexation/onyx/internal/port/http/middleware"
	"github.com/Elexation/onyx/internal/service"
)

const (
	ssePollInterval      = 500 * time.Millisecond
	sseHeartbeatInterval = 30 * time.Second
	sseWriteTimeout      = 10 * time.Second // per-write deadline; a non-reading client must not wedge the goroutine
	sseRetryMs           = 5000
)

type ChangesHandler struct {
	events *service.EventStore
}

func NewChangesHandler(events *service.EventStore) *ChangesHandler {
	return &ChangesHandler{events: events}
}

type sseEvent struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Get handles GET /api/changes as a Server-Sent Events stream.
//
// Cursor recovery: reads Last-Event-ID header (set automatically by
// EventSource on reconnect) to resume from the client's last seen event.
// Without it, bootstraps from the current latest event id so new
// connections don't replay retained history.
//
// PAT exclusion: bearer-authenticated requests are rejected as
// defense-in-depth; the primary gate is CheckScope's admin block list.
func (h *ChangesHandler) Get(w http.ResponseWriter, r *http.Request) {
	if middleware.IsBearerAuth(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not available via bearer token"})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming not supported"})
		return
	}

	cursor := int64(-1)
	if lastID := r.Header.Get("Last-Event-ID"); lastID != "" {
		if parsed, err := strconv.ParseInt(lastID, 10, 64); err == nil && parsed >= 0 {
			cursor = parsed
		}
	}
	if cursor < 0 {
		latest, err := h.events.LatestID()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read latest event"})
			return
		}
		cursor = latest
	}

	rc := http.NewResponseController(w)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	_ = rc.SetWriteDeadline(time.Now().Add(sseWriteTimeout))
	if _, err := fmt.Fprintf(w, "retry: %d\n\n", sseRetryMs); err != nil {
		return
	}
	flusher.Flush()

	poll := time.NewTicker(ssePollInterval)
	defer poll.Stop()
	heartbeat := time.NewTicker(sseHeartbeatInterval)
	defer heartbeat.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			_ = rc.SetWriteDeadline(time.Now().Add(sseWriteTimeout))
			if _, err := fmt.Fprintf(w, ": heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-poll.C:
			events, minID, maxID, err := h.events.Since(cursor, service.MaxEventsPerPoll)
			if err != nil {
				slog.Warn("changes: poll failed", "error", err)
				continue
			}

			_ = rc.SetWriteDeadline(time.Now().Add(sseWriteTimeout))
			needsFlush := false

			if minID > 0 && cursor < minID-1 {
				if _, err := fmt.Fprintf(w, "data: {\"type\":\"behind\"}\n\n"); err != nil {
					return
				}
				needsFlush = true
			}

			for _, ev := range events {
				data, err := json.Marshal(sseEvent{Type: ev.Type, Payload: ev.Payload})
				if err != nil {
					continue
				}
				if _, err := fmt.Fprintf(w, "id: %d\ndata: %s\n\n", ev.ID, data); err != nil {
					return
				}
				cursor = ev.ID
				needsFlush = true
			}

			if len(events) == 0 && maxID > cursor {
				cursor = maxID
			}

			if needsFlush {
				flusher.Flush()
			}
		}
	}
}
