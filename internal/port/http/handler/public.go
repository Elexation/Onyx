package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Elexation/onyx/internal/domain"
	"github.com/Elexation/onyx/internal/port/http/middleware"
	"github.com/Elexation/onyx/internal/service"
)

type shareSession struct {
	token     string
	expiresAt time.Time
}

type PublicHandler struct {
	shares       *service.ShareService
	files        *service.FileService
	rl           *middleware.RateLimiter
	trustedProxy bool
	requireHTTPS bool

	mu       sync.RWMutex
	sessions map[string]shareSession // cookie value → session
}

func NewPublicHandler(shares *service.ShareService, files *service.FileService, rl *middleware.RateLimiter, trustedProxy, requireHTTPS bool) *PublicHandler {
	h := &PublicHandler{
		shares:       shares,
		files:        files,
		rl:           rl,
		trustedProxy: trustedProxy,
		requireHTTPS: requireHTTPS,
		sessions:     make(map[string]shareSession),
	}
	go h.cleanSessions()
	return h
}

// Info handles GET /s/{token} — returns share metadata (or "password required").
func (h *PublicHandler) Info(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	link, _, err := h.shares.Validate(token)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if link == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "share not found or expired"})
		return
	}

	if link.HasPassword && !h.hasValidSession(r, token) {
		writeJSON(w, http.StatusOK, map[string]any{
			"passwordRequired": true,
			"isDir":            link.IsDir,
		})
		return
	}

	h.writeShareInfo(w, link)
}

// Verify handles POST /s/{token}/verify — checks password and sets session cookie.
func (h *PublicHandler) Verify(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	link, pwHash, err := h.shares.Validate(token)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	// Collapse non-existent token and wrong password into a single 403 to
	// avoid an existence oracle that lets brute force enumerate valid tokens.
	if link == nil || pwHash == nil || !h.shares.CheckPassword(*pwHash, req.Password) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "incorrect password"})
		return
	}

	h.rl.RecordSuccess(r)
	sessionID, err := h.createSession(token)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "share_session",
		Value:    sessionID,
		Path:     "/api/public/s/" + token,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   h.requireHTTPS || middleware.IsHTTPS(r, h.trustedProxy),
		MaxAge:   3600,
	})

	h.writeShareInfo(w, link)
}

// Download handles GET /s/{token}/dl or GET /s/{token}/dl/* — serves a file from the share.
func (h *PublicHandler) Download(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	link, _, err := h.shares.Validate(token)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if link == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "share not found or expired"})
		return
	}

	if link.HasPassword && !h.hasValidSession(r, token) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "password required"})
		return
	}

	filePath := link.FilePath
	if link.IsDir {
		// Extract sub-path: everything after /s/{token}/dl
		subPath := extractSubPath(r, token)
		if subPath == "" || subPath == "/" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "specify a file to download"})
			return
		}
		filePath = path.Join(link.FilePath, subPath)
		sharePrefix := strings.TrimSuffix(link.FilePath, "/") + "/"
		if !strings.HasPrefix(filePath, sharePrefix) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid path"})
			return
		}
	}

	file, modTime, _, err := h.files.OpenFile(filePath)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "file not found"})
		return
	}
	defer file.Close()

	h.shares.RecordAccess(link.ID)

	name := path.Base(filePath)
	w.Header().Set("Content-Disposition", contentDisposition("attachment", name))
	http.ServeContent(w, r, name, modTime, file)
}

// DownloadZip handles GET /s/{token}/zip — streams the entire shared directory as a zip archive.
func (h *PublicHandler) DownloadZip(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	link, _, err := h.shares.Validate(token)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if link == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "share not found or expired"})
		return
	}

	if !link.IsDir {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "not a directory share"})
		return
	}

	if link.HasPassword && !h.hasValidSession(r, token) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "password required"})
		return
	}

	h.shares.RecordAccess(link.ID)

	zipName := path.Base(link.FilePath) + ".zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", contentDisposition("attachment", zipName))

	if err := h.files.WriteZip(w, []string{link.FilePath}); err != nil {
		slog.Error("share zip stream error", "error", err)
	}
}

// Raw handles GET /s/{token}/raw or GET /s/{token}/raw/* — serves a file inline for preview.
func (h *PublicHandler) Raw(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	link, _, err := h.shares.Validate(token)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if link == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "share not found or expired"})
		return
	}

	if link.HasPassword && !h.hasValidSession(r, token) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "password required"})
		return
	}

	filePath := link.FilePath
	if link.IsDir {
		subPath := extractSubPath2(r, token, "raw")
		if subPath == "" || subPath == "/" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "specify a file to preview"})
			return
		}
		filePath = path.Join(link.FilePath, subPath)
		sharePrefix := strings.TrimSuffix(link.FilePath, "/") + "/"
		if !strings.HasPrefix(filePath, sharePrefix) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid path"})
			return
		}
	}

	file, modTime, _, err := h.files.OpenFile(filePath)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "file not found"})
		return
	}
	defer file.Close()

	name := path.Base(filePath)

	if needsSandbox(name) {
		w.Header().Set("Content-Security-Policy", "sandbox")
	}

	w.Header().Set("Content-Disposition", contentDisposition("inline", name))
	http.ServeContent(w, r, name, modTime, file)
}

func (h *PublicHandler) writeShareInfo(w http.ResponseWriter, link *domain.ShareLink) {
	resp := map[string]any{
		"isDir":    link.IsDir,
		"fileName": path.Base(link.FilePath),
	}

	if link.IsDir {
		items, err := h.files.ListDirectory(link.FilePath, false)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list directory"})
			return
		}
		shareBase := strings.TrimSuffix(link.FilePath, "/") + "/"
		for i := range items {
			items[i].Path = strings.TrimPrefix(items[i].Path, shareBase)
		}
		resp["items"] = items
	} else {
		info, err := h.files.GetFileInfo(link.FilePath)
		if err == nil {
			resp["mimeType"] = info.MIMEType
			resp["size"] = info.Size
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

const maxShareSessions = 10000

func (h *PublicHandler) createSession(token string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	id := hex.EncodeToString(b)

	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.sessions) >= maxShareSessions {
		h.evictOldestLocked()
	}
	h.sessions[id] = shareSession{token: token, expiresAt: time.Now().Add(1 * time.Hour)}
	return id, nil
}

// evictOldestLocked removes the session with the earliest expiry. Caller must
// hold h.mu.
func (h *PublicHandler) evictOldestLocked() {
	var oldestID string
	var oldestExp time.Time
	first := true
	for id, sess := range h.sessions {
		if first || sess.expiresAt.Before(oldestExp) {
			oldestID = id
			oldestExp = sess.expiresAt
			first = false
		}
	}
	if oldestID != "" {
		delete(h.sessions, oldestID)
	}
}

func (h *PublicHandler) hasValidSession(r *http.Request, token string) bool {
	cookie, err := r.Cookie("share_session")
	if err != nil {
		return false
	}

	h.mu.RLock()
	sess, ok := h.sessions[cookie.Value]
	h.mu.RUnlock()

	return ok && sess.token == token && time.Now().Before(sess.expiresAt)
}

func (h *PublicHandler) cleanSessions() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		h.mu.Lock()
		for id, sess := range h.sessions {
			if now.After(sess.expiresAt) {
				delete(h.sessions, id)
			}
		}
		h.mu.Unlock()
	}
}

func extractSubPath(r *http.Request, token string) string {
	prefix := "/api/public/s/" + token + "/dl"
	p := strings.TrimPrefix(r.URL.Path, prefix)
	if p == "" {
		return "/"
	}
	return p
}

func extractSubPath2(r *http.Request, token string, segment string) string {
	prefix := "/api/public/s/" + token + "/" + segment
	p := strings.TrimPrefix(r.URL.Path, prefix)
	if p == "" {
		return "/"
	}
	return p
}
