package storage

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// ThumbStore manages cached thumbnail files on disk. The layout is sharded
// by the first 2 hex characters of the SHA-256 hash to avoid piling millions
// of files into one directory. Filenames encode both mtime and size so they
// are self-invalidating: a different mtime produces a different filename.
//
//	{root}/{hash[:2]}/{hash[:16]}-{mtime}-{size}.jpg
//	{root}/{hash[:2]}/{hash[:16]}-{mtime}-{size}.fail
type ThumbStore struct {
	root string
}

// NewThumbStore creates the thumbnail cache directory if missing.
func NewThumbStore(root string) (*ThumbStore, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create thumb dir %q: %w", root, err)
	}
	return &ThumbStore{root: root}, nil
}

// Root returns the cache root directory.
func (ts *ThumbStore) Root() string { return ts.root }

// Path computes the on-disk path for a thumbnail with the given hash,
// mtime, and size label. Does not touch the filesystem.
func (ts *ThumbStore) Path(hash []byte, mtime int64, size string) string {
	h := hex.EncodeToString(hash)
	return filepath.Join(ts.root, h[:2], fmt.Sprintf("%s-%d-%s.jpg", h[:16], mtime, size))
}

// FailPath is Path with a .fail suffix for negative caching.
func (ts *ThumbStore) FailPath(hash []byte, mtime int64, size string) string {
	h := hex.EncodeToString(hash)
	return filepath.Join(ts.root, h[:2], fmt.Sprintf("%s-%d-%s.fail", h[:16], mtime, size))
}

// Exists returns true if a file exists at p.
func (ts *ThumbStore) Exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// EnsureDir makes sure the parent directory of p exists.
func (ts *ThumbStore) EnsureDir(p string) error {
	return os.MkdirAll(filepath.Dir(p), 0o755)
}

// SaveAtomic writes data to a temp file in the same directory and renames
// it into place, preventing half-written thumbnails from being served.
func (ts *ThumbStore) SaveAtomic(dst string, write func(f *os.File) error) error {
	if err := ts.EnsureDir(dst); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".thumb-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() {
		tmp.Close()
		os.Remove(tmpName)
	}
	if err := write(tmp); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, dst); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

// WriteFailMarker creates an empty .fail file; its mtime is the attempt time.
func (ts *ThumbStore) WriteFailMarker(p string) error {
	if err := ts.EnsureDir(p); err != nil {
		return err
	}
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	return f.Close()
}

// IsFailFresh returns true if a .fail marker exists at p and is newer than ttl.
func (ts *ThumbStore) IsFailFresh(p string, ttl time.Duration) bool {
	info, err := os.Stat(p)
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) < ttl
}

// Touch updates the atime/mtime of a thumbnail file to now. Used so the
// LRU janitor can tell which thumbs are actively being served.
func (ts *ThumbStore) Touch(p string) {
	now := time.Now()
	_ = os.Chtimes(p, now, now)
}

// Walk visits every regular file under the cache root. Errors on individual
// entries are ignored.
func (ts *ThumbStore) Walk(fn func(path string, info fs.FileInfo)) error {
	return filepath.WalkDir(ts.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		fn(p, info)
		return nil
	})
}

// Remove deletes a file from the cache. Missing files are not an error.
func (ts *ThumbStore) Remove(p string) {
	_ = os.Remove(p)
}
