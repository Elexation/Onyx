package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	_ "image/gif"
	_ "image/png"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	"golang.org/x/image/draw"

	"github.com/Elexation/onyx/internal/adapter/media"
	"github.com/Elexation/onyx/internal/adapter/storage"
)

// ThumbSize is one of small / medium / large.
type ThumbSize string

const (
	ThumbSmall  ThumbSize = "small"
	ThumbMedium ThumbSize = "medium"
	ThumbLarge  ThumbSize = "large"
)

var thumbWidths = map[ThumbSize]int{
	ThumbSmall:  128,
	ThumbMedium: 256,
	ThumbLarge:  384,
}

// maxThumbInputPixels caps total source pixels (width*height). image.Decode
// allocates the full backing buffer (Width*Height*4 for RGBA) before reading
// IDAT/SOF data — a ~1KB crafted PNG declaring 65535x65535 would otherwise
// allocate ~16 GB and OOM-kill the server. image.Decode is unbounded, so a
// DecodeConfig pre-check is the only mitigation; the per-IP streamRL token
// bucket bounds request rate but not memory per request.
const maxThumbInputPixels = 64 * 1024 * 1024 // 64 MP ≈ 8192x8192

// maxThumbInputDim caps either dimension. Some decoders (TIFF strip-based)
// can pass a small pixel-product check on a 1×10⁹ sliver, so cap each side
// independently before image.Decode runs.
const maxThumbInputDim = 16384

// ParseThumbSize returns the named size or false if unknown.
func ParseThumbSize(s string) (ThumbSize, bool) {
	switch ThumbSize(s) {
	case ThumbSmall, ThumbMedium, ThumbLarge:
		return ThumbSize(s), true
	}
	return "", false
}

// LookupStatus is the result of a thumbnail lookup request.
type LookupStatus int

const (
	// StatusReady — thumbnail exists on disk, FilePath is set.
	StatusReady LookupStatus = iota
	// StatusQueued — generation has been requested, try again shortly.
	StatusQueued
	// StatusUnsupported — file type cannot produce a thumbnail.
	StatusUnsupported
	// StatusFailed — generation was attempted and failed; fail marker is fresh.
	StatusFailed
)

// LookupResult carries the outcome of ThumbnailService.Lookup.
type LookupResult struct {
	Status   LookupStatus
	FilePath string
}

type thumbJob struct {
	relPath string
	mtime   int64
	size    ThumbSize
	dst     string
	failDst string
	kind    thumbKind
	key     string
}

type thumbKind int

const (
	kindNone thumbKind = iota
	kindImage
	kindVideo
)

// ThumbnailService generates and caches file thumbnails in the background.
type ThumbnailService struct {
	storage  *storage.LocalStorage
	store    *storage.ThumbStore
	ffmpeg   *media.FFmpeg
	dataDir  string
	realRoot string // canonical dataDir after EvalSymlinks

	jobs     chan thumbJob
	inflight sync.Map
	workers  int

	failTTL  time.Duration
	lruTTL   time.Duration

	events EventRecorder
}

// NewThumbnailService wires the service. ffmpeg is probed here; if missing,
// video thumbnails will return StatusUnsupported.
func NewThumbnailService(s *storage.LocalStorage, dataDir, thumbDir string) (*ThumbnailService, error) {
	store, err := storage.NewThumbStore(thumbDir)
	if err != nil {
		return nil, err
	}
	realRoot, err := filepath.EvalSymlinks(dataDir)
	if err != nil {
		return nil, fmt.Errorf("resolve data dir: %w", err)
	}
	workers := runtime.NumCPU() / 2
	if workers < 1 {
		workers = 1
	}
	return &ThumbnailService{
		storage:  s,
		store:    store,
		ffmpeg:   media.Detect(),
		dataDir:  dataDir,
		realRoot: realRoot,
		jobs:     make(chan thumbJob, 1000),
		workers:  workers,
		failTTL:  1 * time.Hour,
		lruTTL:   30 * 24 * time.Hour,
	}, nil
}

// Start launches the worker pool.
func (ts *ThumbnailService) Start() {
	slog.Info("thumbnail service: starting", "workers", ts.workers, "ffmpeg", ts.ffmpeg.Available())
	for i := 0; i < ts.workers; i++ {
		go ts.worker()
	}
}

func (ts *ThumbnailService) SetEvents(r EventRecorder) {
	ts.events = r
}

// StartJanitor periodically sweeps stale fail markers and LRU-evicts old thumbs.
func (ts *ThumbnailService) StartJanitor(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			ts.sweep()
		}
	}()
}

// HasFFmpeg reports whether video thumbnails can be generated.
func (ts *ThumbnailService) HasFFmpeg() bool { return ts.ffmpeg.Available() }

// Lookup checks the cache for a thumbnail and queues generation on miss.
func (ts *ThumbnailService) Lookup(relPath string, size ThumbSize) (LookupResult, error) {
	info, err := ts.storage.Stat(relPath)
	if err != nil {
		return LookupResult{}, err
	}
	if info.IsDir {
		return LookupResult{Status: StatusUnsupported}, nil
	}

	kind := classifyForThumbnail(info.MIMEType)
	if kind == kindNone {
		return LookupResult{Status: StatusUnsupported}, nil
	}
	if kind == kindVideo && !ts.ffmpeg.Available() {
		return LookupResult{Status: StatusUnsupported}, nil
	}

	hash := sha256.Sum256([]byte(info.Path))
	dst := ts.store.Path(hash[:], info.ModTime, string(size))
	if ts.store.Exists(dst) {
		ts.store.Touch(dst)
		return LookupResult{Status: StatusReady, FilePath: dst}, nil
	}

	failDst := ts.store.FailPath(hash[:], info.ModTime, string(size))
	if ts.store.IsFailFresh(failDst, ts.failTTL) {
		return LookupResult{Status: StatusFailed}, nil
	}

	key := fmt.Sprintf("%x-%d-%s", hash[:8], info.ModTime, size)
	job := thumbJob{
		relPath: info.Path,
		mtime:   info.ModTime,
		size:    size,
		dst:     dst,
		failDst: failDst,
		kind:    kind,
		key:     key,
	}

	if _, loaded := ts.inflight.LoadOrStore(key, struct{}{}); loaded {
		return LookupResult{Status: StatusQueued}, nil
	}
	select {
	case ts.jobs <- job:
		return LookupResult{Status: StatusQueued}, nil
	default:
		ts.inflight.Delete(key)
		return LookupResult{Status: StatusQueued}, nil
	}
}

func (ts *ThumbnailService) worker() {
	for job := range ts.jobs {
		ts.run(job)
		ts.inflight.Delete(job.key)
	}
}

func (ts *ThumbnailService) run(job thumbJob) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("thumbnail: panic during generation", "path", job.relPath, "size", job.size, "panic", r)
			if markerErr := ts.store.WriteFailMarker(job.failDst); markerErr != nil {
				slog.Warn("thumbnail: fail marker write", "error", markerErr)
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var err error
	switch job.kind {
	case kindImage:
		err = ts.generateImage(job)
	case kindVideo:
		err = ts.generateVideo(ctx, job)
	}
	if err != nil {
		slog.Warn("thumbnail: generation failed", "path", job.relPath, "size", job.size, "error", err)
		if markerErr := ts.store.WriteFailMarker(job.failDst); markerErr != nil {
			slog.Warn("thumbnail: fail marker write", "error", markerErr)
		}
		return
	}
	recordIf(ts.events, "thumb.ready", ThumbReadyPayload{Path: job.relPath})
}

func (ts *ThumbnailService) generateImage(job thumbJob) error {
	reader, _, _, err := ts.storage.Open(job.relPath)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer reader.Close()

	orientation := 1
	if isJPEG(job.relPath) {
		orientation = readJPEGOrientation(reader)
		if _, err := reader.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("seek: %w", err)
		}
	}

	cfg, _, err := image.DecodeConfig(reader)
	if err != nil {
		return fmt.Errorf("decode config: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 ||
		cfg.Width > maxThumbInputDim || cfg.Height > maxThumbInputDim {
		return fmt.Errorf("image dimensions out of range: %dx%d", cfg.Width, cfg.Height)
	}
	if int64(cfg.Width)*int64(cfg.Height) > maxThumbInputPixels {
		return fmt.Errorf("image pixel count exceeds limit: %dx%d", cfg.Width, cfg.Height)
	}
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek: %w", err)
	}

	src, _, err := image.Decode(reader)
	if err != nil {
		return fmt.Errorf("decode: %w", err)
	}

	width := thumbWidths[job.size]
	dstImg := resizeKeepAspect(src, width)
	if orientation != 1 {
		dstImg = applyOrientation(dstImg, orientation)
	}

	return ts.store.SaveAtomic(job.dst, func(f *os.File) error {
		return jpeg.Encode(f, dstImg, &jpeg.Options{Quality: 85})
	})
}

func (ts *ThumbnailService) generateVideo(ctx context.Context, job thumbJob) error {
	srcAbs, err := ts.resolveSafePath(job.relPath)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}
	duration, err := ts.ffmpeg.Probe(ctx, srcAbs)
	if err != nil {
		return fmt.Errorf("probe: %w", err)
	}
	ts.store.EnsureDir(job.dst)

	timestamp := math.Min(10, duration*0.25)
	if timestamp < 0.5 {
		timestamp = 0.5
	}
	if duration > 0 && timestamp > duration-0.1 {
		timestamp = math.Max(0, duration-0.1)
	}

	width := thumbWidths[job.size]
	return ts.ffmpeg.ExtractPoster(ctx, srcAbs, job.dst, width, timestamp)
}

// resolveSafePath validates relPath via os.Root then returns an absolute OS
// path confined to the data directory. Symlinks that escape dataDir are rejected.
func (ts *ThumbnailService) resolveSafePath(relPath string) (string, error) {
	if _, err := ts.storage.Stat(relPath); err != nil {
		return "", err
	}

	clean := strings.TrimLeft(relPath, "/")
	clean = filepath.Clean(filepath.FromSlash(clean))
	if clean == ".." || strings.HasPrefix(clean, string(filepath.Separator)+"..") || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == "." {
		return "", fmt.Errorf("invalid path")
	}

	osPath := filepath.Join(ts.dataDir, filepath.FromSlash(clean))
	resolved, err := filepath.EvalSymlinks(osPath)
	if err != nil {
		return "", err
	}
	root := ts.realRoot + string(filepath.Separator)
	if resolved != ts.realRoot && !strings.HasPrefix(resolved, root) {
		return "", fmt.Errorf("path escapes data directory")
	}
	return resolved, nil
}

func (ts *ThumbnailService) sweep() {
	var removedFails, removedThumbs int
	now := time.Now()
	_ = ts.store.Walk(func(p string, info os.FileInfo) {
		name := info.Name()
		if strings.HasSuffix(name, ".fail") {
			if now.Sub(info.ModTime()) > ts.failTTL {
				ts.store.Remove(p)
				removedFails++
			}
			return
		}
		if strings.HasSuffix(name, ".jpg") {
			if now.Sub(info.ModTime()) > ts.lruTTL {
				ts.store.Remove(p)
				removedThumbs++
			}
		}
	})
	if removedFails+removedThumbs > 0 {
		slog.Info("thumbnail janitor: swept", "fails", removedFails, "thumbs", removedThumbs)
	}
}

// classifyForThumbnail maps MIME types to the generator pipeline that
// handles them. Unsupported types return kindNone.
func classifyForThumbnail(mime string) thumbKind {
	switch mime {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "image/bmp", "image/tiff":
		return kindImage
	case "video/mp4", "video/webm", "video/quicktime", "video/x-matroska", "video/x-msvideo", "video/ogg":
		return kindVideo
	}
	return kindNone
}

// resizeKeepAspect fits src into a box of the given maximum edge length,
// preserving aspect ratio. The longest edge becomes maxEdge.
func resizeKeepAspect(src image.Image, maxEdge int) image.Image {
	b := src.Bounds()
	sw := b.Dx()
	sh := b.Dy()
	if sw <= maxEdge && sh <= maxEdge {
		return src
	}
	var dw, dh int
	if sw >= sh {
		dw = maxEdge
		dh = int(float64(sh) * float64(maxEdge) / float64(sw))
	} else {
		dh = maxEdge
		dw = int(float64(sw) * float64(maxEdge) / float64(sh))
	}
	if dw < 1 {
		dw = 1
	}
	if dh < 1 {
		dh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)
	return dst
}
