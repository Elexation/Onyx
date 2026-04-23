package handler

import (
	"net/http"

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

// Restore handles POST /api/trash/{id}/restore
func (h *TrashHandler) Restore(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	if err := h.trash.Restore(id); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "restored"})
}

// PermanentDelete handles DELETE /api/trash/{id}
func (h *TrashHandler) PermanentDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	if err := h.trash.PermanentDelete(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// EmptyTrash handles DELETE /api/trash
func (h *TrashHandler) EmptyTrash(w http.ResponseWriter, r *http.Request) {
	if err := h.trash.EmptyTrash(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
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
