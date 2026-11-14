package service

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// SetCacheLimit bounds the transcode cache in bytes. 0 means no limit.
func (ts *TranscodeService) SetCacheLimit(bytes int64) {
	ts.cacheLimit.Store(bytes)
}

// dropIdleSessions unregisters sessions nobody has fetched a segment from in idleDropAfter, then deletes their cache
func (ts *TranscodeService) dropIdleSessions() {
	cutoff := time.Now().Add(-idleDropAfter)
	ts.mu.Lock()
	var idle []*TranscodeSession
	for h, s := range ts.sessions {
		if s.lastUsed.Before(cutoff) {
			delete(ts.sessions, h)
			idle = append(idle, s)
		}
	}
	ts.mu.Unlock()
	for _, s := range idle {
		ts.dropSession(s, "idle session dropped")
	}
}

// cacheVictim is one cache dir the size limit may delete; rank orders who goes first
type cacheVictim struct {
	hash    string
	session *TranscodeSession // nil for a dir no session owns
	rank    int
	at      time.Time
	owner   string // visitor key for a visitor's session
}

const (
	rankOrphan = iota
	rankPublic
	rankAdminIdle
)

// enforceCacheLimit deletes unowned dirs, then visitor sessions, then admin idle ones until under the limit
func (ts *TranscodeService) enforceCacheLimit() {
	limit := ts.cacheLimit.Load()
	if limit <= 0 {
		return
	}
	sizes, total := ts.cacheDirSizes()
	if total <= limit {
		return
	}

	now := time.Now()
	var victims []cacheVictim
	ts.mu.Lock()
	for hash := range sizes {
		s, ok := ts.sessions[hash]
		switch {
		case !ok:
			at := dirAge(filepath.Join(ts.cacheRoot, hash))
			if now.Sub(at) >= youngCacheDir {
				victims = append(victims, cacheVictim{hash: hash, rank: rankOrphan, at: at})
			}
		case s.public:
			victims = append(victims, cacheVictim{hash: hash, session: s, rank: rankPublic, at: s.lastUsed, owner: s.owner})
		case now.Sub(s.lastUsed) >= idleStopAfter:
			victims = append(victims, cacheVictim{hash: hash, session: s, rank: rankAdminIdle, at: s.lastUsed})
		}
	}
	ts.mu.Unlock()
	sort.Slice(victims, func(i, j int) bool {
		if victims[i].rank != victims[j].rank {
			return victims[i].rank < victims[j].rank
		}
		return victims[i].at.Before(victims[j].at)
	})
	victims = fairPublicOrder(victims, sizes)

	for _, v := range victims {
		if total <= limit {
			break
		}
		if v.session == nil {
			if err := os.RemoveAll(filepath.Join(ts.cacheRoot, v.hash)); err != nil {
				slog.Warn("transcode cache limit: remove failed", "hash", v.hash, "error", err)
				continue
			}
		} else {
			// Used or handed to the admin since the snapshot? Leave it.
			ts.mu.Lock()
			still := ts.sessions[v.hash] == v.session && (v.session.public || (v.rank == rankAdminIdle && now.Sub(v.session.lastUsed) >= idleStopAfter))
			if still {
				delete(ts.sessions, v.hash)
			}
			ts.mu.Unlock()
			if !still {
				continue
			}
			ts.dropSession(v.session, "session dropped for the cache size limit")
		}
		total -= sizes[v.hash]
	}
	if total > limit {
		slog.Warn("transcode cache is over its limit with only the admin's active sessions left", "bytes", total, "limit", limit)
	}
}

// cacheDirSizes sums the files under each session dir in the cache root
func (ts *TranscodeService) cacheDirSizes() (map[string]int64, int64) {
	sizes := make(map[string]int64)
	var total int64
	entries, err := os.ReadDir(ts.cacheRoot)
	if err != nil {
		slog.Warn("transcode cache limit: read cache root failed", "error", err)
		return sizes, 0
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		var n int64
		filepath.WalkDir(filepath.Join(ts.cacheRoot, e.Name()), func(_ string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if info, err := d.Info(); err == nil {
				n += info.Size()
			}
			return nil
		})
		sizes[e.Name()] = n
		total += n
	}
	return sizes, total
}

// dirAge is when a cache dir was last written: its master playlist, else the dir itself
func dirAge(dir string) time.Time {
	if info, err := os.Stat(filepath.Join(dir, "master.m3u8")); err == nil {
		return info.ModTime()
	}
	if info, err := os.Stat(dir); err == nil {
		return info.ModTime()
	}
	return time.Time{}
}

// publicStartRefusedLocked reports whether a visitor's new run on s would be refused. Caller holds s.mu.
func (ts *TranscodeService) publicStartRefusedLocked(s *TranscodeSession, key string) bool {
	running := false
	if s.runDone != nil {
		select {
		case <-s.runDone:
		default:
			running = true
		}
	}
	// Stopping a visitor's run gives its slot back, and the count back too if it is this visitor's
	freesPublic := running && s.runClient.Public
	ts.mu.Lock()
	runs := ts.clientRuns[key]
	ts.mu.Unlock()
	if freesPublic && s.runClient.Key == key {
		runs--
	}
	if runs >= maxClientTranscodes {
		return true
	}
	// Nothing to stop? Waiting for a slot then costs no one an encode.
	if !running || freesPublic {
		return false
	}
	return len(ts.publicSema) >= cap(ts.publicSema)
}

// fairPublicOrder reorders sorted victims so the visitor holding the most cache loses a session first
func fairPublicOrder(victims []cacheVictim, sizes map[string]int64) []cacheVictim {
	out := make([]cacheVictim, 0, len(victims))
	var public []cacheVictim
	held := make(map[string]int64)
	for _, v := range victims {
		switch v.rank {
		case rankOrphan:
			out = append(out, v)
		case rankPublic:
			public = append(public, v)
			held[v.owner] += sizes[v.hash]
		}
	}
	for len(public) > 0 {
		// Ties keep the earlier entry, which is the older session
		best := 0
		for i := 1; i < len(public); i++ {
			if held[public[i].owner] > held[public[best].owner] {
				best = i
			}
		}
		v := public[best]
		out = append(out, v)
		held[v.owner] -= sizes[v.hash]
		public = append(public[:best], public[best+1:]...)
	}
	for _, v := range victims {
		if v.rank == rankAdminIdle {
			out = append(out, v)
		}
	}
	return out
}
