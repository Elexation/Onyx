package handler

import (
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Elexation/onyx/internal/domain"
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
		dirsOnly := r.URL.Query().Get("dirsOnly") == "true"
		var items []domain.FileInfo
		var err error
		if dirsOnly {
			items, err = h.files.ListDirectoriesOnly(filePath, showHidden)
		} else {
			items, err = h.files.ListDirectory(filePath, showHidden)
		}
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

	w.Header().Set("Content-Disposition", contentDisposition("attachment", name))
	http.ServeContent(w, r, name, modTime, file)
}

// Preview handles GET /api/preview/* — serves a file inline for browser preview.
func (h *FileHandler) Preview(w http.ResponseWriter, r *http.Request) {
	filePath := extractWildcard(r, "/api/preview")

	file, modTime, _, err := h.files.OpenFile(filePath)
	if err != nil {
		writeFileError(w, err)
		return
	}
	defer file.Close()

	name := filePath
	if idx := strings.LastIndex(filePath, "/"); idx >= 0 {
		name = filePath[idx+1:]
	}

	ctype := resolvePreviewContentType(file, name)
	if !isSafeInline(ctype) {
		w.Header().Set("Content-Security-Policy", "sandbox")
	}

	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Disposition", contentDisposition("inline", name))
	http.ServeContent(w, r, name, modTime, file)
}

// resolvePreviewContentType returns the Content-Type the browser will see for
// an inline preview, mirroring http.ServeContent's resolution: extension first,
// 512-byte sniff fallback. The resolved type drives the sandbox decision —
// extension-only matching is bypassable via files with no extension (sniffed
// to text/html), .xht (resolves to application/xhtml+xml), Windows-registry
// MIME entries, or future MIME-DB additions.
func resolvePreviewContentType(file io.ReadSeeker, name string) string {
	if ct := mime.TypeByExtension(path.Ext(name)); ct != "" {
		return ct
	}
	var buf [512]byte
	n, _ := file.Read(buf[:])
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "application/octet-stream"
	}
	if n == 0 {
		return "application/octet-stream"
	}
	return http.DetectContentType(buf[:n])
}

// isSafeInline reports whether ct is a Content-Type the browser cannot execute
// scripts under when served inline. Anything not in this allow-list — HTML,
// XHTML, XML, MHTML, SVG, application/octet-stream, unknown types — must be
// sandboxed at the CSP layer. Allow-list is strictly safer than enumerating
// scriptable extensions: the web evolves, mime DBs differ, attacker uploads
// can be extension-less.
func isSafeInline(ct string) bool {
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	ct = strings.ToLower(strings.TrimSpace(ct))
	if ct == "image/svg+xml" {
		return false
	}
	if strings.HasPrefix(ct, "image/") ||
		strings.HasPrefix(ct, "video/") ||
		strings.HasPrefix(ct, "audio/") {
		return true
	}
	switch ct {
	case "application/pdf", "text/plain":
		return true
	}
	return false
}

// DownloadZip handles GET /api/download/zip?path=...&path=...
// Streams a zip archive containing all requested files and directories.
func (h *FileHandler) DownloadZip(w http.ResponseWriter, r *http.Request) {
	paths := r.URL.Query()["path"]
	if len(paths) == 0 {
		http.Error(w, `{"error":"no paths specified"}`, http.StatusBadRequest)
		return
	}
	if len(paths) > 1000 {
		http.Error(w, `{"error":"too many paths"}`, http.StatusBadRequest)
		return
	}

	// filepath-based cleaning so `..\..\x` is normalized on Windows too;
	// defense-in-depth (os.Root is authoritative), mirrors resolveSafePath.
	sep := string(filepath.Separator)
	for i, p := range paths {
		fpClean := filepath.Clean(filepath.FromSlash(strings.TrimLeft(p, "/")))
		if fpClean == "" || fpClean == "." || fpClean == ".." ||
			strings.HasPrefix(fpClean, ".."+sep) || strings.HasPrefix(fpClean, sep+"..") {
			http.Error(w, `{"error":"invalid path"}`, http.StatusBadRequest)
			return
		}
		cleaned := "/" + filepath.ToSlash(fpClean)
		paths[i] = cleaned
		if _, err := h.files.GetFileInfo(cleaned); err != nil {
			writeFileError(w, err)
			return
		}
	}

	zipName := "download.zip"
	if len(paths) == 1 {
		name := paths[0]
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			name = name[idx+1:]
		}
		zipName = name + ".zip"
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", contentDisposition("attachment", zipName))

	if err := h.files.WriteZip(w, paths); err != nil {
		if !errors.Is(err, service.ErrZipSizeExceeded) {
			slog.Error("zip stream error", "error", err)
		}
	}
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
		if os.IsExist(err) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "a folder with this name already exists"})
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

	slog.Warn("file handler error", "err", err)
	http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
}

func contentDisposition(disposition, filename string) string {
	cd := mime.FormatMediaType(disposition, map[string]string{"filename": filename})
	if cd == "" {
		return disposition
	}
	return cd
}
