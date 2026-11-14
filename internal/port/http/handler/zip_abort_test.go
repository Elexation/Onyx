package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Elexation/onyx/internal/adapter/storage"
	"github.com/Elexation/onyx/internal/port/http/middleware"
	"github.com/Elexation/onyx/internal/service"
)

// A zip that fails midway must reach the client as a failed transfer, not a 200 that looks complete
func TestDownloadZipAbortsOnFailure(t *testing.T) {
	dir := t.TempDir()
	folder := filepath.Join(dir, "folder")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "a.txt"), make([]byte, 256<<10), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "missing"), filepath.Join(folder, "b-dangling")); err != nil {
		t.Skipf("symlinks not available here: %v", err)
	}
	ls, err := storage.NewLocalStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer ls.Close()

	h := NewFileHandler(service.NewFileService(ls))
	srv := httptest.NewServer(middleware.Recovery(http.HandlerFunc(h.DownloadZip)))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/download/zip?path=/folder")
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if _, err := io.ReadAll(resp.Body); err == nil {
		t.Fatalf("client read a complete %d response from a failed zip", resp.StatusCode)
	}
}
