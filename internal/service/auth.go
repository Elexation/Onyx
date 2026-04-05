package service

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"

	"github.com/Elexation/onyx/internal/domain"
)

const (
	argonMemory  = 65536
	argonTime    = 3
	argonThreads = 1
	argonSaltLen = 16
	argonKeyLen  = 32
)

type UserRepo interface {
	Create(username, passwordHash string) (*domain.User, error)
	GetByUsername(username string) (*domain.User, error)
	Exists() (bool, error)
}

type SessionRepo interface {
	Create(s *domain.Session) error
	GetByID(id string) (*domain.Session, error)
	UpdateLastActive(id string) error
	Delete(id string) error
	DeleteExpired() (int64, error)
}

type AuthService struct {
	users    UserRepo
	sessions SessionRepo
	settings *SettingsService
}

func NewAuthService(users UserRepo, sessions SessionRepo, settings *SettingsService) *AuthService {
	return &AuthService{users: users, sessions: sessions, settings: settings}
}

func (a *AuthService) IsFirstRun() (bool, error) {
	exists, err := a.users.Exists()
	if err != nil {
		return false, err
	}
	return !exists, nil
}

func (a *AuthService) Setup(password string) (*domain.Session, error) {
	exists, err := a.users.Exists()
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("admin already exists")
	}

	hash, err := hashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user, err := a.users.Create("admin", hash)
	if err != nil {
		return nil, fmt.Errorf("create admin: %w", err)
	}

	return a.createSession(user.ID)
}

func (a *AuthService) Login(password string) (*domain.Session, error) {
	user, err := a.users.GetByUsername("admin")
	if err != nil {
		return nil, fmt.Errorf("get admin: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	if !verifyPassword(password, user.PasswordHash) {
		return nil, fmt.Errorf("invalid credentials")
	}

	return a.createSession(user.ID)
}

func (a *AuthService) Logout(sessionID string) error {
	return a.sessions.Delete(sessionID)
}

func (a *AuthService) ValidateSession(sessionID string) (*domain.Session, error) {
	session, err := a.sessions.GetByID(sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, nil
	}

	now := time.Now().Unix()
	if session.ExpiresAt < now {
		a.sessions.Delete(sessionID)
		return nil, nil
	}

	// Throttle last_active_at updates to once per minute
	if now-session.LastActiveAt > 60 {
		if err := a.sessions.UpdateLastActive(sessionID); err != nil {
			slog.Warn("failed to update session activity", "error", err)
		}
	}

	return session, nil
}

func (a *AuthService) StartCleanup(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			count, err := a.sessions.DeleteExpired()
			if err != nil {
				slog.Warn("session cleanup failed", "error", err)
				continue
			}
			if count > 0 {
				slog.Info("cleaned up expired sessions", "count", count)
			}
		}
	}()
}

func (a *AuthService) createSession(userID int64) (*domain.Session, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("generate session token: %w", err)
	}

	csrfBytes := make([]byte, 32)
	if _, err := rand.Read(csrfBytes); err != nil {
		return nil, fmt.Errorf("generate csrf token: %w", err)
	}

	lifetime, err := a.settings.Get(domain.SettingSessionLifetime)
	if err != nil {
		return nil, fmt.Errorf("get session lifetime: %w", err)
	}
	dur := domain.GetDuration(lifetime)

	now := time.Now().Unix()
	session := &domain.Session{
		ID:           hex.EncodeToString(tokenBytes),
		UserID:       userID,
		CSRFToken:    hex.EncodeToString(csrfBytes),
		CreatedAt:    now,
		LastActiveAt: now,
		ExpiresAt:    now + int64(dur.Seconds()),
	}

	if err := a.sessions.Create(session); err != nil {
		return nil, err
	}
	return session, nil
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

func verifyPassword(password, hash string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}

	var memory uint32
	var iterations uint32
	var threads uint8
	_, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &threads)
	if err != nil {
		return false
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}

	expectedKey, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}

	key := argon2.IDKey([]byte(password), salt, iterations, memory, threads, uint32(len(expectedKey)))
	return subtle.ConstantTimeCompare(key, expectedKey) == 1
}
