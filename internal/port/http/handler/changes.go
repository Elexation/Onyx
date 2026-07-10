package handler

import (
	"net/http"
	"strconv"

	"github.com/Elexation/onyx/internal/port/http/middleware"
	"github.com/Elexation/onyx/internal/service"
)

type ChangesHandler struct {
	events *service.EventStore
}

func NewChangesHandler(events *service.EventStore) *ChangesHandler {
	return &ChangesHandler{events: events}
}

type changesResponse struct {
	Cursor int64           `json:"cursor"`
	Events []service.Event `json:"events"`
	Behind bool            `json:"behind,omitempty"`
}

// Get handles GET /api/changes?since={cursor}.
//
// Bootstrap (no since): returns {cursor: latestID, events: []} so the next
// poll has an anchor without dumping retained history.
//
// Normal (since=N): returns events with id > N (oldest first, capped at
// MaxEventsPerPoll). When the client cursor is older than the oldest
// retained event (cursor < minID-1), sets behind: true so the page knows
// to refetch its slice.
//
// PAT exclusion: bearer-authenticated requests are rejected here as
// defense-in-depth; the primary gate is CheckScope's admin block list.
func (h *ChangesHandler) Get(w http.ResponseWriter, r *http.Request) {
	if middleware.IsBearerAuth(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not available via bearer token"})
		return
	}

	sinceParam := r.URL.Query().Get("since")
	if sinceParam == "" {
		latest, err := h.events.LatestID()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read latest event"})
			return
		}
		writeJSON(w, http.StatusOK, changesResponse{Cursor: latest, Events: []service.Event{}})
		return
	}

	cursor, err := strconv.ParseInt(sinceParam, 10, 64)
	if err != nil || cursor < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "since must be a non-negative integer"})
		return
	}

	events, minID, maxID, err := h.events.Since(cursor, service.MaxEventsPerPoll)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read events"})
		return
	}
	if events == nil {
		events = []service.Event{}
	}

	nextCursor := cursor
	if len(events) > 0 {
		nextCursor = events[len(events)-1].ID
	} else if maxID > nextCursor {
		nextCursor = maxID
	}

	resp := changesResponse{Cursor: nextCursor, Events: events}
	if minID > 0 && cursor < minID-1 {
		resp.Behind = true
	}
	writeJSON(w, http.StatusOK, resp)
}
