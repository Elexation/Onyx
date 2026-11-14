package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Elexation/onyx/internal/adapter/media"
	"github.com/Elexation/onyx/internal/adapter/storage"
)

// A playlist the UI never plays as video must not reach ffprobe, which would follow its references
func TestProbeRejectsNonVideoBeforeFFprobe(t *testing.T) {
	dir := t.TempDir()
	playlist := "#EXTM3U\n#EXT-X-TARGETDURATION:10\n#EXTINF:10,\n/etc/secret.ts\n#EXT-X-ENDLIST\n"
	files := map[string]string{
		"notes.txt": "hello",
		"list.m3u8": playlist,
		"noext":     playlist,
		"song.mp3":  "ID3",
		"clip.mp4":  "not really a video",
		"movie.mkv": "not really a video",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	ls, err := storage.NewLocalStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer ls.Close()
	ps, err := NewProbeService(ls, dir)
	if err != nil {
		t.Fatal(err)
	}
	// No binaries, so reaching ffprobe fails with an error other than ErrNoVideoStream
	ps.ffmpeg = &media.FFmpeg{}

	for _, name := range []string{"notes.txt", "list.m3u8", "noext", "song.mp3"} {
		_, err := ps.Probe(context.Background(), "/"+name)
		if !errors.Is(err, ErrNoVideoStream) {
			t.Errorf("%s: err = %v, want ErrNoVideoStream before ffprobe", name, err)
		}
	}
	for _, name := range []string{"clip.mp4", "movie.mkv"} {
		_, err := ps.Probe(context.Background(), "/"+name)
		if err == nil || errors.Is(err, ErrNoVideoStream) {
			t.Errorf("%s: err = %v, want it to reach ffprobe", name, err)
		}
	}
}

// A leader whose request is cancelled while queued must not hand its followers (nil, nil)
func TestProbeSingleFlightLeaderCancelled(t *testing.T) {
	dir := t.TempDir()
	ls, err := storage.NewLocalStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer ls.Close()
	ps, err := NewProbeService(ls, dir)
	if err != nil {
		t.Fatal(err)
	}
	ps.ffmpeg = &media.FFmpeg{}

	// Every probe slot busy, so the leader queues
	for i := 0; i < cap(ps.sema); i++ {
		ps.sema <- struct{}{}
	}
	abs := filepath.Join(dir, "clip.mp4")

	leaderCtx, cancelLeader := context.WithCancel(context.Background())
	leaderDone := make(chan struct{})
	go func() {
		ps.probeSingleFlight(leaderCtx, abs, 1)
		close(leaderDone)
	}()
	for {
		if _, ok := ps.inflight.Load(abs); ok {
			break
		}
		time.Sleep(time.Millisecond)
	}

	type result struct {
		info *media.ProbeInfo
		err  error
	}
	followerCtx, cancelFollower := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancelFollower()
	got := make(chan result, 1)
	go func() {
		info, err := ps.probeSingleFlight(followerCtx, abs, 1)
		got <- result{info, err}
	}()
	time.Sleep(50 * time.Millisecond)
	cancelLeader()
	<-leaderDone

	r := <-got
	if r.info == nil && r.err == nil {
		t.Fatal("follower got (nil, nil) after the leader was cancelled")
	}
	if !errors.Is(r.err, context.DeadlineExceeded) {
		t.Fatalf("follower err = %v, want its own deadline after retrying", r.err)
	}
}
