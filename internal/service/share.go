package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"strings"
	"time"

	"github.com/Elexation/onyx/internal/domain"
)

type ShareRepo interface {
	Create(tokenHash, tokenLast8, filePath string, isDir bool, createdAt int64, expiresAt *int64, passwordHash *string) (int64, error)
	GetByTokenHash(tokenHash string) (*domain.ShareLink, *string, error)
	GetByPath(filePath string) (*domain.ShareLink, error)
	List() ([]domain.ShareLink, error)
	Delete(id int64) error
	IncrementDownloadCount(id int64) error
	DeleteAll() (int64, error)
	Count() (int64, error)
	DeleteExpired(now int64) (int64, error)
	DeleteByPath(p string) (int64, error)
	DeleteByPathRecursive(dirPath string) (int64, error)
	UpdatePath(oldPath, newPath string) (int64, error)
	UpdatePathRecursive(oldDir, newDir string) (int64, error)
}

// SharePathChecker is the minimal storage interface ShareService needs to
// verify that a share target exists and matches the claimed isDir flag.
// FileService satisfies this implicitly.
type SharePathChecker interface {
	GetFileInfo(filePath string) (*domain.FileInfo, error)
}

type ShareService struct {
	repo     ShareRepo
	settings *SettingsService
	files    SharePathChecker
	events   EventRecorder
}

func NewShareService(repo ShareRepo, settings *SettingsService, files SharePathChecker) *ShareService {
	return &ShareService{repo: repo, settings: settings, files: files}
}

func (s *ShareService) SetEvents(r EventRecorder) {
	s.events = r
}

func (s *ShareService) GetByPath(filePath string) (*domain.ShareLink, error) {
	link, err := s.repo.GetByPath(filePath)
	if err != nil {
		return nil, err
	}
	if link != nil && link.ExpiresAt > 0 && link.ExpiresAt < time.Now().Unix() {
		_ = s.repo.Delete(link.ID)
		recordIf(s.events, "share.changed", ShareChangedPayload{Kind: "expired", ID: link.ID})
		return nil, nil
	}
	return link, nil
}

func (s *ShareService) Create(filePath string, isDir bool, expiresIn *time.Duration, password string) (*domain.ShareLink, string, error) {
	enabledStr, _ := s.settings.Get(domain.SettingSharesEnabled)
	if !domain.GetBool(enabledStr) {
		return nil, "", fmt.Errorf("sharing is disabled")
	}

	cleaned := path.Clean(filePath)
	if cleaned == "/" || cleaned == "." || cleaned == "" {
		return nil, "", fmt.Errorf("invalid share path")
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") || strings.HasSuffix(cleaned, "/..") {
		return nil, "", fmt.Errorf("invalid share path")
	}
	info, err := s.files.GetFileInfo(cleaned)
	if err != nil {
		return nil, "", fmt.Errorf("share path not found")
	}
	if info.IsDir != isDir {
		return nil, "", fmt.Errorf("share path type mismatch")
	}
	filePath = cleaned

	existing, err := s.repo.GetByPath(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("check existing share: %w", err)
	}
	if existing != nil {
		if existing.ExpiresAt == 0 || existing.ExpiresAt >= time.Now().Unix() {
			return nil, "", fmt.Errorf("a share link already exists for this path")
		}
		_ = s.repo.Delete(existing.ID)
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
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, "", fmt.Errorf("a share link already exists for this path")
		}
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

	recordIf(s.events, "share.changed", ShareChangedPayload{Kind: "create", ID: id})
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

	// Defensive orphan check — covers OS-level deletes that bypassed the
	// cascade hooks (file removed via SFTP, etc.). Stat is cheap; the
	// uniform 403/404 elsewhere keeps us from being a token oracle.
	if _, statErr := s.files.GetFileInfo(link.FilePath); statErr != nil && errors.Is(statErr, os.ErrNotExist) {
		if delErr := s.repo.Delete(link.ID); delErr != nil {
			slog.Warn("share validate: failed to delete orphan", "id", link.ID, "path", link.FilePath, "error", delErr)
		} else {
			recordIf(s.events, "share.changed", ShareChangedPayload{Kind: "orphaned", ID: link.ID})
		}
		return nil, nil, nil
	}

	return link, pwHash, nil
}

// CheckPassword reports whether password matches the share's stored hash.
// When the link is missing or passwordless it burns the same argon2 work
// against a throwaway hash, so Verify latency cannot reveal token existence.
func (s *ShareService) CheckPassword(link *domain.ShareLink, pwHash *string, password string) bool {
	if link == nil || pwHash == nil {
		verifyPassword(password, dummyHash)
		return false
	}
	return verifyPassword(password, *pwHash)
}

func (s *ShareService) List() ([]domain.ShareLink, error) {
	return s.repo.List()
}

func (s *ShareService) Delete(id int64) error {
	if err := s.repo.Delete(id); err != nil {
		return err
	}
	recordIf(s.events, "share.changed", ShareChangedPayload{Kind: "revoke", ID: id})
	return nil
}

func (s *ShareService) RecordAccess(id int64) {
	if err := s.repo.IncrementDownloadCount(id); err != nil {
		slog.Warn("failed to increment share download count", "id", id, "error", err)
	}
}

func (s *ShareService) DeleteAll() (int64, error) {
	return s.repo.DeleteAll()
}

func (s *ShareService) Count() (int64, error) {
	return s.repo.Count()
}

func (s *ShareService) CleanExpired() {
	count, err := s.repo.DeleteExpired(time.Now().Unix())
	if err != nil {
		slog.Warn("share cleanup failed", "error", err)
		return
	}
	if count > 0 {
		slog.Info("cleaned up expired shares", "count", count)
		recordIf(s.events, "share.changed", ShareChangedPayload{Kind: "expired"})
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

// DeleteForPath removes share rows pointing at p. When isDir is true, the
// share for the directory itself AND every share for a path inside it is
// removed. Used as a cascade when files/dirs are trashed or permanently
// deleted. Returns the number of share rows removed (0 is not an error).
func (s *ShareService) DeleteForPath(p string, isDir bool) (int64, error) {
	var (
		n   int64
		err error
	)
	if isDir {
		n, err = s.repo.DeleteByPathRecursive(p)
	} else {
		n, err = s.repo.DeleteByPath(p)
	}
	if err != nil {
		return 0, err
	}
	if n > 0 {
		recordIf(s.events, "share.changed", ShareChangedPayload{Kind: "cascade"})
	}
	return n, nil
}

// RewritePath updates the file_path of any share matching oldPath. When
// isDir is true, the directory itself AND every descendant share has its
// path rewritten by replacing the oldPath prefix with newPath. Used when
// files/dirs are renamed or moved so existing share links survive the move.
func (s *ShareService) RewritePath(oldPath, newPath string, isDir bool) (int64, error) {
	var (
		n   int64
		err error
	)
	if isDir {
		n, err = s.repo.UpdatePathRecursive(oldPath, newPath)
	} else {
		n, err = s.repo.UpdatePath(oldPath, newPath)
	}
	if err != nil {
		return 0, err
	}
	if n > 0 {
		recordIf(s.events, "share.changed", ShareChangedPayload{Kind: "rewrite"})
	}
	return n, nil
}

// SweepOrphans walks every share and removes those whose target file no
// longer exists in the data directory. Catches OS-level deletes that
// bypassed the cascade hooks (e.g. files removed via SFTP while the server
// was off). Returns the number of orphans removed.
func (s *ShareService) SweepOrphans() (int, error) {
	links, err := s.repo.List()
	if err != nil {
		return 0, fmt.Errorf("list shares for sweep: %w", err)
	}
	removed := 0
	for _, link := range links {
		_, err := s.files.GetFileInfo(link.FilePath)
		if err == nil {
			continue
		}
		// Only delete on confirmed non-existence. Transient errors (EACCES
		// from a backup tool, EBUSY on Windows during AV scan, EIO on a
		// flaky disk, an unmounted backing store at boot) must not nuke
		// share rows — share tokens are credentials with no recovery.
		if !errors.Is(err, os.ErrNotExist) {
			slog.Warn("share sweep: stat error, skipping (not deleted)", "id", link.ID, "path", link.FilePath, "error", err)
			continue
		}
		if delErr := s.repo.Delete(link.ID); delErr != nil {
			slog.Warn("share sweep: failed to delete orphan", "id", link.ID, "path", link.FilePath, "error", delErr)
			continue
		}
		removed++
	}
	if removed > 0 {
		recordIf(s.events, "share.changed", ShareChangedPayload{Kind: "orphaned"})
	}
	return removed, nil
}
