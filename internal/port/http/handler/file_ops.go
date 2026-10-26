package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"runtime"
	"strings"

	"github.com/Elexation/onyx/internal/port/http/middleware"
	"github.com/Elexation/onyx/internal/service"
)

type FileOpsHandler struct {
	files *service.FileService
}

func NewFileOpsHandler(files *service.FileService) *FileOpsHandler {
	return &FileOpsHandler{files: files}
}

// MakeDir handles POST /api/files/mkdir
func (h *FileOpsHandler) MakeDir(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Path == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "path is required"})
		return
	}

	if err := h.files.MakeDir(req.Path); err != nil {
		if errors.Is(err, service.ErrDirBlockedByFile) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "a file with this name already exists"})
			return
		}
		if errors.Is(err, service.ErrInvalidDirPath) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid folder name"})
			return
		}
		writeFileError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"path": req.Path})
}

// Rename handles POST /api/files/rename
func (h *FileOpsHandler) Rename(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path    string `json:"path"`
		NewName string `json:"newName"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Path == "" || req.NewName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "path and newName are required"})
		return
	}
	if rejectSepAlias(w, req.Path) {
		return
	}

	err := h.files.Rename(req.Path, req.NewName)
	if err != nil {
		var conflict *service.ConflictError
		if errors.As(err, &conflict) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": conflict.Error()})
			return
		}
		var invalid *service.InvalidRenameError
		if errors.As(err, &invalid) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": invalid.Error()})
			return
		}
		writeFileError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Move handles POST /api/files/move
func (h *FileOpsHandler) Move(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Paths       []string `json:"paths"`
		Destination string   `json:"destination"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if len(req.Paths) == 0 || req.Destination == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "paths and destination are required"})
		return
	}
	if len(req.Paths) > 500 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "too many paths (max 500)"})
		return
	}
	if rejectSepAlias(w, append(req.Paths, req.Destination)...) {
		return
	}

	results, err := h.files.Move(req.Paths, req.Destination)
	if err != nil {
		writeFileError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

// Copy handles POST /api/files/copy
func (h *FileOpsHandler) Copy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Paths       []string `json:"paths"`
		Destination string   `json:"destination"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if len(req.Paths) == 0 || req.Destination == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "paths and destination are required"})
		return
	}
	if len(req.Paths) > 500 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "too many paths (max 500)"})
		return
	}
	if rejectSepAlias(w, append(req.Paths, req.Destination)...) {
		return
	}

	results, err := h.files.Copy(req.Paths, req.Destination)
	if err != nil {
		writeFileError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

// Delete handles DELETE /api/files
func (h *FileOpsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Paths     []string `json:"paths"`
		Permanent bool     `json:"permanent"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if len(req.Paths) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "paths is required"})
		return
	}
	if len(req.Paths) > 500 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "too many paths (max 500)"})
		return
	}
	if rejectSepAlias(w, req.Paths...) {
		return
	}

	// Cookie sessions get a 409 so the SPA re-asks; token callers keep the permanent fallback
	if !req.Permanent && !middleware.IsBearerAuth(r.Context()) && !h.files.TrashEnabled() {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "trash is disabled", "code": "trash_disabled"})
		return
	}

	results := h.files.Delete(req.Paths, req.Permanent)
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

// rejectSepAlias 400s a Windows path containing \: storage splits on it, share/trash/event bookkeeping doesn't
func rejectSepAlias(w http.ResponseWriter, paths ...string) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	for _, p := range paths {
		if strings.Contains(p, `\`) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid path"})
			return true
		}
	}
	return false
}

// decodeBody parses the JSON request body into dst.
// Returns false and writes an error response if decoding fails.
func decodeBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return false
	}
	return true
}
