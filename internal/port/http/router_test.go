package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Elexation/onyx/internal/adapter/database"
	"github.com/Elexation/onyx/internal/adapter/storage"
	"github.com/Elexation/onyx/internal/service"
)

// tusd never validates the upload id it reads off the URL, and its filestore
// joins that id straight into a filesystem path. Paths are set directly here
// because that is what normalizePath hands downstream.
func TestValidUploadID(t *testing.T) {
	cases := []struct {
		name string
		path string
		want int
	}{
		{"create has no id", "/api/upload/", http.StatusOK},
		{"bare prefix", "/api/upload", http.StatusOK},
		{"tusd hex id", "/api/upload/0f8b2c1d4e5a6b7c8d9e0f1a", http.StatusOK},
		{"hyphen and underscore", "/api/upload/probe-1_a", http.StatusOK},
		{"backslash traversal", `/api/upload/..\..\..\Windows\Temp\pwn`, http.StatusNotFound},
		{"slash traversal", "/api/upload/../../etc/passwd", http.StatusNotFound},
		{"nested segment", "/api/upload/a/b", http.StatusNotFound},
		{"dot in id", "/api/upload/id.info", http.StatusNotFound},
		{"null byte", "/api/upload/id\x00", http.StatusNotFound},
		{"drive letter", "/api/upload/C:/Windows/Temp/pwn", http.StatusNotFound},
	}

	reached := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodHead, "http://x/api/upload/", nil)
			req.URL.Path = tc.path
			rec := httptest.NewRecorder()
			validUploadID(reached).ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

// Build the real router with only auth and files wired, so other routes must not be hit
func newCacheTestRouter(t *testing.T) (http.Handler, *http.Cookie) {
	t.Helper()
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(filepath.Join(dataDir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "docs", "secret.pdf"), []byte("%PDF-1.4 private"), 0o644); err != nil {
		t.Fatal(err)
	}

	db, err := database.Open(filepath.Join(dir, "onyx.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	settings := service.NewSettingsService(database.NewSettingsRepo(db))
	auth := service.NewAuthService(database.NewUserRepo(db), database.NewSessionRepo(db), settings)
	session, err := auth.Setup("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}

	ls, err := storage.NewLocalStorage(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ls.Close() })
	files := service.NewFileService(ls)

	router := NewRouter(auth, files, settings, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, false, false, false, "8080", map[string]string{})
	return router, &http.Cookie{Name: "session", Value: session.ID}
}

// Keep authenticated responses out of shared caches that could replay them signed out
func TestRouterAPINoStore(t *testing.T) {
	router, cookie := newCacheTestRouter(t)

	cases := []struct {
		name   string
		path   string
		auth   bool
		status int
	}{
		{"download", "/api/download/docs/secret.pdf", true, http.StatusOK},
		{"preview", "/api/preview/docs/secret.pdf", true, http.StatusOK},
		{"listing", "/api/files/docs/", true, http.StatusOK},
		{"file metadata", "/api/files/docs/secret.pdf", true, http.StatusOK},
		{"zip", "/api/download/zip?path=/docs", true, http.StatusOK},
		{"unauthenticated", "/api/download/docs/secret.pdf", false, http.StatusUnauthorized},
		{"unknown api path", "/api/nope", true, http.StatusNotFound},
		{"dot segments into api", "/static/../api/download/docs/secret.pdf", true, http.StatusOK},
		{"upload preflight", "/api/upload/", false, http.StatusNoContent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			method := http.MethodGet
			if tc.name == "upload preflight" {
				method = http.MethodOptions
			}
			req := httptest.NewRequest(method, "http://x"+tc.path, nil)
			if tc.auth {
				req.AddCookie(cookie)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
			if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
				t.Fatalf("Cache-Control = %q, want %q", got, "private, no-store")
			}
		})
	}
}

// Keep the SPA shell on its own revalidation policy
func TestRouterNoStoreSkipsSPA(t *testing.T) {
	router, _ := newCacheTestRouter(t)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://x/files/docs", nil))
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q, want %q", got, "no-cache")
	}
}

// Let a handler's own policy win, as thumbs and the change feed need
func TestAPINoStoreHandlerOverride(t *testing.T) {
	h := apiNoStore(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, max-age=86400, immutable")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://x/api/thumbs/a.jpg", nil))
	if got := rec.Header().Get("Cache-Control"); got != "private, max-age=86400, immutable" {
		t.Fatalf("Cache-Control = %q, want handler's own policy", got)
	}
}
