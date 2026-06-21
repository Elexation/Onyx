package handler

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Elexation/onyx/internal/adapter/media"
	"github.com/Elexation/onyx/internal/service"
)

type StreamHandler struct {
	probe     *service.ProbeService
	transcode *service.TranscodeService
}

func NewStreamHandler(probe *service.ProbeService, transcode *service.TranscodeService) *StreamHandler {
	return &StreamHandler{probe: probe, transcode: transcode}
}

// infoResponse is the JSON returned by GET /api/stream/info/*.
type infoResponse struct {
	Codec          string  `json:"codec"`
	Width          int     `json:"width"`
	Height         int     `json:"height"`
	Duration       float64 `json:"duration"`
	Bitrate        int64   `json:"bitrate"`
	Framerate      float64 `json:"framerate"`
	NeedsTranscode bool    `json:"needsTranscode"`
}

// Info handles GET /api/stream/info/* — returns codec metadata for the file
// and an advisory hint on whether it needs transcoding.
func (h *StreamHandler) Info(w http.ResponseWriter, r *http.Request) {
	filePath := extractWildcard(r, "/api/stream/info")
	if h.redirectDirectNavigate(w, r, filePath) {
		return
	}
	if !h.probe.HasFFprobe() {
		http.Error(w, `{"error":"ffprobe not available"}`, http.StatusNotImplemented)
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

// needsTranscode is an advisory hint. The frontend makes the final decision
// via MediaCapabilities since codec support is browser-specific.
func needsTranscode(codec string) bool {
	switch codec {
	case "h264", "vp8", "vp9":
		return false
	}
	return true
}

// Master handles GET /api/stream/master/* — triggers transcode session
// creation if this file has never been transcoded, then returns the
// master playlist. Subsequent calls are map-lookup cheap.
func (h *StreamHandler) Master(w http.ResponseWriter, r *http.Request) {
	filePath := extractWildcard(r, "/api/stream/master")
	if h.redirectDirectNavigate(w, r, filePath) {
		return
	}
	if h.transcode == nil || !h.transcode.HasFFmpeg() {
		http.Error(w, `{"error":"transcoding not available"}`, http.StatusNotImplemented)
		return
	}
	session, err := h.transcode.Ensure(r.Context(), filePath)
	if err != nil {
		writeFileError(w, err)
		return
	}
	h.serveCachedFile(w, r, filepath.Join(session.Dir(), "master.m3u8"), "application/vnd.apple.mpegurl")
}

// Playlist handles GET /api/stream/playlist/{v}/* — returns the full
// VOD playlist for variant v (all segments listed, ENDLIST present).
// The playlist is written at session start from the probed duration.
func (h *StreamHandler) Playlist(w http.ResponseWriter, r *http.Request) {
	filePath := extractWildcard(r, "/api/stream/playlist/"+chi.URLParam(r, "v"))
	if h.redirectDirectNavigate(w, r, filePath) {
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
	session, err := h.transcode.Ensure(r.Context(), filePath)
	if err != nil {
		writeFileError(w, err)
		return
	}
	if variant >= session.VariantCount() {
		http.Error(w, `{"error":"variant out of range"}`, http.StatusBadRequest)
		return
	}
	h.serveCachedFile(w, r, filepath.Join(session.Dir(), media.VariantDir(variant), "playlist.m3u8"), "application/vnd.apple.mpegurl")
}

// Init handles GET /api/stream/init/{v}/* — serves variant v's fMP4
// init segment, blocking briefly if ffmpeg has not yet produced it.
func (h *StreamHandler) Init(w http.ResponseWriter, r *http.Request) {
	filePath := extractWildcard(r, "/api/stream/init/"+chi.URLParam(r, "v"))
	if h.redirectDirectNavigate(w, r, filePath) {
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
	h.serveCachedFile(w, r, initPath, "video/mp4")
}

// Segment handles GET /api/stream/segment/{v}/{n}/* — returns variant
// v's data{NNNNNN}.m4s, blocking up to 30s while ffmpeg produces the
// segment and triggering a seek restart if the requested segment is
// outside the active window.
func (h *StreamHandler) Segment(w http.ResponseWriter, r *http.Request) {
	filePath := extractWildcard(r, "/api/stream/segment/"+chi.URLParam(r, "v")+"/"+chi.URLParam(r, "n"))
	if h.redirectDirectNavigate(w, r, filePath) {
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

// parseVariant extracts the {v} URL param and validates it as a
// non-negative integer. Writes a 400 and returns ok=false on failure.
func parseVariant(w http.ResponseWriter, r *http.Request) (int, bool) {
	v, err := strconv.Atoi(chi.URLParam(r, "v"))
	if err != nil || v < 0 {
		http.Error(w, `{"error":"invalid variant"}`, http.StatusBadRequest)
		return 0, false
	}
	return v, true
}

// redirectDirectNavigate bounces a direct browser navigation (user pasted
// a stream URL into the address bar) to the file browser's parent folder
// view. Detects two signals: Sec-Fetch-Mode=navigate (the primary modern
// signal) and an Accept header asking for text/html (the fallback when
// privacy extensions strip Sec-Fetch-*). hls.js XHR and <video> element
// fetches send Accept: */* and Sec-Fetch-Mode=cors|no-cors, so MSE
// playback is unaffected.
func (h *StreamHandler) redirectDirectNavigate(w http.ResponseWriter, r *http.Request, filePath string) bool {
	mode := r.Header.Get("Sec-Fetch-Mode")
	accept := r.Header.Get("Accept")
	if mode != "navigate" && !strings.Contains(accept, "text/html") {
		return false
	}
	dir := path.Dir(filePath)
	if dir == "" || dir == "." {
		dir = "/"
	}
	u := &url.URL{Path: "/files" + dir}
	if !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
	}
	http.Redirect(w, r, u.String(), http.StatusFound)
	return true
}

// serveCachedFile reads a file produced by ffmpeg / the service and
// sends it with the given content type. Small files only — master and
// media playlists, init.mp4.
func (h *StreamHandler) serveCachedFile(w http.ResponseWriter, r *http.Request, path, contentType string) {
	data, err := os.ReadFile(path)
	if err != nil {
		writeFileError(w, err)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=60")
	w.Write(data)
}

// waitForStableFile polls up to 30 seconds for path to exist with a
// non-zero size that stays constant across two consecutive polls. This
// avoids races where os.Stat reports the file exists but ffmpeg has not
// yet written the bytes, resulting in an empty read.
func waitForStableFile(ctx context.Context, path string) error {
	deadline := time.Now().Add(30 * time.Second)
	var lastSize int64 = -1
	for {
		if info, err := os.Stat(path); err == nil && info.Size() > 0 {
			if info.Size() == lastSize {
				return nil
			}
			lastSize = info.Size()
		}
		if time.Now().After(deadline) {
			return errors.New("timeout")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}
