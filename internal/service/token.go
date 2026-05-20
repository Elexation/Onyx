package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Elexation/onyx/internal/domain"
)

const MaxActiveTokens = 50

type TokenRepo interface {
	Create(name, tokenHash, tokenLast8, scope string, createdAt int64, expiresAt *int64) (int64, error)
	GetByTokenHash(tokenHash string) (*domain.PersonalAccessToken, error)
	List() ([]domain.PersonalAccessToken, error)
	Delete(id int64) error
	Count() (int64, error)
	UpdateLastUsed(id, ts int64) error
	DeleteExpired(now int64) (int64, error)
}

type TokenService struct {
	repo TokenRepo
}

func NewTokenService(repo TokenRepo) *TokenService {
	return &TokenService{repo: repo}
}

// Create generates a new PAT. expiresAt is an absolute unix seconds timestamp;
// pass nil for no expiry.
func (s *TokenService) Create(name, scope string, expiresAt *int64) (*domain.PersonalAccessToken, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, "", fmt.Errorf("name is required")
	}
	if !domain.IsValidTokenScope(scope) {
		return nil, "", fmt.Errorf("invalid scope")
	}

	count, err := s.repo.Count()
	if err != nil {
		return nil, "", fmt.Errorf("count tokens: %w", err)
	}
	if count >= MaxActiveTokens {
		return nil, "", fmt.Errorf("maximum of %d active tokens reached — revoke one to create another", MaxActiveTokens)
	}

	now := time.Now().Unix()
	if expiresAt != nil && *expiresAt <= now {
		return nil, "", fmt.Errorf("expiration must be in the future")
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, "", fmt.Errorf("generate token: %w", err)
	}
	fullToken := "onyx_" + base64.RawURLEncoding.EncodeToString(tokenBytes)

	hash := sha256.Sum256([]byte(fullToken))
	tokenHash := hex.EncodeToString(hash[:])
	tokenLast8 := fullToken[len(fullToken)-8:]

	id, err := s.repo.Create(name, tokenHash, tokenLast8, scope, now, expiresAt)
	if err != nil {
		return nil, "", err
	}

	tok := &domain.PersonalAccessToken{
		ID:         id,
		Name:       name,
		TokenLast8: tokenLast8,
		Scope:      scope,
		CreatedAt:  now,
	}
	if expiresAt != nil {
		tok.ExpiresAt = *expiresAt
	}
	return tok, fullToken, nil
}

// ValidateToken takes a raw token string, verifies it against the database,
// checks expiry, and updates last_used_at. Returns nil, nil if the token
// does not exist or is expired.
func (s *TokenService) ValidateToken(token string) (*domain.PersonalAccessToken, error) {
	if !strings.HasPrefix(token, "onyx_") {
		return nil, nil
	}
	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	tok, err := s.repo.GetByTokenHash(tokenHash)
	if err != nil {
		return nil, err
	}
	if tok == nil {
		return nil, nil
	}

	now := time.Now().Unix()
	if tok.ExpiresAt > 0 && tok.ExpiresAt < now {
		return nil, nil
	}

	if err := s.repo.UpdateLastUsed(tok.ID, now); err != nil {
		slog.Warn("failed to update token last_used_at", "id", tok.ID, "error", err)
	}
	tok.LastUsedAt = now

	return tok, nil
}

func (s *TokenService) List() ([]domain.PersonalAccessToken, error) {
	return s.repo.List()
}

func (s *TokenService) Delete(id int64) error {
	return s.repo.Delete(id)
}

func (s *TokenService) Count() (int64, error) {
	return s.repo.Count()
}

// CheckScope is a method wrapper over the package-level CheckScope helper so
// that TokenService satisfies the TokenValidator interface consumed by the
// auth middleware without forcing the middleware to import this package.
func (s *TokenService) CheckScope(scope, method, path string) bool {
	return CheckScope(scope, method, path)
}

func (s *TokenService) CleanExpired() {
	n, err := s.repo.DeleteExpired(time.Now().Unix())
	if err != nil {
		slog.Warn("token cleanup failed", "error", err)
		return
	}
	if n > 0 {
		slog.Info("cleaned up expired tokens", "count", n)
	}
}

func (s *TokenService) StartCleanup(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			s.CleanExpired()
		}
	}()
}

// CheckScope decides whether a given (scope, method, path) combination is
// allowed for a bearer-authenticated request. Admin-only endpoints
// (/api/tokens, /api/auth, /api/settings) are blocked for all bearer tokens
// regardless of scope — they require a real browser session.
func CheckScope(scope, method, path string) bool {
	// Admin endpoints are never accessible via bearer token, even with full scope.
	// A leaked token must not be able to create more tokens, change the password,
	// or alter server settings.
	if strings.HasPrefix(path, "/api/tokens") ||
		strings.HasPrefix(path, "/api/auth") ||
		strings.HasPrefix(path, "/api/settings") {
		return false
	}

	switch scope {
	case domain.ScopeFull:
		return true

	case domain.ScopeRead:
		return method == http.MethodGet || method == http.MethodHead

	case domain.ScopeUpload:
		// Reads needed to navigate and locate upload targets.
		if method == http.MethodGet || method == http.MethodHead {
			return true
		}
		// tus resumable-upload protocol uses POST (create), PATCH (append),
		// HEAD (resume), DELETE (cancel) on /api/upload/*.
		if strings.HasPrefix(path, "/api/upload") {
			return true
		}
		// Directory creation is needed so upload scripts can create their
		// target folders before uploading.
		if method == http.MethodPost && (path == "/api/files/mkdir" || path == "/api/files/check-conflicts") {
			return true
		}
		return false
	}
	return false
}
