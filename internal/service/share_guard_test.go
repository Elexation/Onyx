package service

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Elexation/onyx/internal/domain"
)

func TestShareGuardLimitsPerToken(t *testing.T) {
	g := newShareGuard()
	now := time.Now()
	g.now = func() time.Time { return now }

	for i := 0; i < unknownFreeFailures; i++ {
		if _, err := g.begin("onyx_real", ""); err != nil {
			t.Fatalf("attempt %d throttled: %v", i+1, err)
		}
	}
	var throttled *ShareThrottledError
	if _, err := g.begin("onyx_real", ""); !errors.As(err, &throttled) || throttled.RetryAfter <= 0 {
		t.Fatalf("attempt past the free budget: got %v, want ShareThrottledError", err)
	}
	if _, err := g.begin("onyx_other", ""); err != nil {
		t.Fatalf("a different share was throttled: %v", err)
	}

	// The cooldown passes, then one more guess is let through and charged
	now = now.Add(unknownCooldown(unknownFreeFailures))
	if _, err := g.begin("onyx_real", ""); err != nil {
		t.Fatalf("after the cooldown: %v", err)
	}
	if _, err := g.begin("onyx_real", ""); !errors.As(err, &throttled) {
		t.Fatalf("second guess right after the cooldown: got %v, want throttled", err)
	}

	// An hour of quiet clears the count
	now = now.Add(guardIdleReset)
	if _, err := g.begin("onyx_real", ""); err != nil {
		t.Fatalf("after an idle hour: %v", err)
	}
}

// A recipient who knows the password must not reset a guesser's count
func TestShareGuardSucceedRemovesOnlyOwnCharge(t *testing.T) {
	g := newShareGuard()
	for i := 0; i < unknownFreeFailures-1; i++ {
		g.begin("onyx_real", "")
	}
	a, _ := g.begin("onyx_real", "")
	g.succeed(a)
	if _, err := g.begin("onyx_real", ""); err != nil {
		t.Fatalf("succeed left the count too high: %v", err)
	}
	if _, err := g.begin("onyx_real", ""); err == nil {
		t.Fatal("succeed cleared more than its own charge")
	}
}

// A flood of fake tokens must not push a guessed-at share out of the map and reset its count
func TestShareGuardFloodKeepsGuessedShare(t *testing.T) {
	g := newShareGuard()
	now := time.Now()
	g.now = func() time.Time { return now }
	for i := 0; i < unknownFreeFailures; i++ {
		g.begin("onyx_real", "")
	}
	// Long enough that the cooldown is over, so the share is between cooldowns
	now = now.Add(2 * time.Minute)
	for i := 0; i < maxShareGuardEntries+10; i++ {
		g.begin(fmt.Sprintf("onyx_fake_%d", i), "")
	}
	g.begin("onyx_real", "")
	var throttled *ShareThrottledError
	if _, err := g.begin("onyx_real", ""); !errors.As(err, &throttled) {
		t.Fatalf("after the flood the share's count was reset: got %v, want throttled", err)
	}
}

// A recipient who got in before keeps getting in while a guesser holds the share-wide limit
func TestShareVerifyRecipientCookieSkipsSharedLimit(t *testing.T) {
	hash, err := hashPassword("right")
	if err != nil {
		t.Fatal(err)
	}
	s := NewShareService(nil, nil, nil)
	ok, cookie, err := s.VerifyPassword(t.Context(), "onyx_real", "", testLink(), &hash, "right")
	if !ok || err != nil || cookie == "" {
		t.Fatalf("first visit: ok=%v cookie=%q err=%v", ok, cookie, err)
	}

	for i := 0; i < unknownFreeFailures; i++ {
		s.guard.begin("onyx_real", "")
	}
	var throttled *ShareThrottledError
	if _, _, err := s.VerifyPassword(t.Context(), "onyx_real", "", testLink(), &hash, "right"); !errors.As(err, &throttled) {
		t.Fatalf("new browser while the share is throttled: got %v, want ShareThrottledError", err)
	}
	ok, again, err := s.VerifyPassword(t.Context(), "onyx_real", cookie, testLink(), &hash, "right")
	if !ok || err != nil || again != cookie {
		t.Fatalf("returning recipient: ok=%v cookie kept=%v err=%v", ok, again == cookie, err)
	}

	// A forged cookie is no cookie
	if _, _, err := s.VerifyPassword(t.Context(), "onyx_real", "forged.mac", testLink(), &hash, "right"); !errors.As(err, &throttled) {
		t.Fatalf("forged cookie: got %v, want ShareThrottledError", err)
	}
}

// Fake tokens are throttled exactly like real ones, so a 429 can't confirm a share exists
func TestShareVerifyThrottlesUnknownTokens(t *testing.T) {
	s := NewShareService(nil, nil, nil)
	for i := 0; i < unknownFreeFailures; i++ {
		ok, _, err := s.VerifyPassword(t.Context(), "onyx_fake", "", nil, nil, "guess")
		if ok || err != nil {
			t.Fatalf("attempt %d: ok=%v err=%v", i+1, ok, err)
		}
	}
	var throttled *ShareThrottledError
	if _, _, err := s.VerifyPassword(t.Context(), "onyx_fake", "", nil, nil, "guess"); !errors.As(err, &throttled) {
		t.Fatalf("unknown token past the budget: got %v, want ShareThrottledError", err)
	}
}

func testLink() *domain.ShareLink {
	return &domain.ShareLink{ID: 1, FilePath: "/shared", IsDir: true, HasPassword: true}
}
