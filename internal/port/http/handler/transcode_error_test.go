package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Elexation/onyx/internal/service"
)

// hls.js gives up on any 4xx, so a busy refusal must be a retryable 503
func TestWriteTranscodeErrorBusyIs503(t *testing.T) {
	rec := httptest.NewRecorder()
	writeTranscodeError(rec, fmt.Errorf("seek restart: %w", service.ErrTranscodeBusy))
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("got %d with Retry-After %q, want 503 with a Retry-After", rec.Code, rec.Header().Get("Retry-After"))
	}
}
