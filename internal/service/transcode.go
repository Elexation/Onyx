package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Elexation/onyx/internal/adapter/media"
	"github.com/Elexation/onyx/internal/adapter/storage"
)

// Block-and-wait tuning for segment readiness and cache expiry.
const (
	segmentWaitTimeout   = 30 * time.Second
	segmentPollInterval  = 100 * time.Millisecond
	forwardSeekTolerance = 10 // restart if N > currentStartSegment + this
	cacheExpiry          = 24 * time.Hour
	cleanupInterval      = 1 * time.Hour
)

// ErrSessionNotFound is returned when GetSegment is called with an
// unknown hash (session never started or was evicted).
var ErrSessionNotFound = errors.New("transcode session not found")

// ErrSegmentTimeout is returned when a segment does not appear within
// segmentWaitTimeout. Maps to HTTP 504.
var ErrSegmentTimeout = errors.New("segment not ready")

// ErrSegmentOutOfRange is returned when a segment number exceeds the
// maximum derived from the source duration. Maps to HTTP 400.
var ErrSegmentOutOfRange = errors.New("segment out of range")

// TranscodeSession holds the state of one active multi-variant
// transcode. Each session produces one set of fMP4 variants ordered
// high-to-low (renditions[0] is the highest rung). Seek restart rewinds
// every variant together — hls.js switches variants independently but
// our ffmpeg always has every rung at the same input timestamp.
type TranscodeSession struct {
	hash       string
	dir        string // absolute path to .cache/transcode/{hash}
	srcPath    string // absolute resolved source path
	duration   float64
	renditions []media.Rendition
	hasAudio   bool

	highestRequested atomic.Int64

	mu           sync.Mutex
	startSegment int
	cancel       context.CancelFunc
	runDone      chan struct{} // closed when the current ffmpeg goroutine exits
	startedAt    time.Time
}

// Hash returns the content hash used as the session's cache key.
func (s *TranscodeSession) Hash() string { return s.hash }

// Dir returns the absolute path to the session's cache directory.
// Contains master.m3u8 and one stream_{N}/ subdirectory per variant
// (init.mp4, data{NNNNNN}.m4s, playlist.m3u8).
func (s *TranscodeSession) Dir() string { return s.dir }

// VariantCount returns the number of ABR variants in this session.
func (s *TranscodeSession) VariantCount() int { return len(s.renditions) }

// TranscodeService starts, caches, and serves multi-variant HLS fMP4
// transcodes. Sessions are keyed by a hash derived from absolute path
// + mtime + size + cap + encoder, so any source change, cap change, or
// encoder swap invalidates the cache naturally.
type TranscodeService struct {
	storage   *storage.LocalStorage
	probe     *ProbeService
	ffmpeg    *media.FFmpeg
	dataDir   string
	realRoot  string
	cacheRoot string

	encoder   media.Encoder
	maxHeight int

	mu       sync.Mutex
	sessions map[string]*TranscodeSession
	sema     chan struct{}

	initFlight sync.Map // hash → *transcodeInflight

	stopCh chan struct{}
	wg     sync.WaitGroup
}

type transcodeInflight struct {
	done    chan struct{}
	session *TranscodeSession
	err     error
}

// NewTranscodeService resolves dataDir to an absolute, symlink-free
// path, creates the cache root, selects the encoder from the startup
// hwaccel probe, and launches the periodic cache cleanup goroutine.
func NewTranscodeService(s *storage.LocalStorage, probe *ProbeService, dataDir, cacheDir string, hwProbe media.Probe, hwaccelPref string, maxHeight int) (*TranscodeService, error) {
	absDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return nil, fmt.Errorf("abs data dir: %w", err)
	}
	realRoot, err := filepath.EvalSymlinks(absDataDir)
	if err != nil {
		return nil, fmt.Errorf("resolve data dir: %w", err)
	}
	cacheRoot := filepath.Join(cacheDir, "transcode")
	if err := os.MkdirAll(cacheRoot, 0o755); err != nil {
		return nil, fmt.Errorf("create transcode cache: %w", err)
	}
	limit := runtime.NumCPU() / 2
	if limit < 1 {
		limit = 1
	}
	absCacheRoot, err := filepath.Abs(cacheRoot)
	if err != nil {
		return nil, fmt.Errorf("abs cache root: %w", err)
	}
	encoder := hwProbe.Select(hwaccelPref)
	slog.Info("transcode service: encoder selected",
		"encoder", encoder,
		"max_height", maxHeight,
	)
	ts := &TranscodeService{
		storage:   s,
		probe:     probe,
		ffmpeg:    media.Detect(),
		dataDir:   absDataDir,
		realRoot:  realRoot,
		cacheRoot: absCacheRoot,
		encoder:   encoder,
		maxHeight: maxHeight,
		sessions:  make(map[string]*TranscodeSession),
		sema:      make(chan struct{}, limit),
		stopCh:    make(chan struct{}),
	}
	ts.wg.Add(1)
	go ts.cleanupLoop()
	return ts, nil
}

// HasFFmpeg reports whether the ffmpeg binary is available.
func (ts *TranscodeService) HasFFmpeg() bool { return ts.ffmpeg.Available() }

// Encoder returns the encoder currently selected for transcoding.
func (ts *TranscodeService) Encoder() media.Encoder { return ts.encoder }

// Ensure resolves relPath, starts a session if one does not exist yet,
// and returns it after master and per-variant playlists are written.
// Concurrent callers for the same file coalesce via single-flight.
func (ts *TranscodeService) Ensure(ctx context.Context, relPath string) (*TranscodeSession, error) {
	if !ts.ffmpeg.Available() {
		return nil, fmt.Errorf("ffmpeg not available")
	}

	stat, err := ts.storage.Stat(relPath)
	if err != nil {
		return nil, err
	}
	if stat.IsDir {
		return nil, fmt.Errorf("not a file")
	}

	absPath, err := ts.resolveSafePath(relPath)
	if err != nil {
		return nil, err
	}

	info, err := ts.probe.Probe(ctx, relPath)
	if err != nil {
		return nil, fmt.Errorf("probe: %w", err)
	}
	if info.Duration <= 0 {
		return nil, fmt.Errorf("unknown duration")
	}
	if info.Height <= 0 {
		return nil, fmt.Errorf("unknown height")
	}

	rungs := media.SelectRungs(info.Height, ts.maxHeight)
	hash := sessionKey(absPath, stat.ModTime, stat.Size, ts.maxHeight, ts.encoder)

	ts.mu.Lock()
	if existing, ok := ts.sessions[hash]; ok {
		ts.mu.Unlock()
		return existing, nil
	}
	ts.mu.Unlock()

	entry := &transcodeInflight{done: make(chan struct{})}
	if existing, loaded := ts.initFlight.LoadOrStore(hash, entry); loaded {
		e := existing.(*transcodeInflight)
		select {
		case <-e.done:
			return e.session, e.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	defer func() {
		close(entry.done)
		ts.initFlight.Delete(hash)
	}()

	ts.mu.Lock()
	if existing, ok := ts.sessions[hash]; ok {
		ts.mu.Unlock()
		entry.session = existing
		return existing, nil
	}
	ts.mu.Unlock()

	session, err := ts.initSession(ctx, hash, absPath, relPath, info, rungs)
	entry.session = session
	entry.err = err
	if err != nil {
		return nil, err
	}

	slog.Info("transcode session started",
		"hash", hash,
		"path", relPath,
		"duration", info.Duration,
		"source_height", info.Height,
		"rungs", len(rungs),
		"encoder", ts.encoder,
	)
	return session, nil
}

// initSession creates directories, writes playlists, starts ffmpeg, and
// registers the session. Called only by the single-flight winner.
func (ts *TranscodeService) initSession(ctx context.Context, hash, absPath, relPath string, info *media.ProbeInfo, rungs []media.Rendition) (*TranscodeSession, error) {
	sessionDir := filepath.Join(ts.cacheRoot, hash)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}
	for v := range rungs {
		if err := os.MkdirAll(filepath.Join(sessionDir, media.VariantDir(v)), 0o755); err != nil {
			return nil, fmt.Errorf("create variant dir: %w", err)
		}
	}

	if err := writeMasterPlaylist(sessionDir, info, relPath, rungs); err != nil {
		return nil, fmt.Errorf("write master: %w", err)
	}
	for v, r := range rungs {
		if err := writeVariantPlaylist(sessionDir, info.Duration, relPath, v, r); err != nil {
			return nil, fmt.Errorf("write variant playlist: %w", err)
		}
	}

	session := &TranscodeSession{
		hash:       hash,
		dir:        sessionDir,
		srcPath:    absPath,
		duration:   info.Duration,
		renditions: rungs,
		hasAudio:   info.HasAudio,
	}

	session.mu.Lock()
	if err := ts.startFFmpegLocked(session, 0); err != nil {
		session.mu.Unlock()
		return nil, err
	}
	session.mu.Unlock()

	ts.mu.Lock()
	ts.sessions[hash] = session
	ts.mu.Unlock()

	return session, nil
}

// SessionDir returns the absolute cache directory for the given hash,
// or empty string if no session exists.
func (ts *TranscodeService) SessionDir(hash string) string {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if s, ok := ts.sessions[hash]; ok {
		return s.dir
	}
	return ""
}

// GetSegment returns the bytes of segment N of variant V for the given
// hash. Blocks up to segmentWaitTimeout waiting for the file to
// appear, triggering a seek restart first if N falls outside the
// transcode window currently in progress. Seek restart rewinds every
// variant together.
func (ts *TranscodeService) GetSegment(ctx context.Context, hash string, variant, segNum int) ([]byte, error) {
	ts.mu.Lock()
	session, ok := ts.sessions[hash]
	ts.mu.Unlock()
	if !ok {
		return nil, ErrSessionNotFound
	}
	if variant < 0 || variant >= len(session.renditions) {
		return nil, fmt.Errorf("variant %d out of range", variant)
	}
	maxSeg := int(math.Ceil(session.duration / float64(media.HLSSegmentSeconds)))
	if segNum >= maxSeg {
		return nil, ErrSegmentOutOfRange
	}

	if int64(segNum) > session.highestRequested.Load() {
		session.highestRequested.Store(int64(segNum))
	}

	segPath := filepath.Join(session.dir, media.HLSSegmentName(variant, segNum))
	if data, ok := readIfStable(segPath); ok {
		return data, nil
	}

	session.mu.Lock()
	needRestart := segNum < session.startSegment || segNum > session.startSegment+forwardSeekTolerance
	if needRestart {
		if err := ts.startFFmpegLocked(session, segNum); err != nil {
			session.mu.Unlock()
			return nil, fmt.Errorf("seek restart: %w", err)
		}
	}
	session.mu.Unlock()

	deadline := time.Now().Add(segmentWaitTimeout)
	ticker := time.NewTicker(segmentPollInterval)
	defer ticker.Stop()
	var lastSize int64 = -1
	for {
		if info, err := os.Stat(segPath); err == nil && info.Size() > 0 {
			if info.Size() == lastSize {
				if data, err := os.ReadFile(segPath); err == nil {
					return data, nil
				}
			}
			lastSize = info.Size()
		}
		if time.Now().After(deadline) {
			return nil, ErrSegmentTimeout
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

// readIfStable returns the file contents if it exists and its size is
// stable across two stat calls 100ms apart.
func readIfStable(path string) ([]byte, bool) {
	first, err := os.Stat(path)
	if err != nil || first.Size() == 0 {
		return nil, false
	}
	time.Sleep(100 * time.Millisecond)
	second, err := os.Stat(path)
	if err != nil || second.Size() != first.Size() {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return data, true
}

// Shutdown cancels the cleanup loop and every active session's ffmpeg,
// blocking until each has fully exited.
func (ts *TranscodeService) Shutdown() {
	close(ts.stopCh)
	ts.mu.Lock()
	sessions := make([]*TranscodeSession, 0, len(ts.sessions))
	for _, s := range ts.sessions {
		sessions = append(sessions, s)
	}
	ts.mu.Unlock()

	for _, s := range sessions {
		s.mu.Lock()
		cancel := s.cancel
		done := s.runDone
		s.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		if done != nil {
			<-done
		}
	}
	ts.wg.Wait()
}

// startFFmpegLocked kills the current ffmpeg if any, then starts a new
// one from fromSegment. Caller must hold session.mu.
func (ts *TranscodeService) startFFmpegLocked(s *TranscodeSession, fromSegment int) error {
	if s.cancel != nil {
		s.cancel()
	}
	if s.runDone != nil {
		<-s.runDone
	}

	select {
	case ts.sema <- struct{}{}:
	default:
		slog.Info("transcode: waiting for concurrency slot", "hash", s.hash)
		ts.sema <- struct{}{}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd, err := ts.ffmpeg.BuildHLSCommand(ctx, media.HLSOptions{
		SrcPath:      s.srcPath,
		OutDir:       s.dir,
		StartSegment: fromSegment,
		Encoder:      ts.encoder,
		Renditions:   s.renditions,
		HasAudio:     s.hasAudio,
	})
	if err != nil {
		cancel()
		<-ts.sema
		return err
	}

	stderrBuf := &boundedBuffer{max: 8192}
	cmd.Stderr = stderrBuf
	if err := cmd.Start(); err != nil {
		cancel()
		<-ts.sema
		return err
	}

	done := make(chan struct{})
	s.cancel = cancel
	s.runDone = done
	s.startSegment = fromSegment
	s.startedAt = time.Now()

	go ts.waitFFmpeg(cmd, s, done, fromSegment, stderrBuf)

	return nil
}

// waitFFmpeg reaps the process and releases the semaphore slot.
func (ts *TranscodeService) waitFFmpeg(cmd *exec.Cmd, s *TranscodeSession, done chan struct{}, fromSegment int, stderrBuf *boundedBuffer) {
	err := cmd.Wait()
	<-ts.sema
	close(done)
	if err != nil && !errors.Is(err, context.Canceled) {
		if exitErr, ok := err.(*exec.ExitError); ok {
			slog.Warn("ffmpeg exited with error",
				"hash", s.hash,
				"from_segment", fromSegment,
				"exit_code", exitErr.ExitCode(),
				"stderr", stderrBuf.String(),
			)
			return
		}
		slog.Warn("ffmpeg wait error", "hash", s.hash, "error", err, "stderr", stderrBuf.String())
		return
	}
	slog.Debug("ffmpeg finished", "hash", s.hash, "from_segment", fromSegment)
}

// boundedBuffer captures at most max bytes of ffmpeg stderr for
// diagnostic logging; further writes are silently dropped.
type boundedBuffer struct {
	mu  sync.Mutex
	buf []byte
	max int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	remaining := b.max - len(b.buf)
	if remaining <= 0 {
		return len(p), nil
	}
	if len(p) > remaining {
		b.buf = append(b.buf, p[:remaining]...)
	} else {
		b.buf = append(b.buf, p...)
	}
	return len(p), nil
}

func (b *boundedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.TrimRight(string(b.buf), "\r\n")
}

// resolveSafePath mirrors ProbeService.resolveSafePath — validates
// relPath via storage.Stat, cleans it, and returns the symlink-resolved
// absolute path confined to the data directory.
func (ts *TranscodeService) resolveSafePath(relPath string) (string, error) {
	if _, err := ts.storage.Stat(relPath); err != nil {
		return "", err
	}
	clean := strings.TrimLeft(relPath, "/")
	clean = path.Clean(clean)
	if clean == ".." || strings.HasPrefix(clean, "../") || clean == "." {
		return "", errors.New("invalid path")
	}
	osPath := filepath.Join(ts.dataDir, filepath.FromSlash(clean))
	resolved, err := filepath.EvalSymlinks(osPath)
	if err != nil {
		return "", err
	}
	root := ts.realRoot + string(filepath.Separator)
	if resolved != ts.realRoot && !strings.HasPrefix(resolved, root) {
		return "", errors.New("path escapes data directory")
	}
	return resolved, nil
}

// sessionKey is the cache key. Includes cap + encoder so a config
// change (lower ONYX_MAX_TRANSCODE_HEIGHT or different ONYX_HWACCEL)
// naturally invalidates existing cached transcodes; old entries age
// out via the cleanup loop.
func sessionKey(absPath string, mtime, size int64, cap int, encoder media.Encoder) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\n%d\n%d\n%d\n%s", absPath, mtime, size, cap, encoder)
	return hex.EncodeToString(h.Sum(nil))
}

// buildStreamURL joins an API prefix with relPath, URL-escaping each
// path segment so spaces and non-ASCII characters survive the round
// trip from m3u8 body back to the chi wildcard route.
func buildStreamURL(prefix, relPath string) string {
	segments := strings.Split(strings.TrimPrefix(relPath, "/"), "/")
	for i, seg := range segments {
		segments[i] = url.PathEscape(seg)
	}
	return prefix + "/" + strings.Join(segments, "/")
}

// writeMasterPlaylist writes a multi-variant master.m3u8 listing every
// rung. Each variant URI is an absolute server path to
// /api/stream/playlist/{v}/... so hls.js resolves it against the API
// host regardless of which master URL it was fetched from.
func writeMasterPlaylist(dir string, info *media.ProbeInfo, relPath string, rungs []media.Rendition) error {
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	b.WriteString("#EXT-X-VERSION:7\n")
	b.WriteString("#EXT-X-INDEPENDENT-SEGMENTS\n")
	codecs := "avc1.640028"
	if info.HasAudio {
		codecs = "avc1.640028,mp4a.40.2"
	}
	for i, r := range rungs {
		width := widthFor(info, r.Height)
		bandwidth := bitrateKbps(r.VBitrate) * 1000
		fmt.Fprintf(&b,
			"#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d,CODECS=\"%s\"\n",
			bandwidth, width, r.Height, codecs,
		)
		b.WriteString(buildStreamURL(fmt.Sprintf("/api/stream/playlist/%d", i), relPath))
		b.WriteString("\n")
	}
	return os.WriteFile(filepath.Join(dir, "master.m3u8"), []byte(b.String()), 0o644)
}

// writeVariantPlaylist writes stream_{v}/playlist.m3u8 — a full VOD
// playlist for variant v listing every segment up to the probed
// duration. Init and segment URIs are absolute server paths.
func writeVariantPlaylist(dir string, duration float64, relPath string, variant int, _ media.Rendition) error {
	variantDir := filepath.Join(dir, media.VariantDir(variant))
	if err := os.MkdirAll(variantDir, 0o755); err != nil {
		return err
	}
	segCount := int(math.Ceil(duration / float64(media.HLSSegmentSeconds)))
	initURL := buildStreamURL(fmt.Sprintf("/api/stream/init/%d", variant), relPath)
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	b.WriteString("#EXT-X-VERSION:7\n")
	b.WriteString(fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", media.HLSSegmentSeconds))
	b.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")
	b.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")
	b.WriteString(fmt.Sprintf("#EXT-X-MAP:URI=\"%s\"\n", initURL))
	for i := 0; i < segCount; i++ {
		segDuration := float64(media.HLSSegmentSeconds)
		if i == segCount-1 {
			remain := duration - float64(i*media.HLSSegmentSeconds)
			if remain > 0 && remain < segDuration {
				segDuration = remain
			}
		}
		b.WriteString(fmt.Sprintf("#EXTINF:%.3f,\n", segDuration))
		b.WriteString(buildStreamURL(fmt.Sprintf("/api/stream/segment/%d/%d", variant, i), relPath))
		b.WriteString("\n")
	}
	b.WriteString("#EXT-X-ENDLIST\n")
	return os.WriteFile(filepath.Join(variantDir, "playlist.m3u8"), []byte(b.String()), 0o644)
}

// widthFor returns an approximate target width for a scaled rendition,
// preserving the source's aspect ratio. Used for the master.m3u8
// RESOLUTION metadata; the actual encoded width is whatever ffmpeg's
// scale=-2:H produces at encode time, which may differ by a pixel on
// odd-pixel sources.
func widthFor(info *media.ProbeInfo, targetHeight int) int {
	if info.Height <= 0 || info.Width <= 0 {
		return 0
	}
	w := int(float64(info.Width) * float64(targetHeight) / float64(info.Height))
	if w%2 != 0 {
		w--
	}
	return w
}

// bitrateKbps parses strings like "5000k" or "18000k" into an integer
// kbps value. Returns 0 on malformed input.
func bitrateKbps(v string) int {
	v = strings.TrimSuffix(strings.TrimSpace(v), "k")
	if v == "" {
		return 0
	}
	n := 0
	for _, c := range v {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// cleanupLoop periodically sweeps the transcode cache, removing
// entries whose master.m3u8 mtime is older than cacheExpiry and that
// are not currently in use by an active session. Runs a sweep on
// startup to clear out abandoned cache dirs from crashed prior runs.
func (ts *TranscodeService) cleanupLoop() {
	defer ts.wg.Done()
	ts.sweepCache()
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ts.stopCh:
			return
		case <-ticker.C:
			ts.sweepCache()
		}
	}
}

// sweepCache snapshots active session hashes under the mutex, then
// walks the cache root and deletes every {hash} directory whose
// master.m3u8 is older than cacheExpiry and is not in the active set.
// Dirs without a master.m3u8 (a session that was mid-initialization)
// are skipped.
func (ts *TranscodeService) sweepCache() {
	ts.mu.Lock()
	active := make(map[string]struct{}, len(ts.sessions))
	for h := range ts.sessions {
		active[h] = struct{}{}
	}
	ts.mu.Unlock()

	entries, err := os.ReadDir(ts.cacheRoot)
	if err != nil {
		slog.Warn("transcode cleanup: read cache root failed", "error", err)
		return
	}
	threshold := time.Now().Add(-cacheExpiry)
	var removed int
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		hash := e.Name()
		if _, ok := active[hash]; ok {
			continue
		}
		dirPath := filepath.Join(ts.cacheRoot, hash)
		masterPath := filepath.Join(dirPath, "master.m3u8")
		info, err := os.Stat(masterPath)
		if err != nil {
			continue
		}
		if info.ModTime().After(threshold) {
			continue
		}
		if err := os.RemoveAll(dirPath); err != nil {
			slog.Warn("transcode cleanup: remove failed", "dir", dirPath, "error", err)
			continue
		}
		removed++
	}
	if removed > 0 {
		slog.Info("transcode cleanup: swept stale cache entries", "count", removed)
	}
}
