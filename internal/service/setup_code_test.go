package service

import (
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Elexation/onyx/internal/adapter/database"
)

func newFirstRunAuth(t *testing.T) *AuthService {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "onyx.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return NewAuthService(database.NewUserRepo(db), database.NewSessionRepo(db), NewSettingsService(database.NewSettingsRepo(db)))
}

// Whoever reaches a fresh install first must not become admin without the code from the server log
func TestSetupRequiresCode(t *testing.T) {
	auth := newFirstRunAuth(t)

	if _, err := auth.Setup("password123", "anything"); !errors.Is(err, ErrInvalidSetupCode) {
		t.Fatalf("setup before a code was armed: %v, want ErrInvalidSetupCode", err)
	}

	code, err := auth.InitSetupCode("")
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[A-Z2-7]{4}(-[A-Z2-7]{1,4}){6}$`).MatchString(code) {
		t.Fatalf("code %q is not grouped base32 of 16 random bytes", code)
	}

	for _, bad := range []string{"", "wrong", code[:len(code)-1]} {
		if _, err := auth.Setup("password123", bad); !errors.Is(err, ErrInvalidSetupCode) {
			t.Fatalf("setup with %q: %v, want ErrInvalidSetupCode", bad, err)
		}
	}
	if first, _ := auth.IsFirstRun(); !first {
		t.Fatal("a rejected setup created the admin")
	}

	retyped := " " + strings.ToLower(strings.ReplaceAll(code, "-", " ")) + " "
	if _, err := auth.Setup("password123", retyped); err != nil {
		t.Fatalf("setup with retyped code: %v", err)
	}
	if _, err := auth.Setup("password456", code); err == nil || err.Error() != "admin already exists" {
		t.Fatalf("second setup: %v, want admin already exists", err)
	}
	if again, _ := auth.InitSetupCode(""); again != "" {
		t.Fatalf("code armed after admin exists: %q", again)
	}
	auth.setupMu.Lock()
	left := auth.setupCode
	auth.setupMu.Unlock()
	if left != "" {
		t.Fatal("setup code still held after setup succeeded")
	}
}

func TestSetupCodePreset(t *testing.T) {
	auth := newFirstRunAuth(t)
	if _, err := auth.InitSetupCode("too-short-1"); err == nil {
		t.Fatal("short ONYX_SETUP_TOKEN accepted")
	}
	preset := "headless-install-token-2026"
	got, err := auth.InitSetupCode(preset)
	if err != nil || got != preset {
		t.Fatalf("InitSetupCode(preset) = %q, %v", got, err)
	}
	if _, err := auth.Setup("password123", "headless-install-token-2025"); !errors.Is(err, ErrInvalidSetupCode) {
		t.Fatalf("wrong preset accepted: %v", err)
	}
	if _, err := auth.Setup("password123", preset); err != nil {
		t.Fatalf("preset rejected: %v", err)
	}
}
