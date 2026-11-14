package service

import (
	"errors"
	"testing"
	"time"
)

func TestShareGuardLimitsPerToken(t *testing.T) {
	g := newShareGuard()
	now := time.Now()
	g.now = func() time.Time { return now }

	for i := 0; i < unknownFreeFailures; i++ {
		if err := g.begin("onyx_real"); err != nil {
			t.Fatalf("attempt %d throttled: %v", i+1, err)
		}
	}
	var throttled *ShareThrottledError
	if err := g.begin("onyx_real"); !errors.As(err, &throttled) || throttled.RetryAfter <= 0 {
		t.Fatalf("attempt past the free budget: got %v, want ShareThrottledError", err)
	}
	if err := g.begin("onyx_other"); err != nil {
		t.Fatalf("a different share was throttled: %v", err)
	}

	// The cooldown passes, then one more guess is let through and charged
	now = now.Add(unknownCooldown(unknownFreeFailures))
	if err := g.begin("onyx_real"); err != nil {
		t.Fatalf("after the cooldown: %v", err)
	}
	if err := g.begin("onyx_real"); !errors.As(err, &throttled) {
		t.Fatalf("second guess right after the cooldown: got %v, want throttled", err)
	}

	// An hour of quiet clears the count
	now = now.Add(guardIdleReset)
	if err := g.begin("onyx_real"); err != nil {
		t.Fatalf("after an idle hour: %v", err)
	}
}

// A recipient who knows the password must not reset a guesser's count
func TestShareGuardUndoRemovesOnlyOwnCharge(t *testing.T) {
	g := newShareGuard()
	for i := 0; i < unknownFreeFailures-1; i++ {
		g.begin("onyx_real")
	}
	g.begin("onyx_real")
	g.undo("onyx_real")
	if err := g.begin("onyx_real"); err != nil {
		t.Fatalf("undo left the count too high: %v", err)
	}
	if err := g.begin("onyx_real"); err == nil {
		t.Fatal("undo cleared more than its own charge")
	}
}

// Fake tokens are throttled exactly like real ones, so a 429 can't confirm a share exists
func TestShareVerifyThrottlesUnknownTokens(t *testing.T) {
	s := NewShareService(nil, nil, nil)
	for i := 0; i < unknownFreeFailures; i++ {
		ok, err := s.VerifyPassword(t.Context(), "onyx_fake", nil, nil, "guess")
		if ok || err != nil {
			t.Fatalf("attempt %d: ok=%v err=%v", i+1, ok, err)
		}
	}
	var throttled *ShareThrottledError
	if _, err := s.VerifyPassword(t.Context(), "onyx_fake", nil, nil, "guess"); !errors.As(err, &throttled) {
		t.Fatalf("unknown token past the budget: got %v, want ShareThrottledError", err)
	}
}
