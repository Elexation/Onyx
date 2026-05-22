package media

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// Encoder identifies an ffmpeg H.264 video encoder. Values map directly
// to the -c:v argument.
type Encoder string

const (
	EncoderNVENC    Encoder = "h264_nvenc"
	EncoderQSV      Encoder = "h264_qsv"
	EncoderVAAPI    Encoder = "h264_vaapi"
	EncoderAMF      Encoder = "h264_amf"
	EncoderSoftware Encoder = "libx264"
)

// probeOrder is the preference when ONYX_HWACCEL=auto. Software is
// always the last resort and is not probed.
var probeOrder = []Encoder{EncoderNVENC, EncoderQSV, EncoderVAAPI, EncoderAMF}

// Probe holds the set of encoders that both appear in the installed
// ffmpeg's -encoders listing and succeed at a minimal test encode.
// Ordered by probeOrder.
type Probe struct {
	Available []Encoder
}

// RunStartupProbe enumerates hardware encoders available on this host.
// Runs `ffmpeg -hide_banner -encoders` to discover which encoders are
// linked into the ffmpeg binary, then runs a short test encode for
// each candidate. Encoders that are listed but fail the test (driver
// mismatch, missing device node, permission denied) are logged at
// WARN once and excluded from Available; successes are logged at INFO
// once.
//
// Software libx264 is always usable, is not part of the returned list,
// and Select falls back to it when no hardware encoder matches.
func RunStartupProbe(ctx context.Context, f *FFmpeg) Probe {
	if f == nil || f.ffmpegPath == "" {
		slog.Info("hwaccel probe skipped: ffmpeg not available")
		return Probe{}
	}
	listed, err := listEncoders(ctx, f.ffmpegPath)
	if err != nil {
		slog.Warn("hwaccel probe: -encoders listing failed", "error", err)
		return Probe{}
	}
	var available []Encoder
	for _, enc := range probeOrder {
		if _, ok := listed[string(enc)]; !ok {
			continue
		}
		if err := testEncode(ctx, f.ffmpegPath, enc); err != nil {
			slog.Warn("hwaccel probe: encoder failed test encode",
				"encoder", enc, "error", err)
			continue
		}
		slog.Info("hwaccel probe: encoder available", "encoder", enc)
		available = append(available, enc)
	}
	if len(available) == 0 {
		slog.Info("hwaccel probe: no hardware encoders available, will use libx264")
	}
	return Probe{Available: available}
}

// Select returns the encoder to use given an ONYX_HWACCEL preference.
// Values: "auto" (prefer first available hardware encoder, else
// software), "nvenc" / "qsv" / "vaapi" / "amf" (force specific, fall
// back to software with WARN if unavailable), "none" / "software" /
// "libx264" (force software).
func (p Probe) Select(forced string) Encoder {
	forced = strings.ToLower(strings.TrimSpace(forced))
	switch forced {
	case "", "auto":
		if len(p.Available) > 0 {
			return p.Available[0]
		}
		return EncoderSoftware
	case "none", "software", "libx264":
		return EncoderSoftware
	case "nvenc", "h264_nvenc":
		return p.pick(EncoderNVENC)
	case "qsv", "h264_qsv":
		return p.pick(EncoderQSV)
	case "vaapi", "h264_vaapi":
		return p.pick(EncoderVAAPI)
	case "amf", "h264_amf":
		return p.pick(EncoderAMF)
	default:
		slog.Warn("hwaccel: unknown ONYX_HWACCEL value, falling back to software", "value", forced)
		return EncoderSoftware
	}
}

func (p Probe) pick(want Encoder) Encoder {
	for _, enc := range p.Available {
		if enc == want {
			return enc
		}
	}
	slog.Warn("hwaccel: forced encoder unavailable, falling back to software", "wanted", want)
	return EncoderSoftware
}

// listEncoders parses `ffmpeg -hide_banner -encoders` output into a
// set of encoder names. The output format is a two-column table with
// a header terminated by "------". Lines after the header start with
// 6-char capability flags followed by the encoder name.
func listEncoders(ctx context.Context, ffmpegPath string) (map[string]struct{}, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, ffmpegPath, "-hide_banner", "-encoders")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg -encoders: %w", err)
	}
	set := make(map[string]struct{})
	var inTable bool
	for _, line := range strings.Split(out.String(), "\n") {
		if !inTable {
			if strings.Contains(line, "------") {
				inTable = true
			}
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		set[fields[1]] = struct{}{}
	}
	return set, nil
}

// testEncode runs a tiny encode using the given encoder. If the
// encoder is listed in the ffmpeg binary but fails to initialize
// (missing driver, no device node, permission denied), the process
// exits non-zero and the encoder is excluded from Probe.Available.
func testEncode(ctx context.Context, ffmpegPath string, enc Encoder) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	args := []string{
		"-hide_banner", "-loglevel", "error", "-nostdin",
		"-f", "lavfi", "-i", "testsrc=size=320x240:rate=1:duration=0.1",
	}
	switch enc {
	case EncoderVAAPI:
		args = append(args,
			"-vaapi_device", "/dev/dri/renderD128",
			"-vf", "format=nv12,hwupload",
		)
	case EncoderQSV:
		args = append(args,
			"-init_hw_device", "qsv=hw",
			"-filter_hw_device", "hw",
			"-vf", "format=nv12,hwupload=extra_hw_frames=16",
		)
	}
	args = append(args, "-c:v", string(enc), "-f", "null", "-")
	cmd := exec.CommandContext(ctx, ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		tail := strings.TrimSpace(stderr.String())
		if len(tail) > 200 {
			tail = tail[len(tail)-200:]
		}
		return fmt.Errorf("%w: %s", err, tail)
	}
	return nil
}
