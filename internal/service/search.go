package service

import (
	"strings"

	"github.com/Elexation/onyx/internal/domain"
)

type SearchService struct {
	repo SearchRepo
}

func NewSearchService(repo SearchRepo) *SearchService {
	return &SearchService{repo: repo}
}

func (s *SearchService) Search(query string, limit int) ([]domain.SearchResult, int, error) {
	if len(strings.TrimSpace(query)) < 2 {
		return nil, 0, nil
	}
	return s.repo.Search(query, limit)
}
