// Package media wraps external media tools (ffmpeg/ffprobe).
package media

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// FFmpeg wraps the ffmpeg and ffprobe binaries. If either is missing at
// construction time, Available returns false and ExtractPoster/Probe fail.
type FFmpeg struct {
	ffmpegPath  string
	ffprobePath string
}

// Detect looks up ffmpeg and ffprobe on PATH. Both must be present.
func Detect() *FFmpeg {
	f := &FFmpeg{}
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		f.ffmpegPath = p
	}
	if p, err := exec.LookPath("ffprobe"); err == nil {
		f.ffprobePath = p
	}
	return f
}

// Available reports whether both ffmpeg and ffprobe are on PATH.
func (f *FFmpeg) Available() bool {
	return f.ffmpegPath != "" && f.ffprobePath != ""
}

// Probe returns the duration of the input file in seconds.
func (f *FFmpeg) Probe(ctx context.Context, srcPath string) (float64, error) {
	if f.ffprobePath == "" {
		return 0, fmt.Errorf("ffprobe not available")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, f.ffprobePath,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "csv=p=0",
		srcPath,
	)
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe: %w", err)
	}
	s := strings.TrimSpace(string(out))
	d, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("parse duration %q: %w", s, err)
	}
	return d, nil
}

// ExtractPoster writes a single JPEG frame from srcPath to dstPath at the
// given timestamp (seconds) and width (height is auto to preserve aspect).
func (f *FFmpeg) ExtractPoster(ctx context.Context, srcPath, dstPath string, width int, timestamp float64) error {
	if f.ffmpegPath == "" {
		return fmt.Errorf("ffmpeg not available")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	ts := strconv.FormatFloat(timestamp, 'f', 2, 64)
	scale := fmt.Sprintf("scale=%d:-2", width)

	cmd := exec.CommandContext(ctx, f.ffmpegPath,
		"-ss", ts,
		"-i", srcPath,
		"-frames:v", "1",
		"-vf", scale,
		"-q:v", "3",
		"-y",
		dstPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
