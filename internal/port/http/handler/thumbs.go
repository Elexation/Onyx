package handler

import (
	"net/http"

	"github.com/Elexation/onyx/internal/service"
)

type ThumbsHandler struct {
	thumbs *service.ThumbnailService
}

func NewThumbsHandler(thumbs *service.ThumbnailService) *ThumbsHandler {
	return &ThumbsHandler{thumbs: thumbs}
}

// Get handles GET /api/thumbs/*?size=small|medium|large
func (h *ThumbsHandler) Get(w http.ResponseWriter, r *http.Request) {
	filePath := extractWildcard(r, "/api/thumbs")

	sizeStr := r.URL.Query().Get("size")
	if sizeStr == "" {
		sizeStr = "medium"
	}
	size, ok := service.ParseThumbSize(sizeStr)
	if !ok {
		http.Error(w, `{"error":"invalid size"}`, http.StatusBadRequest)
		return
	}

	result, err := h.thumbs.Lookup(filePath, size)
	if err != nil {
		writeFileError(w, err)
		return
	}

	switch result.Status {
	case service.StatusReady:
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "private, max-age=86400, immutable")
		http.ServeFile(w, r, result.FilePath)
	case service.StatusQueued:
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(http.StatusAccepted)
	case service.StatusUnsupported, service.StatusFailed:
		w.WriteHeader(http.StatusUnsupportedMediaType)
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}
}
