package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/Elexation/onyx/internal/domain"
)

type ShareRepo interface {
	Create(tokenHash, tokenLast8, filePath string, isDir bool, createdAt int64, expiresAt *int64, passwordHash *string) (int64, error)
	GetByTokenHash(tokenHash string) (*domain.ShareLink, *string, error)
	List() ([]domain.ShareLink, error)
	Delete(id int64) error
	IncrementDownloadCount(id int64) error
	DeleteExpired(now int64) (int64, error)
}

type ShareService struct {
	repo     ShareRepo
	settings *SettingsService
}

func NewShareService(repo ShareRepo, settings *SettingsService) *ShareService {
	return &ShareService{repo: repo, settings: settings}
}

func (s *ShareService) Create(filePath string, isDir bool, expiresIn *time.Duration, password string) (*domain.ShareLink, string, error) {
	enabledStr, _ := s.settings.Get(domain.SettingSharesEnabled)
	if !domain.GetBool(enabledStr) {
		return nil, "", fmt.Errorf("sharing is disabled")
	}

	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, "", fmt.Errorf("generate token: %w", err)
	}
	fullToken := "onyx_" + base64.RawURLEncoding.EncodeToString(tokenBytes)

	hash := sha256.Sum256([]byte(fullToken))
	tokenHash := hex.EncodeToString(hash[:])
	tokenLast8 := fullToken[len(fullToken)-8:]

	now := time.Now().Unix()

	var expiresAt *int64
	if expiresIn != nil {
		exp := now + int64(expiresIn.Seconds())
		expiresAt = &exp
	} else {
		defaultStr, _ := s.settings.Get(domain.SettingSharesDefaultExpiry)
		d := domain.GetDuration(defaultStr)
		if d > 0 {
			exp := now + int64(d.Seconds())
			expiresAt = &exp
		}
	}

	var pwHash *string
	if password != "" {
		h, err := hashPassword(password)
		if err != nil {
			return nil, "", fmt.Errorf("hash share password: %w", err)
		}
		pwHash = &h
	}

	id, err := s.repo.Create(tokenHash, tokenLast8, filePath, isDir, now, expiresAt, pwHash)
	if err != nil {
		return nil, "", err
	}

	link := &domain.ShareLink{
		ID:          id,
		TokenLast8:  tokenLast8,
		FilePath:    filePath,
		IsDir:       isDir,
		CreatedAt:   now,
		HasPassword: pwHash != nil,
	}
	if expiresAt != nil {
		link.ExpiresAt = *expiresAt
	}

	return link, fullToken, nil
}

func (s *ShareService) Validate(token string) (*domain.ShareLink, *string, error) {
	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	link, pwHash, err := s.repo.GetByTokenHash(tokenHash)
	if err != nil {
		return nil, nil, err
	}
	if link == nil {
		return nil, nil, nil
	}

	if link.ExpiresAt > 0 && link.ExpiresAt < time.Now().Unix() {
		return nil, nil, nil
	}

	return link, pwHash, nil
}

func (s *ShareService) CheckPassword(pwHash, password string) bool {
	return verifyPassword(password, pwHash)
}

func (s *ShareService) List() ([]domain.ShareLink, error) {
	return s.repo.List()
}

func (s *ShareService) Delete(id int64) error {
	return s.repo.Delete(id)
}

func (s *ShareService) RecordAccess(id int64) {
	if err := s.repo.IncrementDownloadCount(id); err != nil {
		slog.Warn("failed to increment share download count", "id", id, "error", err)
	}
}

func (s *ShareService) CleanExpired() {
	count, err := s.repo.DeleteExpired(time.Now().Unix())
	if err != nil {
		slog.Warn("share cleanup failed", "error", err)
		return
	}
	if count > 0 {
		slog.Info("cleaned up expired shares", "count", count)
	}
}

func (s *ShareService) StartCleanup(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			s.CleanExpired()
		}
	}()
}
