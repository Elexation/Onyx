package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
	code, err := auth.InitSetupCode("")
	if err != nil {
		t.Fatal(err)
	}
	session, err := auth.Setup("correct horse battery", code)
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

// Guessing spread over many addresses hits the shared limit, but a browser that logged in before gets through
func TestRouterLoginSharedLimitAndDeviceCookie(t *testing.T) {
	router, _ := newCacheTestRouter(t)
	login := func(remote, password string, device *http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "http://x/api/auth/login", strings.NewReader(`{"password":"`+password+`"}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = remote
		if device != nil {
			req.AddCookie(device)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	rec := login("198.51.100.1:1000", "correct horse battery", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d", rec.Code)
	}
	var device *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "onyx_device" {
			device = c
		}
	}
	if device == nil || device.Path != "/api/auth" || !device.HttpOnly || device.SameSite != http.SameSiteStrictMode || device.MaxAge <= 0 {
		t.Fatalf("device cookie = %+v, want HttpOnly, SameSite=Strict, Path=/api/auth, long-lived", device)
	}

	for i := 0; i < 20; i++ {
		if rec := login(fmt.Sprintf("203.0.113.%d:1000", i+1), "wrong guess", nil); rec.Code != http.StatusUnauthorized {
			t.Fatalf("guess %d status = %d, want 401", i+1, rec.Code)
		}
	}
	rec = login("203.0.113.200:1000", "correct horse battery", nil)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("unknown browser: status = %d Retry-After = %q, want 429 with Retry-After", rec.Code, rec.Header().Get("Retry-After"))
	}
	if rec := login("203.0.113.201:1000", "correct horse battery", &http.Cookie{Name: "onyx_device", Value: device.Value}); rec.Code != http.StatusOK {
		t.Fatalf("known browser status = %d, want 200", rec.Code)
	}
}

// A fresh install answers setup only to a caller holding the code from the server log
func TestRouterSetupRequiresCode(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "onyx.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	settings := service.NewSettingsService(database.NewSettingsRepo(db))
	auth := service.NewAuthService(database.NewUserRepo(db), database.NewSessionRepo(db), settings)
	code, err := auth.InitSetupCode("")
	if err != nil {
		t.Fatal(err)
	}
	router := NewRouter(auth, nil, settings, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, false, false, false, "8080", map[string]string{})

	setup := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "http://x/api/auth/setup", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	for name, body := range map[string]string{
		"no code":    `{"password":"correct horse battery"}`,
		"wrong code": `{"password":"correct horse battery","setupCode":"AAAA-BBBB"}`,
	} {
		if rec := setup(body); rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "invalid setup code") {
			t.Fatalf("%s: status = %d body = %s, want 403 invalid setup code", name, rec.Code, rec.Body.String())
		}
	}
	if rec := setup(`{"password":"correct horse battery","setupCode":"` + code + `"}`); rec.Code != http.StatusOK {
		t.Fatalf("setup with code: status = %d body = %s", rec.Code, rec.Body.String())
	}
	if first, _ := auth.IsFirstRun(); first {
		t.Fatal("admin not created")
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
