package handler

import (
	"net/http"

	"github.com/Elexation/onyx/internal/service"
)

type UploadHandler struct {
	files *service.FileService
}

func NewUploadHandler(files *service.FileService) *UploadHandler {
	return &UploadHandler{files: files}
}

// CheckConflicts handles POST /api/files/check-conflicts
func (h *UploadHandler) CheckConflicts(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TargetDir string   `json:"targetDir"`
		Paths     []string `json:"paths"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if len(req.Paths) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "paths is required"})
		return
	}
	if req.TargetDir == "" {
		req.TargetDir = "/"
	}

	conflicts, err := h.files.CheckConflicts(req.TargetDir, req.Paths)
	if err != nil {
		writeFileError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"conflicts": conflicts})
}
