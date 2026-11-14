package service

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Elexation/onyx/internal/adapter/database"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestGuard() (*loginGuard, *fakeClock) {
	c := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	g := newLoginGuard()
	g.now = c.now
	return g, c
}

func throttledFor(t *testing.T, err error) time.Duration {
	t.Helper()
	var th *LoginThrottledError
	if !errors.As(err, &th) {
		t.Fatalf("err = %v, want *LoginThrottledError", err)
	}
	return th.RetryAfter
}

func TestLoginGuardSharedLimitEscalates(t *testing.T) {
	g, c := newTestGuard()
	for i := 0; i < unknownFreeFailures; i++ {
		if _, err := g.begin("h", ""); err != nil {
			t.Fatalf("attempt %d throttled early: %v", i+1, err)
		}
	}
	if wait := throttledFor(t, func() error { _, err := g.begin("h", ""); return err }()); wait != 10*time.Second {
		t.Fatalf("wait = %v, want 10s", wait)
	}
	for g.unknownFails < 2*unknownFreeFailures {
		c.advance(10 * time.Second)
		if _, err := g.begin("h", ""); err != nil {
			t.Fatalf("attempt after wait refused: %v", err)
		}
	}
	if wait := throttledFor(t, func() error { _, err := g.begin("h", ""); return err }()); wait != 30*time.Second {
		t.Fatalf("wait = %v, want 30s", wait)
	}
	for g.unknownFails < 3*unknownFreeFailures {
		c.advance(30 * time.Second)
		if _, err := g.begin("h", ""); err != nil {
			t.Fatalf("attempt after wait refused: %v", err)
		}
	}
	if wait := throttledFor(t, func() error { _, err := g.begin("h", ""); return err }()); wait != 60*time.Second {
		t.Fatalf("wait = %v, want 60s", wait)
	}
}

func TestLoginGuardKnownDeviceSkipsSharedLimit(t *testing.T) {
	g, _ := newTestGuard()
	for i := 0; i < unknownFreeFailures; i++ {
		g.begin("h", "")
	}
	if _, err := g.begin("h", ""); err == nil {
		t.Fatal("unknown browser not throttled")
	}
	if _, err := g.begin("h", "dev1"); err != nil {
		t.Fatalf("known device throttled by shared limit: %v", err)
	}
}

func TestLoginGuardDeviceLockoutFallsBackToShared(t *testing.T) {
	g, c := newTestGuard()
	for i := 0; i < deviceMaxFailures; i++ {
		if a, err := g.begin("h", "dev1"); err != nil || a.deviceID != "dev1" {
			t.Fatalf("device attempt %d: %+v, %v", i+1, a, err)
		}
	}
	a, err := g.begin("h", "dev1")
	if err != nil || a.deviceID != "" {
		t.Fatalf("locked device should be charged as unknown, got %+v, %v", a, err)
	}
	c.advance(deviceLockout)
	if a, _ := g.begin("h", "dev1"); a.deviceID != "dev1" {
		t.Fatal("device lockout did not expire")
	}
}

func TestLoginGuardResets(t *testing.T) {
	g, c := newTestGuard()
	fill := func() {
		for i := 0; i < unknownFreeFailures; i++ {
			g.begin("h", "")
		}
	}

	fill()
	a, _ := g.begin("h", "dev1")
	g.succeed(a)
	if _, err := g.begin("h", ""); err != nil {
		t.Fatalf("success did not clear shared count: %v", err)
	}

	g.unknownFails = 0
	fill()
	if _, err := g.begin("h2", ""); err != nil {
		t.Fatalf("password change did not clear shared count: %v", err)
	}

	g.unknownFails = 0
	fill()
	c.advance(guardIdleReset)
	if _, err := g.begin("h2", ""); err != nil {
		t.Fatalf("idle period did not clear shared count: %v", err)
	}
}

func TestDeviceToken(t *testing.T) {
	tok, err := newDeviceToken("hash-a")
	if err != nil {
		t.Fatal(err)
	}
	id := deviceIDFromToken("hash-a", tok)
	if id == "" || !strings.HasPrefix(tok, id+".") {
		t.Fatalf("valid token rejected: %q -> %q", tok, id)
	}
	other, _ := newDeviceToken("hash-a")
	forged := id + "." + strings.SplitN(other, ".", 2)[1]
	for name, bad := range map[string]string{
		"other password": tok,
		"swapped mac":    forged,
		"no mac":         id,
		"empty id":       "." + strings.SplitN(tok, ".", 2)[1],
		"oversized":      strings.Repeat("a", maxDeviceTokenLen+1),
	} {
		hash := "hash-a"
		if name == "other password" {
			hash = "hash-b"
		}
		if got := deviceIDFromToken(hash, bad); got != "" {
			t.Errorf("%s: accepted as %q", name, got)
		}
	}
}

// End to end: guessing locks out unknown browsers only, and reset-password lifts it
func TestAuthLoginSharedLimit(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "onyx.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	auth := NewAuthService(database.NewUserRepo(db), database.NewSessionRepo(db), NewSettingsService(database.NewSettingsRepo(db)))
	code, err := auth.InitSetupCode("")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := auth.Setup("right password", code); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	_, device, err := auth.Login(ctx, "right password", "")
	if err != nil || device == "" {
		t.Fatalf("first login: device=%q err=%v", device, err)
	}

	for i := 0; i < unknownFreeFailures; i++ {
		if _, _, err := auth.Login(ctx, "wrong", ""); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("guess %d: %v", i+1, err)
		}
	}
	if _, _, err := auth.Login(ctx, "right password", ""); err == nil {
		t.Fatal("unknown browser logged in past the shared limit")
	} else {
		throttledFor(t, err)
	}

	s, again, err := auth.Login(ctx, "right password", device)
	if err != nil || s == nil || again != device {
		t.Fatalf("known browser refused: device=%q err=%v", again, err)
	}

	for i := 0; i < unknownFreeFailures; i++ {
		auth.Login(ctx, "wrong", "")
	}
	if _, err := auth.ResetPassword(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := auth.Login(ctx, "wrong", ""); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("reset-password did not lift the shared limit: %v", err)
	}
	if _, _, err := auth.Login(ctx, "wrong", device); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("old device cookie after reset: %v", err)
	}
	if id := deviceIDFromToken(auth.guard.hash, device); id != "" {
		t.Fatal("reset-password left the old device cookie valid")
	}
}
