package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureNoDirOverlap_Aliases(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "Data")
	if err := os.MkdirAll(data, 0755); err != nil {
		t.Fatal(err)
	}
	if err := ensureNoDirOverlap(data, filepath.Join(root, "config")); err != nil {
		t.Fatalf("separate dirs rejected: %v", err)
	}
	if err := ensureNoDirOverlap(data, filepath.Join(data, "config")); err == nil {
		t.Fatal("config inside data accepted")
	}

	// A case-insensitive volume resolves "data/config" into Data
	if _, err := os.Stat(filepath.Join(root, "data")); err == nil {
		if err := ensureNoDirOverlap(data, filepath.Join(root, "data", "config")); err == nil {
			t.Fatal("case-aliased config inside data accepted")
		}
	}

	link := filepath.Join(root, "link")
	if err := os.Symlink(data, link); err == nil {
		if err := ensureNoDirOverlap(data, filepath.Join(link, "config")); err == nil {
			t.Fatal("config reached through a symlink into data accepted")
		}
	} else {
		t.Logf("symlink not available here: %v", err)
	}
}

func TestEnsureOutsideData_TLSKey(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "data")
	if err := os.MkdirAll(data, 0755); err != nil {
		t.Fatal(err)
	}
	if err := ensureOutsideData(data, filepath.Join(root, "certs", "tls.key")); err != nil {
		t.Fatalf("key outside data rejected: %v", err)
	}
	err := ensureOutsideData(data, filepath.Join(data, "certs", "tls.key"))
	if err == nil || !strings.Contains(err.Error(), "inside the data dir") {
		t.Fatalf("key inside data: got %v", err)
	}
}
