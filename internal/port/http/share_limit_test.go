package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/Elexation/onyx/internal/port/http/middleware"
)

// The per-share download cap sees the token inside a route group, and never logs it
func TestShareLimitKeyCapsPerShare(t *testing.T) {
	release := make(chan struct{})
	held := make(chan struct{}, 1)
	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(middleware.NewConcurrencyLimiterBy(1, shareLimitKey).Middleware)
		r.Get("/api/public/s/{token}/dl", func(w http.ResponseWriter, r *http.Request) {
			held <- struct{}{}
			<-release
		})
	})
	go r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/public/s/onyx_aaa/dl", nil))
	<-held

	same := httptest.NewRecorder()
	r.ServeHTTP(same, httptest.NewRequest(http.MethodGet, "/api/public/s/onyx_aaa/dl", nil))
	if same.Code != http.StatusTooManyRequests {
		t.Fatalf("second download of a full share got %d, want 429", same.Code)
	}
	close(release)

	other := httptest.NewRecorder()
	r.ServeHTTP(other, httptest.NewRequest(http.MethodGet, "/api/public/s/onyx_bbb/dl", nil))
	if other.Code != http.StatusOK {
		t.Fatalf("another share got %d, want its own budget", other.Code)
	}
	if k := shareLimitKey(httptest.NewRequest(http.MethodGet, "/", nil)); len(k) != len("share:")+16 {
		t.Fatalf("key %q is not a short hash", k)
	}
}
