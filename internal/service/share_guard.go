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

type shareGuard struct {
	mu      sync.Mutex
	entries map[[sha256.Size]byte]*deviceAttempts
	now     func() time.Time
}

func newShareGuard() *shareGuard {
	return &shareGuard{entries: make(map[[sha256.Size]byte]*deviceAttempts), now: time.Now}
}

// begin charges one failure up front, unknown tokens too, so the limit can't reveal which shares exist
func (g *shareGuard) begin(token string) error {
	key := sha256.Sum256([]byte(token))
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()

	e := g.entries[key]
	if e != nil && now.Sub(e.last) >= guardIdleReset {
		e.fails = 0
	}
	if e == nil {
		if len(g.entries) >= maxShareGuardEntries {
			g.evictLocked(now)
		}
		e = &deviceAttempts{}
		g.entries[key] = e
	}
	if e.fails >= unknownFreeFailures {
		if wait := unknownCooldown(e.fails) - now.Sub(e.last); wait > 0 {
			return &ShareThrottledError{RetryAfter: wait}
		}
	}
	e.fails++
	e.last = now
	return nil
}

// undo removes one charge after a right password. A full reset would let one recipient clear a guesser's count.
func (g *shareGuard) undo(token string) {
	key := sha256.Sum256([]byte(token))
	g.mu.Lock()
	defer g.mu.Unlock()
	e := g.entries[key]
	if e == nil {
		return
	}
	e.fails--
	if e.fails <= 0 {
		delete(g.entries, key)
	}
}

// evictLocked drops the oldest entry not in cooldown, or the oldest overall if all are. Caller holds g.mu.
func (g *shareGuard) evictLocked(now time.Time) {
	var victim [sha256.Size]byte
	var victimLast time.Time
	found, victimLocked := false, false
	for key, e := range g.entries {
		locked := e.fails >= unknownFreeFailures && now.Sub(e.last) < unknownCooldown(e.fails)
		if !found || (victimLocked && !locked) || (locked == victimLocked && e.last.Before(victimLast)) {
			victim, victimLast, victimLocked, found = key, e.last, locked, true
		}
	}
	if found {
		delete(g.entries, victim)
	}
}
