package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/Elexation/onyx/internal/domain"
	"github.com/Elexation/onyx/internal/service"
)

type TrashHandler struct {
	trash *service.TrashService
}

func NewTrashHandler(trash *service.TrashService) *TrashHandler {
	return &TrashHandler{trash: trash}
}

// List handles GET /api/trash
func (h *TrashHandler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.trash.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list trash"})
		return
	}
	if items == nil {
		items = []domain.TrashItem{}
	}

	count, _ := h.trash.Count()
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "count": count})
}

// Restore handles POST /api/trash/{id}/restore?strategy=replace|keepBoth|skip
func (h *TrashHandler) Restore(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	strategy := r.URL.Query().Get("strategy")
	switch strategy {
	case "", "replace", "keepBoth", "skip":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid strategy"})
		return
	}

	path, err := h.trash.Restore(id, strategy)
	if err != nil {
		// Conflict path stays 409 for back-compat (the conflict message is
		// already generic and contains no internal paths).
		if strings.HasPrefix(err.Error(), "cannot restore: a file or directory already exists") {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		// Generic 500: wrapped errors include absolute FS paths and OS
		// errno text we don't want to leak (CLAUDE.md: "writeJSON(500,
		// err.Error()) leaks service-layer details").
		slog.Warn("trash restore failed", "id", id, "strategy", strategy, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to restore"})
		return
	}

	if strategy == "skip" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "skipped"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "restored", "path": path})
}

// CheckRestoreConflicts handles POST /api/trash/check-restore-conflicts
func (h *TrashHandler) CheckRestoreConflicts(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	// Cap the per-request id count: each dir id triggers a recursive
	// dirSize walk, so an unbounded array is a disk-I/O DoS vector even
	// behind admin auth (stolen session/PAT). 500 mirrors the spirit of
	// the existing /api/download/zip 1000-paths cap.
	if len(body.IDs) > 500 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "too many ids (max 500)"})
		return
	}
	conflicts, err := h.trash.CheckRestoreConflicts(body.IDs)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to check conflicts"})
		return
	}
	if conflicts == nil {
		conflicts = []service.RestoreConflict{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"conflicts": conflicts})
}

// PermanentDelete handles DELETE /api/trash/{id}
func (h *TrashHandler) PermanentDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	if err := h.trash.PermanentDelete(id); err != nil {
		slog.Warn("trash permanent delete failed", "id", id, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// EmptyTrash handles DELETE /api/trash
func (h *TrashHandler) EmptyTrash(w http.ResponseWriter, r *http.Request) {
	if err := h.trash.EmptyTrash(); err != nil {
		slog.Warn("trash empty failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to empty trash"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "emptied"})
}

// Count handles GET /api/trash/count
func (h *TrashHandler) Count(w http.ResponseWriter, r *http.Request) {
	count, err := h.trash.Count()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to count trash"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]int{"count": count})
}
