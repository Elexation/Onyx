package service

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/Elexation/onyx/internal/adapter/media"
)

func TestWaitFFmpeg_FastFailFallsBackToSoftware(t *testing.T) {
	ts := &TranscodeService{
		encoder:     media.EncoderNVENC,
		hwaccelPref: "auto",
		sema:        make(chan struct{}, 2),
		stopCh:      make(chan struct{}),
		sessions:    make(map[string]*TranscodeSession),
		ffmpeg:      media.Detect(),
	}

	done := make(chan struct{})
	s := &TranscodeSession{
		hash:      "test-fast-fail",
		runDone:   done,
		startedAt: time.Now(),
		cancel:    func() {},
	}

	// Fill one semaphore slot (waitFFmpeg drains it).
	ts.sema <- struct{}{}

	cmd := exec.Command("cmd", "/C", "exit 1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start failing command: %v", err)
	}

	stderrBuf := &boundedBuffer{max: 1024}
	ts.waitFFmpeg(context.Background(), cmd, s, done, 0, stderrBuf, media.EncoderNVENC)

	if enc := ts.Encoder(); enc != media.EncoderSoftware {
		t.Errorf("expected encoder to be %q after fast fail, got %q", media.EncoderSoftware, enc)
	}

	select {
	case <-done:
	default:
		t.Error("done channel was not closed")
	}

	if !ts.reprobing.Load() {
		t.Error("expected reprobing to be true after fast fail")
	}

	// Shut down the re-probe goroutine.
	close(ts.stopCh)
	ts.wg.Wait()
}

func TestWaitFFmpeg_SlowFailAlsoFallsBack(t *testing.T) {
	ts := &TranscodeService{
		encoder:     media.EncoderNVENC,
		hwaccelPref: "auto",
		sema:        make(chan struct{}, 2),
		stopCh:      make(chan struct{}),
		sessions:    make(map[string]*TranscodeSession),
		ffmpeg:      media.Detect(),
	}

	done := make(chan struct{})
	s := &TranscodeSession{
		hash:      "test-slow-fail",
		runDone:   done,
		startedAt: time.Now().Add(-5 * time.Second),
		cancel:    func() {},
	}

	ts.sema <- struct{}{}

	cmd := exec.Command("cmd", "/C", "exit 1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start failing command: %v", err)
	}

	stderrBuf := &boundedBuffer{max: 1024}
	ts.waitFFmpeg(context.Background(), cmd, s, done, 0, stderrBuf, media.EncoderNVENC)

	if enc := ts.Encoder(); enc != media.EncoderSoftware {
		t.Errorf("expected encoder to be %q after slow fail, got %q", media.EncoderSoftware, enc)
	}

	select {
	case <-done:
	default:
		t.Error("done channel was not closed")
	}

	if !ts.reprobing.Load() {
		t.Error("expected reprobing to be true after slow fail")
	}

	close(ts.stopCh)
	ts.wg.Wait()
}

func TestWaitFFmpeg_SoftwareEncoderDoesNotFallBack(t *testing.T) {
	ts := &TranscodeService{
		encoder:     media.EncoderSoftware,
		hwaccelPref: "auto",
		sema:        make(chan struct{}, 2),
		stopCh:      make(chan struct{}),
		sessions:    make(map[string]*TranscodeSession),
	}

	done := make(chan struct{})
	s := &TranscodeSession{
		hash:      "test-software-fail",
		runDone:   done,
		startedAt: time.Now(),
		cancel:    func() {},
	}

	ts.sema <- struct{}{}

	cmd := exec.Command("cmd", "/C", "exit 1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start failing command: %v", err)
	}

	stderrBuf := &boundedBuffer{max: 1024}
	ts.waitFFmpeg(context.Background(), cmd, s, done, 0, stderrBuf, media.EncoderSoftware)

	if enc := ts.Encoder(); enc != media.EncoderSoftware {
		t.Errorf("expected encoder to remain %q, got %q", media.EncoderSoftware, enc)
	}

	select {
	case <-done:
	default:
		t.Error("done channel was not closed")
	}

	if ts.reprobing.Load() {
		t.Error("expected reprobing to be false when already on software")
	}

	close(ts.stopCh)
}

func TestWaitFFmpeg_CanceledRunDoesNotFallBack(t *testing.T) {
	ts := &TranscodeService{
		encoder:     media.EncoderNVENC,
		hwaccelPref: "auto",
		sema:        make(chan struct{}, 2),
		stopCh:      make(chan struct{}),
		sessions:    make(map[string]*TranscodeSession),
		ffmpeg:      media.Detect(),
	}

	done := make(chan struct{})
	s := &TranscodeSession{
		hash:      "test-canceled",
		runDone:   done,
		startedAt: time.Now(),
		cancel:    func() {},
	}

	ts.sema <- struct{}{}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "cmd", "/C", "ping -n 30 127.0.0.1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start command: %v", err)
	}
	cancel()

	stderrBuf := &boundedBuffer{max: 1024}
	ts.waitFFmpeg(ctx, cmd, s, done, 0, stderrBuf, media.EncoderNVENC)

	if enc := ts.Encoder(); enc != media.EncoderNVENC {
		t.Errorf("expected encoder to remain %q after canceled run, got %q", media.EncoderNVENC, enc)
	}

	select {
	case <-done:
	default:
		t.Error("done channel was not closed")
	}

	if ts.reprobing.Load() {
		t.Error("expected no reprobe after canceled run")
	}

	close(ts.stopCh)
}

func TestFindSessionLocked_ReusesSessionAcrossEncoderFlip(t *testing.T) {
	ts := &TranscodeService{sessions: make(map[string]*TranscodeSession)}
	orig := &TranscodeSession{hash: "hash-nvenc", srcKey: "src-a"}
	ts.sessions["hash-nvenc"] = orig

	if got := ts.findSessionLocked("hash-nvenc", "src-a"); got != orig {
		t.Error("exact hash lookup did not return the session")
	}
	if got := ts.findSessionLocked("hash-software", "src-a"); got != orig {
		t.Error("expected same-source session reuse across encoder flip")
	}
	if got := ts.findSessionLocked("hash-other", "src-b"); got != nil {
		t.Error("expected no session for a different source")
	}
}

func TestStopIdleSessions_StopsIdleKeepsActive(t *testing.T) {
	ts := &TranscodeService{
		encoder:  media.EncoderSoftware,
		sema:     make(chan struct{}, 2),
		stopCh:   make(chan struct{}),
		sessions: make(map[string]*TranscodeSession),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "cmd", "/C", "ping -n 30 127.0.0.1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start command: %v", err)
	}
	idleDone := make(chan struct{})
	idle := &TranscodeSession{
		hash:     "idle",
		dir:      t.TempDir(),
		runDone:  idleDone,
		cancel:   cancel,
		lastUsed: time.Now().Add(-2 * idleStopAfter),
	}
	activeCanceled := false
	active := &TranscodeSession{
		hash:     "active",
		runDone:  make(chan struct{}),
		cancel:   func() { activeCanceled = true },
		lastUsed: time.Now(),
	}
	ts.sessions["idle"] = idle
	ts.sessions["active"] = active

	ts.sema <- struct{}{}
	go ts.waitFFmpeg(ctx, cmd, idle, idleDone, 0, &boundedBuffer{max: 1024}, media.EncoderSoftware)

	ts.stopIdleSessions()

	select {
	case <-idleDone:
	default:
		t.Error("expected the idle session's ffmpeg to be stopped")
	}
	if activeCanceled {
		t.Error("active session must not be canceled")
	}
	if _, ok := ts.sessions["idle"]; !ok {
		t.Error("idle session must stay registered after its run is stopped")
	}

	close(ts.stopCh)
}

func TestTriggerReprobe_PreventsConcurrent(t *testing.T) {
	ts := &TranscodeService{
		encoder:     media.EncoderSoftware,
		hwaccelPref: "auto",
		sema:        make(chan struct{}, 1),
		stopCh:      make(chan struct{}),
		sessions:    make(map[string]*TranscodeSession),
	}

	ts.reprobing.Store(true)
	ts.triggerReprobe()

	// Give any goroutine a moment to start (there shouldn't be one).
	time.Sleep(50 * time.Millisecond)

	close(ts.stopCh)
	ts.wg.Wait()
}
