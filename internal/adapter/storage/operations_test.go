package storage

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCopyIntoSelf_AliasedDestination(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "Foo", "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	s, err := NewLocalStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	dests := []string{"/Foo/sub"}
	if _, err := os.Stat(filepath.Join(dir, "foo")); err == nil {
		dests = append(dests, "/foo/sub", "/FOO")
	}
	if runtime.GOOS == "windows" {
		dests = append(dests, `/Foo\sub`, `/x\..\Foo`)
	}
	for _, d := range dests {
		for _, op := range []func([]string, string) []OpResult{s.Copy, s.Move} {
			res := op([]string{"/Foo"}, d)
			if res[0].Success || res[0].Error == "" {
				t.Errorf("destination %q: want into-itself error, got %+v", d, res[0])
			}
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "Foo", "sub", "Foo")); err == nil {
		t.Fatal("a copy of Foo landed inside itself")
	}
}

func TestMoveCopy_LockDestination(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []string{"src", "dst"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(dir, "src", f), []byte(f), 0644); err != nil {
			t.Fatal(err)
		}
	}
	s, err := NewLocalStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	var locked []string
	held := 0
	// The first lock simulates an upload landing on that name just before the copy gets it
	landed := false
	s.SetPathLocker(func(p string) func() {
		locked = append(locked, p)
		held++
		if held > 1 {
			t.Errorf("held %d locks at once", held)
		}
		if !landed && p == "dst/b.txt" {
			landed = true
			os.WriteFile(filepath.Join(dir, "dst", "b.txt"), []byte("upload"), 0644)
		}
		return func() { held-- }
	})

	if res := s.Move([]string{"/src/a.txt"}, "/dst"); !res[0].Success {
		t.Fatalf("move: %+v", res[0])
	}
	res := s.Copy([]string{"/src/b.txt"}, "/dst")
	if !res[0].Success {
		t.Fatalf("copy: %+v", res[0])
	}
	if res[0].Path == "/dst/b.txt" {
		t.Fatal("copy wrote over the name an upload took while it waited for the lock")
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "dst", "b.txt")); string(got) != "upload" {
		t.Fatalf("uploaded file was replaced: %q", got)
	}
	want := []string{"dst/a.txt", "dst/b.txt", res[0].Path[1:]}
	if len(locked) != len(want) {
		t.Fatalf("locked %v, want %v", locked, want)
	}
	for i := range want {
		if locked[i] != want[i] {
			t.Fatalf("locked %v, want %v", locked, want)
		}
	}
	if held != 0 {
		t.Fatalf("%d locks left held", held)
	}
}
