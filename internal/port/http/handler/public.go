package handler

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Elexation/onyx/internal/adapter/media"
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
	probe        *service.ProbeService
	transcode    *service.TranscodeService
	rl           *middleware.RateLimiter
	trustedProxy bool
	requireHTTPS bool

	mu       sync.RWMutex
	sessions map[string]shareSession // cookie value → session
}

func NewPublicHandler(shares *service.ShareService, files *service.FileService, probe *service.ProbeService, transcode *service.TranscodeService, rl *middleware.RateLimiter, trustedProxy, requireHTTPS bool) *PublicHandler {
	h := &PublicHandler{
		shares:       shares,
		files:        files,
		probe:        probe,
		transcode:    transcode,
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
		resp := map[string]any{
			"passwordRequired": true,
			"isDir":            link.IsDir,
		}
		writeJSON(w, http.StatusOK, resp)
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

// validateShareAccess handles token validate + password session check for
// share-scoped stream endpoints. Returns the link on success, or writes an
// error response and returns false.
func (h *PublicHandler) validateShareAccess(w http.ResponseWriter, r *http.Request) (*domain.ShareLink, bool) {
	token := chi.URLParam(r, "token")
	link, _, err := h.shares.Validate(token)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return nil, false
	}
	// Collapse not-found + password-required into a uniform 403 so stream
	// endpoints do not become a token-existence oracle. Matches Verify.
	if link == nil || (link.HasPassword && !h.hasValidSession(r, token)) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "password required"})
		return nil, false
	}
	return link, true
}

// redirectDirectNavigate bounces a direct browser navigation (user pasted
// a stream URL into the address bar) to the share landing page. Two signals
// identify a top-level document navigation: Sec-Fetch-Mode=navigate (the
// primary modern signal) and an Accept header asking for text/html (the
// fallback when privacy extensions strip Sec-Fetch-*). hls.js XHR and
// <video> element fetches send Accept: */* and Sec-Fetch-Mode=cors|no-cors,
// so MSE playback is unaffected.
func (h *PublicHandler) redirectDirectNavigate(w http.ResponseWriter, r *http.Request) bool {
	mode := r.Header.Get("Sec-Fetch-Mode")
	accept := r.Header.Get("Accept")
	if mode != "navigate" && !strings.Contains(accept, "text/html") {
		return false
	}
	token := chi.URLParam(r, "token")
	http.Redirect(w, r, "/s/"+token, http.StatusFound)
	return true
}

// extractShareStreamPath pulls the file sub-path out of a stream URL and
// validates it is within the share scope. urlSegment is the part of the
// URL between /s/{token}/ and the file wildcard (e.g. "stream/info",
// "stream/playlist/0").
func (h *PublicHandler) extractShareStreamPath(w http.ResponseWriter, r *http.Request, link *domain.ShareLink, urlSegment string) (string, bool) {
	token := chi.URLParam(r, "token")
	subPath := extractSubPath2(r, token, urlSegment)
	filePath, ok := resolveSharePath(link, subPath)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid path"})
		return "", false
	}
	return filePath, true
}

// servePublicCachedFile sends a small cached artifact (playlist, init
// segment) with HLS-appropriate headers. Duplicated from stream.go's
// private helper to keep share-handler changes self-contained.
func servePublicCachedFile(w http.ResponseWriter, path, contentType string) {
	data, err := os.ReadFile(path)
	if err != nil {
		writeFileError(w, err)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=60")
	w.Write(data)
}

// StreamInfo handles GET /s/{token}/stream/info/* — probes video metadata.
func (h *PublicHandler) StreamInfo(w http.ResponseWriter, r *http.Request) {
	if h.redirectDirectNavigate(w, r) {
		return
	}
	link, ok := h.validateShareAccess(w, r)
	if !ok {
		return
	}
	if h.probe == nil || !h.probe.HasFFprobe() {
		http.Error(w, `{"error":"ffprobe not available"}`, http.StatusNotImplemented)
		return
	}
	filePath, ok := h.extractShareStreamPath(w, r, link, "stream/info")
	if !ok {
		return
	}
	info, err := h.probe.Probe(r.Context(), filePath)
	if err != nil {
		if errors.Is(err, service.ErrNoVideoStream) {
			http.Error(w, `{"error":"no video stream"}`, http.StatusUnsupportedMediaType)
			return
		}
		writeFileError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, infoResponse{
		Codec:          info.Codec,
		Width:          info.Width,
		Height:         info.Height,
		Duration:       info.Duration,
		Bitrate:        info.Bitrate,
		Framerate:      info.Framerate,
		NeedsTranscode: needsTranscode(info.Codec),
	})
}

// StreamMaster handles GET /s/{token}/stream/master/* — master HLS playlist.
func (h *PublicHandler) StreamMaster(w http.ResponseWriter, r *http.Request) {
	if h.redirectDirectNavigate(w, r) {
		return
	}
	link, ok := h.validateShareAccess(w, r)
	if !ok {
		return
	}
	if h.transcode == nil || !h.transcode.HasFFmpeg() {
		http.Error(w, `{"error":"transcoding not available"}`, http.StatusNotImplemented)
		return
	}
	filePath, ok := h.extractShareStreamPath(w, r, link, "stream/master")
	if !ok {
		return
	}
	session, err := h.transcode.Ensure(r.Context(), filePath)
	if err != nil {
		writeFileError(w, err)
		return
	}
	servePublicHLSPlaylist(w, filepath.Join(session.Dir(), "master.m3u8"), "application/vnd.apple.mpegurl", chi.URLParam(r, "token"))
}

// StreamPlaylist handles GET /s/{token}/stream/playlist/{v}/* — variant playlist.
func (h *PublicHandler) StreamPlaylist(w http.ResponseWriter, r *http.Request) {
	if h.redirectDirectNavigate(w, r) {
		return
	}
	link, ok := h.validateShareAccess(w, r)
	if !ok {
		return
	}
	if h.transcode == nil || !h.transcode.HasFFmpeg() {
		http.Error(w, `{"error":"transcoding not available"}`, http.StatusNotImplemented)
		return
	}
	variant, ok := parseVariant(w, r)
	if !ok {
		return
	}
	filePath, ok := h.extractShareStreamPath(w, r, link, "stream/playlist/"+chi.URLParam(r, "v"))
	if !ok {
		return
	}
	session, err := h.transcode.Ensure(r.Context(), filePath)
	if err != nil {
		writeFileError(w, err)
		return
	}
	if variant >= session.VariantCount() {
		http.Error(w, `{"error":"variant out of range"}`, http.StatusBadRequest)
		return
	}
	servePublicHLSPlaylist(w, filepath.Join(session.Dir(), media.VariantDir(variant), "playlist.m3u8"), "application/vnd.apple.mpegurl", chi.URLParam(r, "token"))
}

// StreamInit handles GET /s/{token}/stream/init/{v}/* — fMP4 init segment.
func (h *PublicHandler) StreamInit(w http.ResponseWriter, r *http.Request) {
	if h.redirectDirectNavigate(w, r) {
		return
	}
	link, ok := h.validateShareAccess(w, r)
	if !ok {
		return
	}
	if h.transcode == nil || !h.transcode.HasFFmpeg() {
		http.Error(w, `{"error":"transcoding not available"}`, http.StatusNotImplemented)
		return
	}
	variant, ok := parseVariant(w, r)
	if !ok {
		return
	}
	filePath, ok := h.extractShareStreamPath(w, r, link, "stream/init/"+chi.URLParam(r, "v"))
	if !ok {
		return
	}
	session, err := h.transcode.Ensure(r.Context(), filePath)
	if err != nil {
		writeFileError(w, err)
		return
	}
	if variant >= session.VariantCount() {
		http.Error(w, `{"error":"variant out of range"}`, http.StatusBadRequest)
		return
	}
	initPath := filepath.Join(session.Dir(), media.HLSInitName(variant))
	if err := waitForStableFile(r.Context(), initPath); err != nil {
		http.Error(w, `{"error":"init segment not ready"}`, http.StatusGatewayTimeout)
		return
	}
	servePublicCachedFile(w, initPath, "video/mp4")
}

// StreamSegment handles GET /s/{token}/stream/segment/{v}/{n}/* — media segment.
func (h *PublicHandler) StreamSegment(w http.ResponseWriter, r *http.Request) {
	if h.redirectDirectNavigate(w, r) {
		return
	}
	link, ok := h.validateShareAccess(w, r)
	if !ok {
		return
	}
	if h.transcode == nil || !h.transcode.HasFFmpeg() {
		http.Error(w, `{"error":"transcoding not available"}`, http.StatusNotImplemented)
		return
	}
	variant, ok := parseVariant(w, r)
	if !ok {
		return
	}
	segNum, err := strconv.Atoi(chi.URLParam(r, "n"))
	if err != nil || segNum < 0 {
		http.Error(w, `{"error":"invalid segment number"}`, http.StatusBadRequest)
		return
	}
	filePath, ok := h.extractShareStreamPath(w, r, link, "stream/segment/"+chi.URLParam(r, "v")+"/"+chi.URLParam(r, "n"))
	if !ok {
		return
	}
	session, err := h.transcode.Ensure(r.Context(), filePath)
	if err != nil {
		writeFileError(w, err)
		return
	}
	data, err := h.transcode.GetSegment(r.Context(), session.Hash(), variant, segNum)
	if err != nil {
		if errors.Is(err, service.ErrSegmentTimeout) {
			http.Error(w, `{"error":"segment not ready"}`, http.StatusGatewayTimeout)
			return
		}
		if errors.Is(err, service.ErrSessionNotFound) {
			http.Error(w, `{"error":"session not found"}`, http.StatusNotFound)
			return
		}
		if errors.Is(err, service.ErrSegmentOutOfRange) {
			http.Error(w, `{"error":"segment out of range"}`, http.StatusBadRequest)
			return
		}
		writeFileError(w, err)
		return
	}
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Write(data)
}

func (h *PublicHandler) writeShareInfo(w http.ResponseWriter, link *domain.ShareLink) {
	resp := map[string]any{
		"isDir":    link.IsDir,
		"fileName": path.Base(link.FilePath),
	}
	if link.ExpiresAt > 0 {
		resp["expiresAt"] = link.ExpiresAt
	}

	if link.IsDir {
		items, err := h.files.ListDirectory(link.FilePath, false)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list directory"})
			return
		}
		shareBase := strings.TrimSuffix(link.FilePath, "/") + "/"
		for i := range items {
			items[i].Path = "/" + strings.TrimPrefix(items[i].Path, shareBase)
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

// resolveSharePath accepts a sub-path and returns the absolute file path it
// refers to within the share, or false if it escapes scope. For single-file
// shares, subPath is ignored. For directory shares, subPath may be either
// share-relative (e.g. "/movie.mkv") or already fully-qualified under the
// share base (e.g. "/shared/movie.mkv" — this happens when HLS playlist
// content, rewritten by rewriteShareHLS, flows back through the subsequent
// playlist/segment requests).
func resolveSharePath(link *domain.ShareLink, subPath string) (string, bool) {
	if !link.IsDir {
		return link.FilePath, true
	}
	if subPath == "" || subPath == "/" {
		return "", false
	}
	// path.Clean does not treat \ as a separator on any OS, so `..\..\x`
	// survives normalization. Reject it at the app layer — os.Root is the
	// storage-layer bedrock, this is defense-in-depth.
	if strings.Contains(subPath, "\\") {
		return "", false
	}
	subPath = path.Clean(subPath)
	sharePrefix := strings.TrimSuffix(link.FilePath, "/") + "/"
	if strings.HasPrefix(subPath, sharePrefix) {
		return subPath, true
	}
	filePath := path.Join(link.FilePath, subPath)
	if !strings.HasPrefix(filePath, sharePrefix) {
		return "", false
	}
	return filePath, true
}

// rewriteShareHLS rewrites absolute `/api/stream/…` URLs inside an m3u8
// body to their public-share equivalent so share users can fetch the
// downstream playlist/init/segment without admin auth. The file path
// portion stays intact; resolveSharePath accepts it in full-path form.
func rewriteShareHLS(content []byte, token string) []byte {
	return bytes.ReplaceAll(content, []byte("/api/stream/"), []byte("/api/public/s/"+token+"/stream/"))
}

// servePublicHLSPlaylist reads an m3u8 playlist file, rewrites its internal
// URLs for share-scoped access, and writes it with HLS headers.
func servePublicHLSPlaylist(w http.ResponseWriter, path, contentType, token string) {
	data, err := os.ReadFile(path)
	if err != nil {
		writeFileError(w, err)
		return
	}
	data = rewriteShareHLS(data, token)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=60")
	w.Write(data)
}
