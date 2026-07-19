// Package media wraps external media tools (ffmpeg/ffprobe).
package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// boundedBuffer is an io.Writer wrapper around a bytes.Buffer that
// stops storing data after max bytes but keeps swallowing writes so
// the subprocess never sees a broken pipe. Caps the in-memory
// footprint of ffmpeg/ffprobe stdout/stderr against hostile inputs.
type boundedBuffer struct {
	buf  bytes.Buffer
	max  int
	over bool
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if b.over {
		return len(p), nil
	}
	if b.buf.Len()+len(p) > b.max {
		if remaining := b.max - b.buf.Len(); remaining > 0 {
			b.buf.Write(p[:remaining])
		}
		b.over = true
		return len(p), nil
	}
	return b.buf.Write(p)
}

func (b *boundedBuffer) Bytes() []byte    { return b.buf.Bytes() }
func (b *boundedBuffer) String() string   { return b.buf.String() }
func (b *boundedBuffer) Overflowed() bool { return b.over }

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

// ProbeInfo describes the first video stream of a media file.
type ProbeInfo struct {
	Codec     string  `json:"codec"`
	Width     int     `json:"width"`
	Height    int     `json:"height"`
	Duration  float64 `json:"duration"`
	Bitrate   int64   `json:"bitrate"`
	Framerate float64 `json:"framerate"`
	HasAudio  bool    `json:"hasAudio"`
}

// ErrNoVideoStream is returned when ffprobe succeeds but the file has no
// video stream (e.g. audio-only container, corrupt file).
var ErrNoVideoStream = fmt.Errorf("no video stream")

// ProbeVideo runs ffprobe and returns the first video stream's metadata.
// Returns ErrNoVideoStream if the file has no video stream.
func (f *FFmpeg) ProbeVideo(ctx context.Context, srcPath string) (*ProbeInfo, error) {
	if f.ffprobePath == "" {
		return nil, fmt.Errorf("ffprobe not available")
	}
	// Reject relative paths so a filename like "-show_data" cannot be
	// reinterpreted as a flag by ffprobe's option parser.
	if !filepath.IsAbs(srcPath) {
		return nil, fmt.Errorf("srcPath must be absolute: %q", srcPath)
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, f.ffprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-show_format",
		srcPath,
	)
	out := &boundedBuffer{max: 4 * 1024 * 1024}
	cmd.Stdout = out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffprobe: %w", err)
	}
	if out.Overflowed() {
		return nil, fmt.Errorf("ffprobe output exceeds limit")
	}

	var raw struct {
		Streams []struct {
			CodecType  string `json:"codec_type"`
			CodecName  string `json:"codec_name"`
			Width      int    `json:"width"`
			Height     int    `json:"height"`
			RFrameRate string `json:"r_frame_rate"`
			BitRate    string `json:"bit_rate"`
			Duration   string `json:"duration"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
			BitRate  string `json:"bit_rate"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out.Bytes(), &raw); err != nil {
		return nil, fmt.Errorf("parse ffprobe json: %w", err)
	}

	var hasAudio bool
	for _, s := range raw.Streams {
		if s.CodecType == "audio" {
			hasAudio = true
		}
	}
	for _, s := range raw.Streams {
		if s.CodecType != "video" {
			continue
		}
		info := &ProbeInfo{
			Codec:     s.CodecName,
			Width:     s.Width,
			Height:    s.Height,
			Framerate: parseFraction(s.RFrameRate),
			HasAudio:  hasAudio,
		}
		if d, err := strconv.ParseFloat(s.Duration, 64); err == nil && d > 0 {
			info.Duration = d
		} else if d, err := strconv.ParseFloat(raw.Format.Duration, 64); err == nil {
			info.Duration = d
		}
		if b, err := strconv.ParseInt(s.BitRate, 10, 64); err == nil && b > 0 {
			info.Bitrate = b
		} else if b, err := strconv.ParseInt(raw.Format.BitRate, 10, 64); err == nil {
			info.Bitrate = b
		}
		return info, nil
	}
	return nil, ErrNoVideoStream
}

// parseFraction parses ffprobe's "num/den" framerate strings (e.g. "30000/1001"
// → 29.97). Returns 0 on malformed input.
func parseFraction(s string) float64 {
	if s == "" || s == "0/0" {
		return 0
	}
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		v, _ := strconv.ParseFloat(s, 64)
		return v
	}
	num, err1 := strconv.ParseFloat(parts[0], 64)
	den, err2 := strconv.ParseFloat(parts[1], 64)
	if err1 != nil || err2 != nil || den == 0 {
		return 0
	}
	return num / den
}

// Probe returns the duration of the input file in seconds.
func (f *FFmpeg) Probe(ctx context.Context, srcPath string) (float64, error) {
	if f.ffprobePath == "" {
		return 0, fmt.Errorf("ffprobe not available")
	}
	if !filepath.IsAbs(srcPath) {
		return 0, fmt.Errorf("srcPath must be absolute: %q", srcPath)
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, f.ffprobePath,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "csv=p=0",
		srcPath,
	)
	out := &boundedBuffer{max: 64 * 1024}
	cmd.Stdout = out
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("ffprobe: %w", err)
	}
	if out.Overflowed() {
		return 0, fmt.Errorf("ffprobe output exceeds limit")
	}
	s := strings.TrimSpace(out.String())
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
	if !filepath.IsAbs(srcPath) {
		return fmt.Errorf("srcPath must be absolute: %q", srcPath)
	}
	if !filepath.IsAbs(dstPath) {
		return fmt.Errorf("dstPath must be absolute: %q", dstPath)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	ts := strconv.FormatFloat(timestamp, 'f', 2, 64)
	scale := fmt.Sprintf("scale=%d:-2", width)

	cmd := exec.CommandContext(ctx, f.ffmpegPath,
		"-hide_banner", "-loglevel", "error", "-nostdin",
		"-ss", ts,
		"-i", srcPath,
		"-frames:v", "1",
		"-vf", scale,
		"-q:v", "3",
		"-y",
		dstPath,
	)
	out := &boundedBuffer{max: 8 * 1024}
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg: %w: %s", err, strings.TrimSpace(out.String()))
	}
	return nil
}
