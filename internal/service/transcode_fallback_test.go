package service

import (
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
	ts.waitFFmpeg(cmd, s, done, 0, stderrBuf, media.EncoderNVENC)

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
	ts.waitFFmpeg(cmd, s, done, 0, stderrBuf, media.EncoderNVENC)

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
	ts.waitFFmpeg(cmd, s, done, 0, stderrBuf, media.EncoderSoftware)

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
