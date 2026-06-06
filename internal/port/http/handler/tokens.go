package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/Elexation/onyx/internal/domain"
	"github.com/Elexation/onyx/internal/service"
)

type TokenHandler struct {
	tokens *service.TokenService
}

func NewTokenHandler(tokens *service.TokenService) *TokenHandler {
	return &TokenHandler{tokens: tokens}
}

// Create handles POST /api/tokens
func (h *TokenHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string `json:"name"`
		Scope     string `json:"scope"`
		ExpiresAt *int64 `json:"expiresAt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if !domain.IsValidTokenScope(req.Scope) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid scope"})
		return
	}

	tok, fullToken, err := h.tokens.Create(req.Name, req.Scope, req.ExpiresAt)
	if err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "maximum"):
			writeJSON(w, http.StatusConflict, map[string]string{"error": msg})
		case msg == "expiration must be in the future":
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		default:
			slog.Warn("token create failed", "name", req.Name, "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create token"})
		}
		return
	}

	slog.Info("security_event", "event", "token_create", "name", req.Name)
	tok.Token = fullToken
	writeJSON(w, http.StatusCreated, tok)
}

// List handles GET /api/tokens
func (h *TokenHandler) List(w http.ResponseWriter, r *http.Request) {
	tokens, err := h.tokens.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list tokens"})
		return
	}
	if tokens == nil {
		tokens = []domain.PersonalAccessToken{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"tokens": tokens, "max": service.MaxActiveTokens})
}

// Delete handles DELETE /api/tokens/{id}
func (h *TokenHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	if err := h.tokens.Delete(id); err != nil {
		slog.Warn("token delete failed", "id", id, "error", err)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "token not found"})
		return
	}

	slog.Info("security_event", "event", "token_delete", "id", id)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
