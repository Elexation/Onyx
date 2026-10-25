package service

import (
	"context"
	"strings"

	"github.com/Elexation/onyx/internal/domain"
)

// MaxSearchQueryLen caps the q parameter; each token becomes an FTS5 prefix phrase.
const MaxSearchQueryLen = 256

type SearchService struct {
	repo SearchRepo
}

func NewSearchService(repo SearchRepo) *SearchService {
	return &SearchService{repo: repo}
}

func (s *SearchService) Search(ctx context.Context, query string, limit int) ([]domain.SearchResult, int, error) {
	if len(strings.TrimSpace(query)) < 1 {
		return nil, 0, nil
	}
	return s.repo.Search(ctx, query, limit)
}
