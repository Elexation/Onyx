package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Shared login limit that browsers with a device cookie skip (OWASP "device cookies"), so guessers can't lock you out
const (
	unknownFreeFailures = 20
	deviceMaxFailures   = 10
	deviceLockout       = time.Hour
	guardIdleReset      = time.Hour
	maxDeviceEntries    = 10000
	deviceIDBytes       = 16
	maxDeviceTokenLen   = 128
	deviceMACLabel      = "onyx-device-v1:"
)

// DeviceCookieMaxAge is the longest lifetime browsers honour (Chrome caps cookies at 400 days)
const DeviceCookieMaxAge = 400 * 24 * time.Hour

// LoginThrottledError means the shared limit is in force. RetryAfter says when to try again.
type LoginThrottledError struct{ RetryAfter time.Duration }

func (e *LoginThrottledError) Error() string { return "too many login attempts" }

type deviceAttempts struct {
	fails int
	last  time.Time
}

type loginGuard struct {
	mu           sync.Mutex
	hash         string
	unknownFails int
	unknownLast  time.Time
	devices      map[string]*deviceAttempts
	now          func() time.Time
}

func newLoginGuard() *loginGuard {
	return &loginGuard{devices: make(map[string]*deviceAttempts), now: time.Now}
}

// loginAttempt records which counter an attempt was charged to
type loginAttempt struct{ deviceID string }

// begin charges one failure up front, like RateLimiter, so parallel guesses can't all slip past the gate
func (g *loginGuard) begin(passwordHash, deviceID string) (loginAttempt, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()

	// New password hash (change or reset-password)? Old counters and device cookies no longer apply.
	if g.hash != passwordHash {
		g.hash = passwordHash
		g.unknownFails = 0
		clear(g.devices)
	}

	if deviceID != "" {
		d := g.devices[deviceID]
		if d != nil && now.Sub(d.last) >= deviceLockout {
			delete(g.devices, deviceID)
			d = nil
		}
		// Locked-out device cookie counts as no cookie at all
		if d == nil || d.fails < deviceMaxFailures {
			if d == nil {
				if len(g.devices) >= maxDeviceEntries {
					g.evictOldestDeviceLocked()
				}
				d = &deviceAttempts{}
				g.devices[deviceID] = d
			}
			d.fails++
			d.last = now
			return loginAttempt{deviceID: deviceID}, nil
		}
	}

	if now.Sub(g.unknownLast) >= guardIdleReset {
		g.unknownFails = 0
	}
	if g.unknownFails >= unknownFreeFailures {
		if wait := unknownCooldown(g.unknownFails) - now.Sub(g.unknownLast); wait > 0 {
			return loginAttempt{}, &LoginThrottledError{RetryAfter: wait}
		}
	}
	g.unknownFails++
	g.unknownLast = now
	return loginAttempt{}, nil
}

// succeed clears the shared count and this device's count, since the password was right
func (g *loginGuard) succeed(a loginAttempt) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.unknownFails = 0
	if a.deviceID != "" {
		delete(g.devices, a.deviceID)
	}
}

func unknownCooldown(fails int) time.Duration {
	switch {
	case fails >= 3*unknownFreeFailures:
		return 60 * time.Second
	case fails >= 2*unknownFreeFailures:
		return 30 * time.Second
	default:
		return 10 * time.Second
	}
}

func (g *loginGuard) evictOldestDeviceLocked() {
	var oldestID string
	var oldest time.Time
	for id, d := range g.devices {
		if oldestID == "" || d.last.Before(oldest) {
			oldestID, oldest = id, d.last
		}
	}
	delete(g.devices, oldestID)
}

// newDeviceToken mints "id.mac", signed with the password hash so a password change voids it
func newDeviceToken(passwordHash string) (string, error) {
	raw := make([]byte, deviceIDBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate device id: %w", err)
	}
	id := base64.RawURLEncoding.EncodeToString(raw)
	return id + "." + deviceMAC(passwordHash, id), nil
}

// deviceIDFromToken returns the token's id. Empty if forged, malformed or signed for an older password.
func deviceIDFromToken(passwordHash, token string) string {
	if token == "" || len(token) > maxDeviceTokenLen {
		return ""
	}
	id, mac, ok := strings.Cut(token, ".")
	if !ok || id == "" {
		return ""
	}
	if !hmac.Equal([]byte(mac), []byte(deviceMAC(passwordHash, id))) {
		return ""
	}
	return id
}

func deviceMAC(passwordHash, id string) string {
	m := hmac.New(sha256.New, []byte(passwordHash))
	m.Write([]byte(deviceMACLabel + id))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}
