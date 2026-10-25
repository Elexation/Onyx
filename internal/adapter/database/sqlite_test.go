package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildDSN_EscapesURIMetacharacters(t *testing.T) {
	dsn := buildDSN(filepath.Join("cfg #1", "a?b%41.db"))
	path, _, _ := strings.Cut(strings.TrimPrefix(dsn, "file:"), "?")
	if strings.ContainsAny(path, "#?") || strings.Contains(path, "%41") {
		t.Fatalf("path part not escaped: %q", path)
	}
}

func TestOpen_PathWithHash(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "conf #x")
	db, err := Open(filepath.Join(dir, "onyx.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	db.Close()
	if _, err := os.Stat(filepath.Join(dir, "onyx.db")); err != nil {
		t.Fatalf("database not created at the requested path: %v", err)
	}
}
