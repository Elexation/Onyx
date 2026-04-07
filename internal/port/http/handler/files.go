package handler

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/Elexation/onyx/internal/service"
)

type FileHandler struct {
	files *service.FileService
}

func NewFileHandler(files *service.FileService) *FileHandler {
	return &FileHandler{files: files}
}

// List handles GET /api/files/* — returns a directory listing or file metadata.
func (h *FileHandler) List(w http.ResponseWriter, r *http.Request) {
	filePath := extractWildcard(r, "/api/files")
	showHidden := r.URL.Query().Get("showHidden") == "true"

	info, err := h.files.GetFileInfo(filePath)
	if err != nil {
		writeFileError(w, err)
		return
	}

	if info.IsDir {
		items, err := h.files.ListDirectory(filePath, showHidden)
		if err != nil {
			writeFileError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"path":  info.Path,
			"items": items,
		})
		return
	}

	writeJSON(w, http.StatusOK, info)
}

// Download handles GET /api/download/* — serves a file with Content-Disposition: attachment.
func (h *FileHandler) Download(w http.ResponseWriter, r *http.Request) {
	filePath := extractWildcard(r, "/api/download")

	file, modTime, _, err := h.files.OpenFile(filePath)
	if err != nil {
		writeFileError(w, err)
		return
	}
	defer file.Close()

	// Extract just the filename for the Content-Disposition header
	name := filePath
	if idx := strings.LastIndex(filePath, "/"); idx >= 0 {
		name = filePath[idx+1:]
	}

	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	http.ServeContent(w, r, name, modTime, file)
}

// extractWildcard pulls the path after the prefix from the URL.
func extractWildcard(r *http.Request, prefix string) string {
	p := strings.TrimPrefix(r.URL.Path, prefix)
	if p == "" || p == "/" {
		return "/"
	}
	return p
}

// writeFileError maps filesystem errors to HTTP status codes.
func writeFileError(w http.ResponseWriter, err error) {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		if os.IsNotExist(err) {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		if os.IsPermission(err) {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		// Path traversal attempts from os.Root
		http.Error(w, `{"error":"invalid path"}`, http.StatusBadRequest)
		return
	}

	if os.IsNotExist(err) {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
}
