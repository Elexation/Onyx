//go:build !windows

package storage

// LongPath returns p as is; short-name aliases exist only on Windows
func (s *LocalStorage) LongPath(p string) (string, error) {
	return p, nil
}
