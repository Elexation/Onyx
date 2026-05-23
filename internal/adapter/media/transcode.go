package media

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// HLSSegmentSeconds is the target duration of every HLS segment in seconds.
// Keyframe alignment is forced at this interval via -force_key_frames.
const HLSSegmentSeconds = 6

// Rendition describes one rung of the ABR ladder: output height and
// bitrate caps. Width is derived from the source aspect ratio at encode
// time via scale=-2:<height>.
type Rendition struct {
	Height   int
	VBitrate string
	MaxRate  string
	BufSize  string
}

// Ladder is the fixed 5-rung ABR table. Bitrates are chosen to produce
// a roughly 2× step between rungs so hls.js ABR has meaningful choices.
var Ladder = []Rendition{
	{Height: 2160, VBitrate: "18000k", MaxRate: "19800k", BufSize: "27000k"},
	{Height: 1440, VBitrate: "9000k", MaxRate: "9900k", BufSize: "13500k"},
	{Height: 1080, VBitrate: "5000k", MaxRate: "5500k", BufSize: "7500k"},
	{Height: 720, VBitrate: "2800k", MaxRate: "3080k", BufSize: "4200k"},
	{Height: 480, VBitrate: "1400k", MaxRate: "1540k", BufSize: "2100k"},
}

// SelectRungs returns the subset of Ladder to encode for a source of
// sourceHeight pixels, capped at capHeight. capHeight <= 0 means no
// cap. If no ladder rung fits (source < 480p), returns a single
// rendition at the source height using the smallest ladder bitrates.
func SelectRungs(sourceHeight, capHeight int) []Rendition {
	maxH := sourceHeight
	if capHeight > 0 && capHeight < maxH {
		maxH = capHeight
	}
	var out []Rendition
	for _, r := range Ladder {
		if r.Height <= maxH {
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		out = []Rendition{{
			Height:   sourceHeight,
			VBitrate: "1400k",
			MaxRate:  "1540k",
			BufSize:  "2100k",
		}}
	}
	return out
}

// HLSOptions describes a single multi-variant HLS fMP4 transcode run.
type HLSOptions struct {
	SrcPath      string
	OutDir       string
	StartSegment int
	Encoder      Encoder
	Renditions   []Rendition
	HasAudio     bool
}

// BuildHLSCommand returns an unstarted exec.Cmd configured to produce
// one fMP4 variant per Rendition into OutDir/stream_{N}/. Writes
// init.mp4 + data{NNNNNN}.m4s per variant. The scratch _ffmpeg.m3u8
// playlists are written by ffmpeg but ignored by the service; the
// service writes its own master.m3u8 + stream_{N}/playlist.m3u8 with
// absolute server URIs.
//
// CPU-side scaling is used universally (even for NVENC on Blackwell)
// to sidestep the scale_cuda NV_ENC_ERR_INVALID_PARAM bug on driver
// 595.x. Costs some throughput on high-end NVIDIA but works on every
// GPU.
//
// When StartSegment > 0 the command uses input-side -ss for fast seek,
// which aligns to the nearest prior keyframe. With -force_key_frames
// at HLSSegmentSeconds boundaries every seek boundary is also a
// keyframe, so segment timing stays aligned with the pre-written
// playlist.
func (f *FFmpeg) BuildHLSCommand(ctx context.Context, opts HLSOptions) (*exec.Cmd, error) {
	if f.ffmpegPath == "" {
		return nil, fmt.Errorf("ffmpeg not available")
	}
	if len(opts.Renditions) == 0 {
		return nil, fmt.Errorf("no renditions")
	}
	encoder := opts.Encoder
	if encoder == "" {
		encoder = EncoderSoftware
	}

	args := []string{"-hide_banner", "-loglevel", "warning", "-nostdin"}

	// Pre-input hwaccel device setup. NVENC and AMF accept CPU frames
	// and need no pre-input flags; libx264 obviously not.
	switch encoder {
	case EncoderVAAPI:
		args = append(args, "-vaapi_device", "/dev/dri/renderD128")
	case EncoderQSV:
		args = append(args, "-init_hw_device", "qsv=hw", "-filter_hw_device", "hw")
	}

	if opts.StartSegment > 0 {
		args = append(args, "-ss", strconv.Itoa(opts.StartSegment*HLSSegmentSeconds))
	}

	args = append(args, "-i", opts.SrcPath)

	// Filter complex: split + per-variant scale. For VAAPI/QSV the
	// chain ends with format=nv12,hwupload so the encoder sees a GPU
	// frame; other encoders stay CPU-side all the way.
	args = append(args, "-filter_complex", buildFilterComplex(opts.Renditions, encoder))

	// Per-rendition video mapping + encoder flags.
	for i, r := range opts.Renditions {
		args = append(args, "-map", fmt.Sprintf("[v%dout]", i))
		args = append(args, buildVideoEncoderArgs(encoder, i, r)...)
	}

	// Per-rendition audio mapping — each variant gets its own
	// re-encoded audio stream so the variant is self-contained. Skipped
	// entirely for sources with no audio; the HLS muxer's var_stream_map
	// requires referenced streams to exist, so `-map a:0?` isn't enough.
	if opts.HasAudio {
		for i := range opts.Renditions {
			args = append(args,
				"-map", "a:0",
				fmt.Sprintf("-c:a:%d", i), "aac",
				fmt.Sprintf("-b:a:%d", i), "192k",
				fmt.Sprintf("-ac:%d", i), "2",
			)
		}
	}

	args = append(args,
		"-force_key_frames", fmt.Sprintf("expr:gte(t,n_forced*%d)", HLSSegmentSeconds),
		"-f", "hls",
		"-hls_time", strconv.Itoa(HLSSegmentSeconds),
		"-hls_segment_type", "fmp4",
		"-hls_flags", "independent_segments",
		"-hls_list_size", "0",
		"-hls_fmp4_init_filename", "init.mp4",
		"-hls_segment_filename", "stream_%v/data%06d.m4s",
		"-master_pl_name", "_ffmpeg_master.m3u8",
		"-var_stream_map", buildVarStreamMap(len(opts.Renditions), opts.HasAudio),
		"-start_number", strconv.Itoa(opts.StartSegment),
		"stream_%v/_ffmpeg.m3u8",
	)

	cmd := exec.CommandContext(ctx, f.ffmpegPath, args...)
	// ffmpeg's -hls_fmp4_init_filename and -hls_segment_filename are
	// relative to CWD, so set Dir to keep all output inside the
	// session cache instead of leaking into the server's working
	// directory.
	cmd.Dir = opts.OutDir
	return cmd, nil
}

// buildFilterComplex emits the [0:v]split=N[v0][v1]...; [v0]scale=...
// per-rung chain. For VAAPI/QSV the chain ends with format=nv12,hwupload
// so the encoder sees a GPU frame.
func buildFilterComplex(rungs []Rendition, enc Encoder) string {
	n := len(rungs)
	var b strings.Builder
	b.WriteString("[0:v]split=")
	b.WriteString(strconv.Itoa(n))
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "[v%d]", i)
	}
	b.WriteString(";")
	for i, r := range rungs {
		fmt.Fprintf(&b, "[v%d]scale=-2:%d", i, r.Height)
		switch enc {
		case EncoderVAAPI:
			b.WriteString(",format=nv12,hwupload")
		case EncoderQSV:
			b.WriteString(",format=nv12,hwupload=extra_hw_frames=16")
		}
		fmt.Fprintf(&b, "[v%dout]", i)
		if i < n-1 {
			b.WriteString(";")
		}
	}
	return b.String()
}

// buildVideoEncoderArgs returns the per-rendition video encoder flags
// for the given encoder and rung index. All encoders use VBR with a
// per-rung target bitrate + maxrate + bufsize so the ABR ladder has
// predictable bandwidth rungs regardless of which encoder is selected.
func buildVideoEncoderArgs(enc Encoder, i int, r Rendition) []string {
	sfx := fmt.Sprintf(":v:%d", i)
	switch enc {
	case EncoderNVENC:
		return []string{
			"-c" + sfx, string(enc),
			"-preset" + sfx, "p4",
			"-tune" + sfx, "hq",
			"-rc" + sfx, "vbr",
			"-b" + sfx, r.VBitrate,
			"-maxrate" + sfx, r.MaxRate,
			"-bufsize" + sfx, r.BufSize,
			"-profile" + sfx, "high",
		}
	case EncoderQSV:
		return []string{
			"-c" + sfx, string(enc),
			"-preset" + sfx, "medium",
			"-b" + sfx, r.VBitrate,
			"-maxrate" + sfx, r.MaxRate,
			"-bufsize" + sfx, r.BufSize,
			"-profile" + sfx, "high",
		}
	case EncoderVAAPI:
		return []string{
			"-c" + sfx, string(enc),
			"-b" + sfx, r.VBitrate,
			"-maxrate" + sfx, r.MaxRate,
			"-bufsize" + sfx, r.BufSize,
			"-profile" + sfx, "high",
		}
	case EncoderAMF:
		return []string{
			"-c" + sfx, string(enc),
			"-quality" + sfx, "quality",
			"-rc" + sfx, "vbr_peak",
			"-b" + sfx, r.VBitrate,
			"-maxrate" + sfx, r.MaxRate,
			"-bufsize" + sfx, r.BufSize,
			"-profile" + sfx, "high",
		}
	default:
		return []string{
			"-c" + sfx, string(EncoderSoftware),
			"-preset" + sfx, "fast",
			"-b" + sfx, r.VBitrate,
			"-maxrate" + sfx, r.MaxRate,
			"-bufsize" + sfx, r.BufSize,
			"-profile" + sfx, "high",
			"-level" + sfx, "4.0",
			"-pix_fmt" + sfx, "yuv420p",
		}
	}
}

// buildVarStreamMap emits "v:0,a:0 v:1,a:1 ..." for ffmpeg's
// -var_stream_map flag, or "v:0 v:1 ..." when the source has no audio.
// Each entry becomes one HLS variant.
func buildVarStreamMap(n int, hasAudio bool) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteString(" ")
		}
		if hasAudio {
			fmt.Fprintf(&b, "v:%d,a:%d", i, i)
		} else {
			fmt.Fprintf(&b, "v:%d", i)
		}
	}
	return b.String()
}

// VariantDir returns the subdirectory name for variant v (relative to
// the session cache directory).
func VariantDir(variant int) string {
	return fmt.Sprintf("stream_%d", variant)
}

// HLSSegmentName returns the on-disk filename for segment n in variant
// v (relative to the session cache directory). Must match the
// -hls_segment_filename template above.
func HLSSegmentName(variant, n int) string {
	return fmt.Sprintf("stream_%d/data%06d.m4s", variant, n)
}

// HLSInitName returns the on-disk filename for variant v's init segment
// (relative to the session cache directory). ffmpeg substitutes %v into
// both the segment dir and the init basename when var_stream_map is set,
// so the actual path is stream_{v}/init_{v}.mp4.
func HLSInitName(variant int) string {
	return fmt.Sprintf("stream_%d/init_%d.mp4", variant, variant)
}
