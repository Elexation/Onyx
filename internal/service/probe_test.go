package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

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
