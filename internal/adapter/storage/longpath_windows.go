//go:build windows

package storage

import (
	"errors"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// LongPath expands 8.3 short names (DATA~1) in p to the names on disk
func (s *LocalStorage) LongPath(p string) (string, error) {
	rel := cleanPath(p)
	if rel == "." {
		return "/", nil
	}
	base, err := filepath.Abs(s.dataPath)
	if err != nil {
		return "", err
	}
	src, err := windows.UTF16PtrFromString(filepath.Join(base, filepath.FromSlash(rel)))
	if err != nil {
		return "", err
	}
	buf := make([]uint16, windows.MAX_PATH)
	n, err := windows.GetLongPathName(src, &buf[0], uint32(len(buf)))
	if err == nil && int(n) > len(buf) {
		buf = make([]uint16, n)
		n, err = windows.GetLongPathName(src, &buf[0], uint32(len(buf)))
	}
	if err != nil {
		return "", err
	}
	// Expansion keeps the component count, so the last ones are p's
	long := strings.Split(windows.UTF16ToString(buf[:n]), `\`)
	segs := strings.Split(rel, "/")
	if len(long) < len(segs) {
		return "", errors.New("long path has fewer components than the request")
	}
	return "/" + strings.Join(long[len(long)-len(segs):], "/"), nil
}
