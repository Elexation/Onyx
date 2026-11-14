package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newLimitTestService(t *testing.T, slots int) *TranscodeService {
	t.Helper()
	return &TranscodeService{
		cacheRoot:  t.TempDir(),
		sessions:   make(map[string]*TranscodeSession),
		sema:       make(chan struct{}, slots),
		publicSema: make(chan struct{}, max(slots-1, 1)),
		clientRuns: make(map[string]int),
		stopCh:     make(chan struct{}),
	}
}

// Share visitors filling every public slot must still leave one ffmpeg slot for the admin
func TestPublicTranscodesLeaveAdminSlot(t *testing.T) {
	ts := newLimitTestService(t, 2)
	if err := ts.acquirePublic(context.Background(), "visitor-a"); err != nil {
		t.Fatal(err)
	}
	ts.sema <- struct{}{} // visitor-a's run holds a real slot too

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := ts.acquirePublic(ctx, "visitor-b"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second visitor got a public slot with the public pool full: %v", err)
	}
	if n := ts.clientRuns["visitor-b"]; n != 0 {
		t.Fatalf("a visitor that gave up still counts %d runs", n)
	}
	select {
	case ts.sema <- struct{}{}:
	default:
		t.Fatal("no slot left for the admin while share visitors are transcoding")
	}
}

func TestPublicTranscodesCappedPerClient(t *testing.T) {
	ts := newLimitTestService(t, 8)
	for i := 0; i < maxClientTranscodes; i++ {
		if err := ts.acquirePublic(context.Background(), "visitor-a"); err != nil {
			t.Fatalf("run %d: %v", i+1, err)
		}
	}
	if err := ts.acquirePublic(context.Background(), "visitor-a"); !errors.Is(err, ErrTranscodeBusy) {
		t.Fatalf("run past the per-client limit: got %v, want ErrTranscodeBusy", err)
	}
	if err := ts.acquirePublic(context.Background(), "visitor-b"); err != nil {
		t.Fatalf("another client was refused: %v", err)
	}
	ts.releasePublic("visitor-a")
	if err := ts.acquirePublic(context.Background(), "visitor-a"); err != nil {
		t.Fatalf("after one run ended: %v", err)
	}
}

// Only segment requests keep a session alive, and the admin using one takes it off the public list
func TestGetSegmentRefreshesAndClaims(t *testing.T) {
	ts := newLimitTestService(t, 2)
	old := time.Now().Add(-time.Hour)
	s := &TranscodeSession{hash: "h", lastUsed: old, public: true}
	ts.sessions["h"] = s

	ts.GetSegment(context.Background(), "h", 99, 0, TranscodeClient{Public: true, Key: "visitor"})
	if !s.lastUsed.After(old) || !s.public {
		t.Fatalf("visitor segment request: lastUsed refreshed=%v public=%v, want true, true", s.lastUsed.After(old), s.public)
	}
	ts.GetSegment(context.Background(), "h", 99, 0, TranscodeClient{})
	if s.public {
		t.Fatal("admin segment request left the session marked public")
	}
}

func makeCacheDir(t *testing.T, root, hash string, size int, age time.Duration) string {
	t.Helper()
	dir := filepath.Join(root, hash)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "data000000.m4s"), make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
	master := filepath.Join(dir, "master.m3u8")
	if err := os.WriteFile(master, []byte("#EXTM3U\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	at := time.Now().Add(-age)
	if err := os.Chtimes(master, at, at); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestDropIdleSessions(t *testing.T) {
	ts := newLimitTestService(t, 2)
	idleDir := makeCacheDir(t, ts.cacheRoot, "idle", 1, 0)
	activeDir := makeCacheDir(t, ts.cacheRoot, "active", 1, 0)
	ts.sessions["idle"] = &TranscodeSession{hash: "idle", dir: idleDir, lastUsed: time.Now().Add(-idleDropAfter - time.Minute)}
	ts.sessions["active"] = &TranscodeSession{hash: "active", dir: activeDir, lastUsed: time.Now()}

	ts.dropIdleSessions()
	if _, ok := ts.sessions["idle"]; ok {
		t.Fatal("idle session still registered")
	}
	if _, err := os.Stat(idleDir); !os.IsNotExist(err) {
		t.Fatalf("idle session's cache dir left behind: %v", err)
	}
	if _, ok := ts.sessions["active"]; !ok {
		t.Fatal("active session dropped")
	}
}

// Over the limit, unowned dirs go first, then visitor sessions, then admin idle ones; admin active ones stay
func TestEnforceCacheLimitOrder(t *testing.T) {
	ts := newLimitTestService(t, 2)
	const mb = 1 << 20
	now := time.Now()
	makeCacheDir(t, ts.cacheRoot, "orphan-old", 3*mb, time.Hour)
	makeCacheDir(t, ts.cacheRoot, "orphan-young", 2*mb, 0)
	for hash, s := range map[string]*TranscodeSession{
		"public":       {lastUsed: now, public: true},
		"admin-active": {lastUsed: now},
		"admin-idle":   {lastUsed: now.Add(-idleStopAfter - time.Minute)},
	} {
		s.hash, s.dir = hash, makeCacheDir(t, ts.cacheRoot, hash, 3*mb, 0)
		ts.sessions[hash] = s
	}
	ts.SetCacheLimit(7 * mb)

	ts.enforceCacheLimit()
	for hash, wantKept := range map[string]bool{
		"orphan-old":   false,
		"public":       false,
		"admin-idle":   false,
		"orphan-young": true,
		"admin-active": true,
	} {
		_, err := os.Stat(filepath.Join(ts.cacheRoot, hash))
		if kept := err == nil; kept != wantKept {
			t.Errorf("%s kept=%v, want %v", hash, kept, wantKept)
		}
	}
	if _, total := ts.cacheDirSizes(); total > 7*mb {
		t.Fatalf("cache still %d bytes over a %d limit", total, 7*mb)
	}
}

func TestEnforceCacheLimitOff(t *testing.T) {
	ts := newLimitTestService(t, 2)
	makeCacheDir(t, ts.cacheRoot, "orphan-old", 1<<20, time.Hour)
	ts.SetCacheLimit(0)
	ts.enforceCacheLimit()
	if _, err := os.Stat(filepath.Join(ts.cacheRoot, "orphan-old")); err != nil {
		t.Fatalf("limit 0 still deleted cache: %v", err)
	}
}
