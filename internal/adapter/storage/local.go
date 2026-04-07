package storage

import (
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/Elexation/onyx/internal/domain"
)

// LocalStorage provides traversal-safe filesystem access through os.Root.
type LocalStorage struct {
	root *os.Root
}

// NewLocalStorage opens the given directory as a confined root.
// All subsequent operations are sandboxed — path traversal is impossible.
func NewLocalStorage(dataPath string) (*LocalStorage, error) {
	root, err := os.OpenRoot(dataPath)
	if err != nil {
		return nil, fmt.Errorf("open root %q: %w", dataPath, err)
	}
	return &LocalStorage{root: root}, nil
}

// Close releases the root handle.
func (s *LocalStorage) Close() error {
	return s.root.Close()
}

// ListDir returns the contents of a directory.
func (s *LocalStorage) ListDir(dirPath string) ([]domain.FileInfo, error) {
	dirPath = cleanPath(dirPath)

	f, err := s.root.Open(dirPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	entries, err := f.ReadDir(-1)
	if err != nil {
		return nil, err
	}

	items := make([]domain.FileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		itemPath := path.Join(dirPath, entry.Name())
		fi := domain.FileInfo{
			Name:    entry.Name(),
			Path:    "/" + itemPath,
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().Unix(),
		}

		if !entry.IsDir() {
			fi.MIMEType = s.detectFileMIME(itemPath, entry.Name())
		}

		items = append(items, fi)
	}

	return items, nil
}

// Stat returns metadata for a single path.
func (s *LocalStorage) Stat(filePath string) (*domain.FileInfo, error) {
	filePath = cleanPath(filePath)

	info, err := s.root.Stat(filePath)
	if err != nil {
		return nil, err
	}

	fi := &domain.FileInfo{
		Name:    info.Name(),
		Path:    "/" + filePath,
		IsDir:   info.IsDir(),
		Size:    info.Size(),
		ModTime: info.ModTime().Unix(),
	}

	if !info.IsDir() {
		fi.MIMEType = s.detectFileMIME(filePath, info.Name())
	}

	return fi, nil
}

// Open returns a ReadSeekCloser for the file at the given path,
// along with its modification time and size (for http.ServeContent).
func (s *LocalStorage) Open(filePath string) (io.ReadSeekCloser, time.Time, int64, error) {
	filePath = cleanPath(filePath)

	f, err := s.root.Open(filePath)
	if err != nil {
		return nil, time.Time{}, 0, err
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, time.Time{}, 0, err
	}

	if info.IsDir() {
		f.Close()
		return nil, time.Time{}, 0, fmt.Errorf("not a file: %s", filePath)
	}

	return f, info.ModTime(), info.Size(), nil
}

// detectFileMIME detects MIME for a file, reading header bytes if the
// extension map doesn't have a match.
func (s *LocalStorage) detectFileMIME(filePath, name string) string {
	ext := strings.ToLower(path.Ext(name))
	if ext != "" {
		if mime := DetectMIME(name, nil); mime != "application/octet-stream" {
			return mime
		}
	}

	// Extension unknown — read first 512 bytes for magic-byte detection
	f, err := s.root.Open(filePath)
	if err != nil {
		return "application/octet-stream"
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	if n == 0 {
		return "application/octet-stream"
	}

	return DetectMIME(name, buf[:n])
}

// cleanPath normalizes a request path for use with os.Root.
func cleanPath(p string) string {
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return "."
	}
	return path.Clean(p)
}
