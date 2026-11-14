package service

import (
	"crypto/sha256"
	"sync"
	"time"
)

// Per-share limit on wrong passwords, so guessing one share stays slow however many IPs try
const maxShareGuardEntries = 10000

// ShareThrottledError means a share's wrong-password limit is in force. RetryAfter says when to try again.
type ShareThrottledError struct{ RetryAfter time.Duration }

func (e *ShareThrottledError) Error() string { return "too many password attempts for this share" }

// shareGuard counts wrong passwords per share, with an own budget per recipient cookie so guessers can't lock them out
type shareGuard struct {
	mu      sync.Mutex
	entries map[[sha256.Size]byte]*deviceAttempts
	devices map[string]*deviceAttempts // share hash + recipient cookie id
	now     func() time.Time
}

func newShareGuard() *shareGuard {
	return &shareGuard{
		entries: make(map[[sha256.Size]byte]*deviceAttempts),
		devices: make(map[string]*deviceAttempts),
		now:     time.Now,
	}
}

// shareAttempt records which count an attempt was charged to
type shareAttempt struct {
	share  [sha256.Size]byte
	device string
}

// begin charges one failure up front, unknown tokens too, so the limit can't reveal which shares exist
func (g *shareGuard) begin(token, deviceID string) (shareAttempt, error) {
	share := sha256.Sum256([]byte(token))
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()

	if deviceID != "" {
		key := string(share[:]) + deviceID
		d := g.devices[key]
		if d != nil && now.Sub(d.last) >= deviceLockout {
			delete(g.devices, key)
			d = nil
		}
		// Locked-out recipient cookie counts as no cookie at all
		if d == nil || d.fails < deviceMaxFailures {
			if d == nil {
				if len(g.devices) >= maxShareGuardEntries {
					evictLightest(g.devices, func(*deviceAttempts) bool { return false })
				}
				d = &deviceAttempts{}
				g.devices[key] = d
			}
			d.fails++
			d.last = now
			return shareAttempt{share: share, device: key}, nil
		}
	}

	e := g.entries[share]
	if e != nil && now.Sub(e.last) >= guardIdleReset {
		e.fails = 0
	}
	if e == nil {
		if len(g.entries) >= maxShareGuardEntries {
			// Keep guessed-at shares: a flood of fake tokens must not reset a real share's count
			evictLightest(g.entries, func(e *deviceAttempts) bool {
				return e.fails >= unknownFreeFailures && now.Sub(e.last) < guardIdleReset
			})
		}
		e = &deviceAttempts{}
		g.entries[share] = e
	}
	if e.fails >= unknownFreeFailures {
		if wait := unknownCooldown(e.fails) - now.Sub(e.last); wait > 0 {
			return shareAttempt{}, &ShareThrottledError{RetryAfter: wait}
		}
	}
	e.fails++
	e.last = now
	return shareAttempt{share: share}, nil
}

// succeed clears a recipient's own count but takes just this charge off the share-wide one, which recipients must not reset
func (g *shareGuard) succeed(a shareAttempt) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if a.device != "" {
		delete(g.devices, a.device)
		return
	}
	e := g.entries[a.share]
	if e == nil {
		return
	}
	e.fails--
	if e.fails <= 0 {
		delete(g.entries, a.share)
	}
}

// evictLightest drops the oldest entry heavy says is safe to forget, or the oldest overall if none is
func evictLightest[K comparable](m map[K]*deviceAttempts, heavy func(*deviceAttempts) bool) {
	var victim K
	var victimLast time.Time
	found, victimHeavy := false, false
	for key, e := range m {
		h := heavy(e)
		if !found || (victimHeavy && !h) || (h == victimHeavy && e.last.Before(victimLast)) {
			victim, victimLast, victimHeavy, found = key, e.last, h, true
		}
	}
	if found {
		delete(m, victim)
	}
}
