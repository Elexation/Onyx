package storage

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"syscall"
)

// MakeDir creates a single directory. The parent must already exist.
func (s *LocalStorage) MakeDir(dirPath string) error {
	dirPath = cleanPath(dirPath)
	return s.root.Mkdir(dirPath, 0755)
}

// Rename changes the name of a file or directory within the same parent.
// newName must not contain path separators.
func (s *LocalStorage) Rename(filePath, newName string) error {
	filePath = cleanPath(filePath)
	parent := path.Dir(filePath)
	newPath := path.Join(parent, newName)
	return s.root.Rename(filePath, newPath)
}

// Move relocates paths into a destination directory.
// If os.Rename fails with EXDEV (cross-device), it falls back to copy+delete.
// Returns per-item results.
func (s *LocalStorage) Move(paths []string, destination string) []OpResult {
	destination = cleanPath(destination)
	results := make([]OpResult, len(paths))

	for i, p := range paths {
		p = cleanPath(p)
		name := path.Base(p)
		dst := path.Join(destination, name)

		err := s.root.Rename(p, dst)
		if err != nil && isCrossDevice(err) {
			// Cross-device: copy then delete original
			err = s.copyOne(p, dst)
			if err == nil {
				err = s.root.RemoveAll(p)
			}
		}

		results[i] = OpResult{Path: "/" + p, Success: err == nil}
		if err != nil {
			results[i].Error = err.Error()
		}
	}

	return results
}

// Copy duplicates paths into a destination directory.
// Files are streamed; directories are copied recursively.
// If the destination name already exists, a unique name is generated.
// Returns per-item results.
func (s *LocalStorage) Copy(paths []string, destination string) []OpResult {
	destination = cleanPath(destination)
	results := make([]OpResult, len(paths))

	for i, p := range paths {
		p = cleanPath(p)
		name := path.Base(p)
		dst := path.Join(destination, name)
		dst = s.uniqueName(dst)

		err := s.copyOne(p, dst)
		results[i] = OpResult{Path: "/" + dst, Success: err == nil}
		if err != nil {
			results[i].Error = err.Error()
		}
	}

	return results
}

// uniqueName returns dst unchanged if it doesn't exist, otherwise appends
// " (copy)", " (copy 2)", etc. until a free name is found.
func (s *LocalStorage) uniqueName(dst string) string {
	if _, err := s.root.Lstat(dst); err != nil {
		return dst
	}

	dir := path.Dir(dst)
	base := path.Base(dst)
	ext := path.Ext(base)
	name := base[:len(base)-len(ext)]

	candidate := path.Join(dir, name+" (copy)"+ext)
	if _, err := s.root.Lstat(candidate); err != nil {
		return candidate
	}

	for i := 2; i <= 99; i++ {
		candidate = path.Join(dir, fmt.Sprintf("%s (copy %d)%s", name, i, ext))
		if _, err := s.root.Lstat(candidate); err != nil {
			return candidate
		}
	}

	return dst
}

// Delete removes paths (files or directories, recursively).
// Returns per-item results. Reports an error if a path does not exist.
func (s *LocalStorage) Delete(paths []string) []OpResult {
	results := make([]OpResult, len(paths))

	for i, p := range paths {
		p = cleanPath(p)

		// Check existence first — RemoveAll is idempotent on missing paths
		if _, err := s.root.Lstat(p); err != nil {
			results[i] = OpResult{Path: "/" + p, Success: false, Error: err.Error()}
			continue
		}

		err := s.root.RemoveAll(p)
		results[i] = OpResult{Path: "/" + p, Success: err == nil}
		if err != nil {
			results[i].Error = err.Error()
		}
	}

	return results
}

// OpResult reports the outcome of a single item in a batch operation.
type OpResult struct {
	Path    string `json:"path"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// copyOne copies a single file or directory recursively from src to dst.
func (s *LocalStorage) copyOne(src, dst string) error {
	info, err := s.root.Lstat(src)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return s.copyDir(src, dst)
	}
	return s.copyFile(src, dst)
}

// copyFile streams a single file from src to dst.
func (s *LocalStorage) copyFile(src, dst string) error {
	srcFile, err := s.root.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := s.root.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copy data: %w", err)
	}

	return nil
}

// copyDir recursively copies a directory tree from src to dst.
func (s *LocalStorage) copyDir(src, dst string) error {
	if err := s.root.Mkdir(dst, 0755); err != nil {
		return fmt.Errorf("mkdir %q: %w", dst, err)
	}

	dir, err := s.root.Open(src)
	if err != nil {
		return err
	}

	entries, err := dir.ReadDir(-1)
	dir.Close()
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcChild := path.Join(src, entry.Name())
		dstChild := path.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := s.copyDir(srcChild, dstChild); err != nil {
				return err
			}
		} else {
			if err := s.copyFile(srcChild, dstChild); err != nil {
				return err
			}
		}
	}

	return nil
}

// isCrossDevice checks if an error is an EXDEV (cross-device link) error.
func isCrossDevice(err error) bool {
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return errors.Is(linkErr.Err, syscall.EXDEV)
	}
	return false
}
