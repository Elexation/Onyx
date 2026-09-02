package handler

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/Elexation/onyx/internal/domain"
	"github.com/Elexation/onyx/internal/service"
)

type VersionHandler struct {
	versions *service.VersionService
}

func NewVersionHandler(versions *service.VersionService) *VersionHandler {
	return &VersionHandler{versions: versions}
}

// List handles GET /api/versions?path=/foo/bar.txt
func (h *VersionHandler) List(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "path is required"})
		return
	}

	items, err := h.versions.ListVersions(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list versions"})
		return
	}
	if items == nil {
		items = []domain.FileVersion{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// Restore handles POST /api/versions/{id}/restore
func (h *VersionHandler) Restore(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	if err := h.versions.RestoreVersion(id); err != nil {
		slog.Warn("version restore failed", "id", id, "error", err)
		if strings.Contains(err.Error(), "version not found") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "version not found"})
		} else {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to restore version"})
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "restored"})
}

// Count handles GET /api/versions/count
func (h *VersionHandler) Count(w http.ResponseWriter, r *http.Request) {
	count, err := h.versions.Count()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to count versions"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": count})
}

// Delete handles DELETE /api/versions/{id}
func (h *VersionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	if err := h.versions.DeleteVersion(id); err != nil {
		slog.Warn("version delete failed", "id", id, "error", err)
		if strings.Contains(err.Error(), "version not found") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "version not found"})
		} else {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete version"})
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
