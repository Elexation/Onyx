package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Elexation/onyx/internal/domain"
	"github.com/Elexation/onyx/internal/service"
)

type ShareHandler struct {
	shares *service.ShareService
}

func NewShareHandler(shares *service.ShareService) *ShareHandler {
	return &ShareHandler{shares: shares}
}

// Create handles POST /api/shares
func (h *ShareHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path      string `json:"path"`
		IsDir     bool   `json:"isDir"`
		ExpiresIn string `json:"expiresIn,omitempty"`
		Password  string `json:"password,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if req.Path == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "path is required"})
		return
	}

	var expiresIn *time.Duration
	if req.ExpiresIn != "" {
		d, err := time.ParseDuration(req.ExpiresIn)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid expiration duration"})
			return
		}
		expiresIn = &d
	}

	link, fullToken, err := h.shares.Create(req.Path, req.IsDir, expiresIn, req.Password)
	if err != nil {
		if err.Error() == "a share link already exists for this path" {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	slog.Info("security_event", "event", "share_create", "path", req.Path)
	link.Token = fullToken
	writeJSON(w, http.StatusCreated, link)
}

// GetByPath handles GET /api/shares/by-path?path=...
func (h *ShareHandler) GetByPath(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "path is required"})
		return
	}

	link, err := h.shares.GetByPath(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to look up share"})
		return
	}
	if link == nil {
		writeJSON(w, http.StatusOK, map[string]any{"share": nil})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"share": link})
}

// List handles GET /api/shares
func (h *ShareHandler) List(w http.ResponseWriter, r *http.Request) {
	links, err := h.shares.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list shares"})
		return
	}
	if links == nil {
		links = []domain.ShareLink{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"shares": links})
}

// Count handles GET /api/shares/count
func (h *ShareHandler) Count(w http.ResponseWriter, r *http.Request) {
	count, err := h.shares.Count()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to count shares"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": count})
}

// Delete handles DELETE /api/shares/{id}
func (h *ShareHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	if err := h.shares.Delete(id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	slog.Info("security_event", "event", "share_delete", "id", id)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
