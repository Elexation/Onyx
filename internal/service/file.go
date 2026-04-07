package service

import (
	"io"
	"sort"
	"strings"
	"time"

	"github.com/Elexation/onyx/internal/adapter/storage"
	"github.com/Elexation/onyx/internal/domain"
)

type FileService struct {
	storage *storage.LocalStorage
}

func NewFileService(storage *storage.LocalStorage) *FileService {
	return &FileService{storage: storage}
}

// ListDirectory returns the contents of a directory, optionally filtering
// hidden files and sorting directories first then by name.
func (s *FileService) ListDirectory(dirPath string, showHidden bool) ([]domain.FileInfo, error) {
	items, err := s.storage.ListDir(dirPath)
	if err != nil {
		return nil, err
	}

	if !showHidden {
		filtered := items[:0]
		for _, item := range items {
			if !strings.HasPrefix(item.Name, ".") {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})

	return items, nil
}

// GetFileInfo returns metadata for a single path.
func (s *FileService) GetFileInfo(filePath string) (*domain.FileInfo, error) {
	return s.storage.Stat(filePath)
}

// OpenFile returns a reader, mod time, and size for serving a file.
func (s *FileService) OpenFile(filePath string) (io.ReadSeekCloser, time.Time, int64, error) {
	return s.storage.Open(filePath)
}
