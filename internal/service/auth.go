package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
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

// Cap concurrent argon2 runs, since each one allocates argonMemory KiB
var (
	authArgon = make(chan struct{}, 2)
	// Own slot for public share checks, so a flood cannot stall admin login
	shareArgon = make(chan struct{}, 1)
)

var ErrInvalidCredentials = errors.New("invalid credentials")

// dummyHash absorbs argon2 work on Login when no admin account exists, so the
// request's latency does not leak first-run state. Generated once from random
// bytes at package init; the matching password is discarded.
var dummyHash string

func init() {
	p := make([]byte, 32)
	if _, err := rand.Read(p); err != nil {
		panic("auth: init dummy hash: " + err.Error())
	}
	h, err := hashPassword(string(p))
	if err != nil {
		panic("auth: init dummy hash: " + err.Error())
	}
	dummyHash = h
}

type UserRepo interface {
	Create(username, passwordHash string) (*domain.User, error)
	GetByUsername(username string) (*domain.User, error)
	UpdatePassword(username, passwordHash string) error
	Exists() (bool, error)
}

type SessionRepo interface {
	CreateForHash(s *domain.Session, passwordHash string) (bool, error)
	GetByID(id string) (*domain.Session, error)
	Delete(id string) error
	DeleteOtherSessions(userID int64, keepSessionID string) (int64, error)
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
		// UNIQUE-constraint failure means a concurrent Setup won the race
		// between the Exists() check above and Create(). Map to the same
		// error as the fast path.
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, fmt.Errorf("admin already exists")
		}
		return nil, fmt.Errorf("create admin: %w", err)
	}

	return a.createSession(user.ID, hash)
}

func (a *AuthService) Login(ctx context.Context, password string) (*domain.Session, error) {
	user, err := a.users.GetByUsername("admin")
	if err != nil {
		return nil, fmt.Errorf("get admin: %w", err)
	}
	if user == nil {
		// Absorb argon2 work so Login latency doesn't reveal whether an admin
		// has been configured. Result is discarded.
		verifyPassword(ctx, authArgon, password, dummyHash)
		return nil, ErrInvalidCredentials
	}

	ok, err := verifyPassword(ctx, authArgon, password, user.PasswordHash)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrInvalidCredentials
	}

	return a.createSession(user.ID, user.PasswordHash)
}

func (a *AuthService) Logout(sessionID string) error {
	return a.sessions.Delete(hashSessionID(sessionID))
}

func (a *AuthService) ValidateSession(sessionID string) (*domain.Session, error) {
	hashed := hashSessionID(sessionID)
	session, err := a.sessions.GetByID(hashed)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, nil
	}

	if session.ExpiresAt < time.Now().Unix() {
		a.sessions.Delete(hashed)
		return nil, nil
	}

	session.ID = sessionID // hand back the raw id; the hash never leaves the repo boundary
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

// ChangePassword rotates sessions before storing the hash, so a failure never leaves old sessions valid.
func (a *AuthService) ChangePassword(ctx context.Context, currentPassword, newPassword string) (*domain.Session, error) {
	user, err := a.users.GetByUsername("admin")
	if err != nil {
		return nil, fmt.Errorf("get admin: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	ok, err := verifyPassword(ctx, authArgon, currentPassword, user.PasswordHash)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("invalid current password")
	}

	hash, err := hashPassword(newPassword)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	newSession, err := a.createSession(user.ID, user.PasswordHash)
	if err != nil {
		return nil, fmt.Errorf("rotate session: %w", err)
	}

	// Deletes the caller's previous session along with all others (anything
	// not equal to newSession.ID).
	keep := hashSessionID(newSession.ID)
	if _, err := a.sessions.DeleteOtherSessions(user.ID, keep); err != nil {
		return nil, fmt.Errorf("invalidate other sessions: %w", err)
	}

	if err := a.users.UpdatePassword("admin", hash); err != nil {
		return nil, fmt.Errorf("update password: %w", err)
	}

	// Sweep again: a login that verified the old hash may have landed before the update
	if _, err := a.sessions.DeleteOtherSessions(user.ID, keep); err != nil {
		return nil, fmt.Errorf("invalidate other sessions: %w", err)
	}

	return newSession, nil
}

// ResetPassword sets a random admin password and revokes every session. Returns the new password.
func (a *AuthService) ResetPassword() (string, error) {
	user, err := a.users.GetByUsername("admin")
	if err != nil {
		return "", fmt.Errorf("get admin: %w", err)
	}
	if user == nil {
		return "", fmt.Errorf("no admin account yet; open the web UI to run setup")
	}
	raw := make([]byte, 15)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate password: %w", err)
	}
	password := base64.RawURLEncoding.EncodeToString(raw)
	hash, err := hashPassword(password)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	// Sessions first, like ChangePassword; "" matches no stored id, so all go
	if _, err := a.sessions.DeleteOtherSessions(user.ID, ""); err != nil {
		return "", fmt.Errorf("revoke sessions: %w", err)
	}
	if err := a.users.UpdatePassword("admin", hash); err != nil {
		return "", fmt.Errorf("update password: %w", err)
	}
	// Sweep again: a login that verified the old hash may have landed before the update
	if _, err := a.sessions.DeleteOtherSessions(user.ID, ""); err != nil {
		return "", fmt.Errorf("revoke sessions: %w", err)
	}
	return password, nil
}

// createSession stores a session bound to passwordHash; a hash change since verification refuses it
func (a *AuthService) createSession(userID int64, passwordHash string) (*domain.Session, error) {
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
	if dur <= 0 {
		dur = 720 * time.Hour
	}

	now := time.Now().Unix()
	session := &domain.Session{
		ID:           hex.EncodeToString(tokenBytes),
		UserID:       userID,
		CSRFToken:    hex.EncodeToString(csrfBytes),
		CreatedAt:    now,
		LastActiveAt: now,
		ExpiresAt:    now + int64(dur.Seconds()),
	}

	// The DB row keys on sha256(id); the raw id exists only in the cookie,
	// mirroring PAT storage. CSRF token stays plaintext (inert without the id).
	stored := *session
	stored.ID = hashSessionID(session.ID)
	created, err := a.sessions.CreateForHash(&stored, passwordHash)
	if err != nil {
		return nil, err
	}
	if !created {
		return nil, ErrInvalidCredentials
	}
	return session, nil
}

// hashSessionID returns the hex sha256 of a raw session id. Session ids are
// bearer credentials; only the hash is stored at rest (mirrors PATs).
func hashSessionID(id string) string {
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:])
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	authArgon <- struct{}{}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	<-authArgon

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// verifyPassword waits for a slot in sem. Error only if ctx ends first.
func verifyPassword(ctx context.Context, sem chan struct{}, password, hash string) (bool, error) {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, nil
	}

	var memory uint32
	var iterations uint32
	var threads uint8
	_, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &threads)
	if err != nil {
		return false, nil
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, nil
	}

	expectedKey, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, nil
	}

	select {
	case sem <- struct{}{}:
	case <-ctx.Done():
		return false, ctx.Err()
	}
	key := argon2.IDKey([]byte(password), salt, iterations, memory, threads, uint32(len(expectedKey)))
	<-sem
	return subtle.ConstantTimeCompare(key, expectedKey) == 1, nil
}
