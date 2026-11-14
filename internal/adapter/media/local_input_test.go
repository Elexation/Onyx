package media

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// Check the whitelists sit right before the input, where ffmpeg reads them as input options
func TestBuildHLSCommandRestrictsInput(t *testing.T) {
	f := &FFmpeg{ffmpegPath: "ffmpeg", ffprobePath: "ffprobe"}
	src := filepath.Join(t.TempDir(), "in.mp4")
	cmd, err := f.BuildHLSCommand(context.Background(), HLSOptions{
		SrcPath:    src,
		OutDir:     t.TempDir(),
		Renditions: []Rendition{{Height: 240, VBitrate: "300k", MaxRate: "330k", BufSize: "450k"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	i := slices.Index(cmd.Args, "-i")
	n := len(localInputArgs)
	if i < n || !slices.Equal(cmd.Args[i-n:i], localInputArgs) || cmd.Args[i+1] != src {
		t.Fatalf("want %v right before -i %s, got %v", localInputArgs, src, cmd.Args)
	}
}

// A concat list or DASH manifest renamed .mp4 passes the type gate but must not be demuxed
func TestManifestDisguisedAsVideoIsRefused(t *testing.T) {
	f := Detect()
	if !f.Available() {
		t.Skip("ffmpeg/ffprobe not on PATH")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "private.mp4")
	gen := exec.CommandContext(ctx, f.ffmpegPath, "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=size=320x240:rate=10:duration=2", "-pix_fmt", "yuv420p", outside)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("generate sample: %v: %s", err, out)
	}
	if _, err := f.ProbeVideo(ctx, outside); err != nil {
		t.Fatalf("real video must still probe: %v", err)
	}

	abs := filepath.ToSlash(outside)
	manifests := map[string]string{
		"concat.mp4": "ffconcat version 1.0\nfile private.mp4\n",
		"dash.mp4": `<?xml version="1.0"?><MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="static" mediaPresentationDuration="PT2S" profiles="urn:mpeg:dash:profile:isoff-on-demand:2011">` +
			`<Period><AdaptationSet mimeType="video/mp4"><Representation id="1" bandwidth="1000"><BaseURL>` + abs + `</BaseURL></Representation></AdaptationSet></Period></MPD>`,
	}
	if err := os.Link(outside, filepath.Join(dir, "private.mp4")); err != nil {
		data, rerr := os.ReadFile(outside)
		if rerr != nil {
			t.Fatal(rerr)
		}
		if err := os.WriteFile(filepath.Join(dir, "private.mp4"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range manifests {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if info, err := f.ProbeVideo(ctx, p); err == nil {
			t.Errorf("%s: probed as %dx%d video, want refused", name, info.Width, info.Height)
		}
		if _, err := f.Probe(ctx, p); err == nil {
			t.Errorf("%s: duration probe succeeded, want refused", name)
		}
	}
}

// Probe, poster and HLS transcode must still work on a real local video with the whitelist set
func TestLocalOnlyInputStillTranscodes(t *testing.T) {
	f := Detect()
	if !f.Available() {
		t.Skip("ffmpeg/ffprobe not on PATH")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	dir := t.TempDir()
	src := filepath.Join(dir, "clip.mp4")
	gen := exec.CommandContext(ctx, f.ffmpegPath, "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=size=320x240:rate=10:duration=3",
		"-pix_fmt", "yuv420p", src)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("generate sample: %v: %s", err, out)
	}

	info, err := f.ProbeVideo(ctx, src)
	if err != nil {
		t.Fatalf("ProbeVideo: %v", err)
	}
	if info.Width != 320 || info.Height != 240 {
		t.Fatalf("probe = %dx%d, want 320x240", info.Width, info.Height)
	}
	if d, err := f.Probe(ctx, src); err != nil || d <= 0 {
		t.Fatalf("Probe duration = %v, %v", d, err)
	}
	if err := f.ExtractPoster(ctx, src, filepath.Join(dir, "poster.jpg"), 160, 1); err != nil {
		t.Fatalf("ExtractPoster: %v", err)
	}

	out := filepath.Join(dir, "hls")
	if err := os.MkdirAll(filepath.Join(out, VariantDir(0)), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd, err := f.BuildHLSCommand(ctx, HLSOptions{
		SrcPath:    src,
		OutDir:     out,
		Encoder:    EncoderSoftware,
		Renditions: []Rendition{{Height: 240, VBitrate: "300k", MaxRate: "330k", BufSize: "450k"}},
		Framerate:  info.Framerate,
	})
	if err != nil {
		t.Fatal(err)
	}
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("transcode: %v: %s", err, b)
	}
	for _, name := range []string{HLSInitName(0, 1), HLSSegmentName(0, 0)} {
		if st, err := os.Stat(filepath.Join(out, name)); err != nil || st.Size() == 0 {
			t.Fatalf("missing HLS output %s: %v", name, err)
		}
	}
}
