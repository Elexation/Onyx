package server

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Elexation/onyx/internal/adapter/database"
	"github.com/Elexation/onyx/internal/adapter/storage"
	"github.com/Elexation/onyx/internal/adapter/upload"
	"github.com/Elexation/onyx/internal/domain"
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

// A browser signed in before device cookies existed gets one on its next page load; others don't
func TestRouterStatusIssuesDeviceCookie(t *testing.T) {
	router, session := newCacheTestRouter(t)
	status := func(cookies ...*http.Cookie) *http.Cookie {
		req := httptest.NewRequest(http.MethodGet, "http://x/api/auth/status", nil)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d", rec.Code)
		}
		for _, c := range rec.Result().Cookies() {
			if c.Name == "onyx_device" {
				return c
			}
		}
		return nil
	}

	if c := status(); c != nil {
		t.Fatal("signed-out browser got a device cookie")
	}
	issued := status(session)
	if issued == nil || issued.Path != "/api/auth" || !issued.HttpOnly {
		t.Fatalf("signed-in browser without one got %+v, want a device cookie", issued)
	}
	if c := status(session, &http.Cookie{Name: "onyx_device", Value: issued.Value}); c != nil {
		t.Fatal("re-issued a device cookie to a browser that already had a valid one")
	}
	if c := status(session, &http.Cookie{Name: "onyx_device", Value: "forged.value"}); c == nil {
		t.Fatal("forged device cookie was not replaced")
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

// A leaked token can't make lasting shares, destroy data past the trash, or overwrite files with upload scope
func TestRouterTokenLimits(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"keep.txt", "trashed.txt", "notrash.txt", "cookie.txt", "existing.txt"} {
		if err := os.WriteFile(filepath.Join(dataDir, name), []byte("original"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	exists := func(name string) bool {
		_, err := os.Stat(filepath.Join(dataDir, name))
		return err == nil
	}

	db, err := database.Open(filepath.Join(dir, "onyx.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	settings := service.NewSettingsService(database.NewSettingsRepo(db))
	auth := service.NewAuthService(database.NewUserRepo(db), database.NewSessionRepo(db), settings)
	code, _ := auth.InitSetupCode("")
	session, err := auth.Setup("correct horse battery", code)
	if err != nil {
		t.Fatal(err)
	}
	tokens := service.NewTokenService(database.NewTokenRepo(db))
	_, fullTok, err := tokens.Create("full", domain.ScopeFull, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, uploadTok, err := tokens.Create("upload", domain.ScopeUpload, nil)
	if err != nil {
		t.Fatal(err)
	}

	ls, err := storage.NewLocalStorage(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ls.Close() })
	files := service.NewFileService(ls)
	trash, err := service.NewTrashService(database.NewTrashRepo(db), settings, dataDir, filepath.Join(dir, ".trash"))
	if err != nil {
		t.Fatal(err)
	}
	trash.SetPathLocker(files.LockPath)
	files.SetTrash(trash, settings)
	tus, err := upload.NewTusHandler(filepath.Join(dir, "uploads"), "/api/upload/", files, settings, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tus.Close() })

	router := NewRouter(auth, files, settings, trash, nil, tus, nil, nil, tokens, nil, nil, nil, nil, false, false, false, "8080", map[string]string{})
	do := func(method, target, token, body string, hdr map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "http://x"+target, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		} else {
			req.AddCookie(&http.Cookie{Name: "session", Value: session.ID})
			req.Header.Set("X-CSRF-Token", session.CSRFToken)
		}
		if body != "" && hdr == nil {
			req.Header.Set("Content-Type", "application/json")
		}
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	expect := func(name string, rec *httptest.ResponseRecorder, want int) {
		t.Helper()
		if rec.Code != want {
			t.Errorf("%s: status = %d, want %d (body %s)", name, rec.Code, want, strings.TrimSpace(rec.Body.String()))
		}
	}

	expect("create share", do("POST", "/api/shares", fullTok, `{"path":"/keep.txt"}`, nil), http.StatusForbidden)
	expect("purge trash item", do("DELETE", "/api/trash/1", fullTok, "", nil), http.StatusForbidden)
	expect("empty trash", do("DELETE", "/api/trash", fullTok, "", nil), http.StatusForbidden)
	expect("delete version", do("DELETE", "/api/versions/1", fullTok, "", nil), http.StatusForbidden)

	expect("permanent delete", do("DELETE", "/api/files", fullTok, `{"paths":["/keep.txt"],"permanent":true}`, nil), http.StatusForbidden)
	if !exists("keep.txt") {
		t.Fatal("token permanently deleted a file")
	}
	expect("delete to trash", do("DELETE", "/api/files", fullTok, `{"paths":["/trashed.txt"]}`, nil), http.StatusOK)
	if exists("trashed.txt") {
		t.Fatal("trash delete via token did not move the file")
	}

	if err := settings.Set(domain.SettingTrashEnabled, "false"); err != nil {
		t.Fatal(err)
	}
	expect("token delete, trash off", do("DELETE", "/api/files", fullTok, `{"paths":["/notrash.txt"]}`, nil), http.StatusConflict)
	if !exists("notrash.txt") {
		t.Fatal("token deleted a file permanently with trash off")
	}
	expect("browser delete, trash off", do("DELETE", "/api/files", "", `{"paths":["/cookie.txt"]}`, nil), http.StatusConflict)
	expect("browser permanent delete", do("DELETE", "/api/files", "", `{"paths":["/cookie.txt"],"permanent":true}`, nil), http.StatusOK)
	if exists("cookie.txt") {
		t.Fatal("browser permanent delete did not remove the file")
	}

	b64 := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	create := func(token, strategy string) *httptest.ResponseRecorder {
		meta := "name " + b64("existing.txt") + ",targetDir " + b64("/") + ",conflictStrategy " + b64(strategy)
		return do("POST", "/api/upload/", token, "", map[string]string{"Tus-Resumable": "1.0.0", "Upload-Length": "3", "Upload-Metadata": meta})
	}
	expect("upload token replace", create(uploadTok, "replace"), http.StatusForbidden)
	kept := create(uploadTok, "keepBoth")
	expect("upload token keepBoth", kept, http.StatusCreated)
	// tus download is off: staged bytes are never served back
	staged := "/api/upload/" + path.Base(kept.Header().Get("Location"))
	expect("upload token reads a staged upload", do("GET", staged, uploadTok, "", map[string]string{"Tus-Resumable": "1.0.0"}), http.StatusMethodNotAllowed)
	expect("browser reads a staged upload", do("GET", staged, "", "", map[string]string{"Tus-Resumable": "1.0.0"}), http.StatusMethodNotAllowed)

	rec := create(fullTok, "replace")
	expect("full token replace create", rec, http.StatusCreated)
	id := path.Base(rec.Header().Get("Location"))
	patch := do("PATCH", "/api/upload/"+id, uploadTok, "new", map[string]string{
		"Tus-Resumable": "1.0.0", "Upload-Offset": "0", "Content-Type": "application/offset+octet-stream",
	})
	expect("upload token finishes a replace", patch, http.StatusForbidden)
	if got, _ := os.ReadFile(filepath.Join(dataDir, "existing.txt")); string(got) != "original" {
		t.Fatalf("existing.txt = %q, upload token overwrote it", got)
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
