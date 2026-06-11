package handler

import (
	"log/slog"
	"net/http"

	"github.com/Elexation/onyx/internal/service"
)

type StorageHandler struct {
	files *service.FileService
}

func NewStorageHandler(files *service.FileService) *StorageHandler {
	return &StorageHandler{files: files}
}

// GetUsage returns the data directory's host-filesystem usage.
func (h *StorageHandler) GetUsage(w http.ResponseWriter, r *http.Request) {
	used, total, err := h.files.DiskUsage()
	if err != nil {
		slog.Warn("disk usage lookup failed", "error", err)
		http.Error(w, `{"error":"failed to read disk usage"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]uint64{
		"used":  used,
		"total": total,
	})
}
