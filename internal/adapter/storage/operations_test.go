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
