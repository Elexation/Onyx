package handler

import (
	"net/http"

	"github.com/Elexation/onyx/internal/domain"
	"github.com/Elexation/onyx/internal/service"
)

type SearchHandler struct {
	search *service.SearchService
}

func NewSearchHandler(search *service.SearchService) *SearchHandler {
	return &SearchHandler{search: search}
}

func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"results": []domain.SearchResult{},
			"total":   0,
		})
		return
	}

	results, total, err := h.search.Search(query, 20)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "search failed"})
		return
	}
	if results == nil {
		results = []domain.SearchResult{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"results": results,
		"total":   total,
	})
}
