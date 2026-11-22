package streaming

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"goonhub/internal/config"
	"goonhub/internal/lifecycle"

	"go.uber.org/zap"
)

// TranscodeCache keeps the fragmented MP4 written by the on-the-fly conversions, so that a
// replay, a seek back into a part already converted or a second viewer of the same part is
// served from disk instead of running ffmpeg again.
//
// Each ffmpeg run writes one entry: the output of one scene, with one conversion plan, from
// one start position, to the end of the scene (complete) or to where it was stopped
// (partial). Its fragments (one per video keyframe) are indexed by time, so any position
// inside an entry can be served: the stream sent to the player is the init segment followed
// by the fragments from the one containing the position, with their decode times shifted so
// that it starts at 0, as a fresh conversion would. When a partial entry ends, the stream
// continues with an entry (existing or started then) beginning at the same keyframe.
//
// The cache lives in its own directory (GoonHub data, never a media folder): it only reads
// and deletes files it wrote (marker file and strict file names).
type TranscodeCache struct {
	dir             string
	maxSize         int64
	maxAge          time.Duration
	cleanupInterval time.Duration
	readAhead       float64
	idleTimeout     time.Duration
	logger          *zap.Logger
	lifecycle       *lifecycle.Manager
	now             func() time.Time

	mu      sync.Mutex
	entries map[string]*cacheEntry
	total   int64
	stopped bool

	baseCtx    context.Context
	cancelBase context.CancelFunc
	stopLoop   chan struct{}
	startOnce  sync.Once
	stopOnce   sync.Once
}

// Marker file telling that a directory is a transcode cache (nothing else is ever deleted).
const transcodeCacheMarker = ".goonhub-transcode-cache"

// Cache entry files: <scene>-<variant>-<unique>.mp4 and its .json metadata.
var cacheFilePattern = regexp.MustCompile(`^([0-9]+-[0-9a-f]{16}-[0-9]+)\.(mp4|json)$`)

// Bumped when the cached output changes in a way that makes older entries unusable.
const transcodeCacheFormat = "1"

const (
	// Positions in the cache are compared with this slack (times are sent with 3 decimals).
	startEpsilon = 0.002
	// A keyframe of one run matches a keyframe of another run within this tolerance.
	joinTolerance = 0.25
	// A viewer asking for a position up to this far after what a running conversion has
	// produced waits for it instead of starting another ffmpeg.
	pendingWindow = 10.0
	// How far the times of a stream copy may be from the real ones (a copy starts on a
	// keyframe up to one keyframe interval before its -ss).
	keyDrift = 15.0
	// Shutdown wait of the conversions writing to the cache.
	cacheShutdownWait = 10 * time.Second
)

type cacheState int

const (
	cacheProducing cacheState = iota
	cacheComplete
	cachePartial
	cacheFailed
)

func (s cacheState) String() string {
	switch s {
	case cacheProducing:
		return "producing"
	case cacheComplete:
		return "complete"
	case cachePartial:
		return "partial"
	default:
		return "failed"
	}
}

// cacheMeta is the persisted description of an entry (its .json file).
type cacheMeta struct {
	Format   string    `json:"format"`
	SceneID  uint      `json:"scene_id"`
	Variant  string    `json:"variant"`
	PlanKey  string    `json:"plan_key"`
	Mode     string    `json:"mode"`
	FileSize int64     `json:"file_size"`
	FileMod  int64     `json:"file_mod"`
	Start    float64   `json:"start"`
	Complete bool      `json:"complete"`
	Created  time.Time `json:"created_at"`
}

// cacheFragment is one moof + mdat of an entry.
type cacheFragment struct {
	offset   int64
	size     int64
	moofSize int
	// decode time (seconds, entry output timeline) of the video track and the smallest one
	// of all tracks
	video float64
	min   float64
	// hash of the start of the first video sample: a keyframe copied by two runs has the
	// same (0: unknown)
	key uint64
}

// cacheEntry is the output of one ffmpeg run. Guarded by TranscodeCache.mu.
type cacheEntry struct {
	id       string
	meta     cacheMeta
	path     string
	init     []byte
	tracks   []mp4Track
	frags    []cacheFragment
	size     int64
	state    cacheState
	producer *cacheProducer
	readers  map[*CachedStream]float64
	lastUsed time.Time
	// removed from the cache: not offered to new viewers, deleted once released
	doomed  bool
	changed chan struct{}
}

func (e *cacheEntry) metaPath() string { return strings.TrimSuffix(e.path, ".mp4") + ".json" }

func (e *cacheEntry) pinned() bool { return len(e.readers) > 0 || e.producer != nil }

// broadcast wakes the viewers waiting for the entry to change.
func (e *cacheEntry) broadcast() {
	close(e.changed)
	e.changed = make(chan struct{})
}

func (e *cacheEntry) srcMin(i int) float64   { return e.meta.Start + e.frags[i].min }
func (e *cacheEntry) srcVideo(i int) float64 { return e.meta.Start + e.frags[i].video }

// usable is the number of fragments that can be served. The last fragment of a partial
// entry is kept only as the place where the next entry must start (its end is unknown).
func (e *cacheEntry) usable() int {
	if e.state == cachePartial && len(e.frags) > 0 {
		return len(e.frags) - 1
	}
	return len(e.frags)
}

// lastSrc is the source position reached by the entry.
func (e *cacheEntry) lastSrc() float64 {
	if len(e.frags) == 0 {
		return e.meta.Start
	}
	return e.srcMin(len(e.frags) - 1)
}

func (e *cacheEntry) maxReaderPos() float64 {
	pos := math.Inf(-1)
	for _, p := range e.readers {
		pos = math.Max(pos, p)
	}
	return pos
}

type locateResult int

const (
	locateMiss locateResult = iota
	locateHit
	// a running conversion of the entry will soon produce the position
	locatePending
)

// locate finds the fragment containing source position r.
func (e *cacheEntry) locate(r float64) (int, locateResult) {
	n := len(e.frags)
	next := n
	for i := 0; i < n; i++ {
		if e.srcMin(i) > r+startEpsilon {
			next = i
			break
		}
	}
	idx := next - 1
	if idx >= 0 && idx < e.usable() {
		if e.srcMin(idx) >= r-startEpsilon || next < n || e.state == cacheComplete {
			return idx, locateHit
		}
	}
	// Nothing produced after r yet: the running conversion will get there
	if e.state == cacheProducing && next == n &&
		r >= e.meta.Start-startEpsilon && r <= e.lastSrc()+pendingWindow {
		return -1, locatePending
	}
	return -1, locateMiss
}

// locateJoin finds the fragment starting at the video keyframe at source position at, where
// a previous entry stopped. Stream copies (key not 0) are matched on the keyframe itself:
// a copy can start a whole keyframe interval before its -ss, so its times are only
// approximate. With acceptGap (an entry started for this join), the first fragment after
// the position is accepted when no keyframe matches.
func (e *cacheEntry) locateJoin(at float64, key uint64, acceptGap bool) (int, locateResult) {
	if key != 0 {
		// The closest keyframe with that key (identical keyframes exist, e.g. black ones)
		best := -1
		for i := 0; i < e.usable(); i++ {
			d := math.Abs(e.srcVideo(i) - at)
			if e.frags[i].key == key && d <= keyDrift && (best < 0 || d < math.Abs(e.srcVideo(best)-at)) {
				best = i
			}
		}
		if best >= 0 {
			return best, locateHit
		}
		if e.state == cacheProducing && at >= e.meta.Start-keyDrift && e.lastSrc() <= at+keyDrift {
			return -1, locatePending
		}
		if !acceptGap {
			return -1, locateMiss
		}
	}
	for i := 0; i < e.usable(); i++ {
		src := e.srcVideo(i)
		if math.Abs(src-at) <= joinTolerance {
			return i, locateHit
		}
		if src > at+joinTolerance {
			if acceptGap {
				return i, locateHit
			}
			return -1, locateMiss
		}
	}
	if e.state == cacheProducing && at >= e.meta.Start-joinTolerance && at <= e.lastSrc()+pendingWindow {
		return -1, locatePending
	}
	return -1, locateMiss
}

// TranscodeCacheStats is what the admin page shows.
type TranscodeCacheStats struct {
	Enabled      bool  `json:"enabled"`
	SizeBytes    int64 `json:"size_bytes"`
	Entries      int   `json:"entries"`
	InProgress   int   `json:"in_progress"`
	MaxSizeBytes int64 `json:"max_size_bytes"`
	MaxAgeSecs   int64 `json:"max_age_seconds"`
}

// NewTranscodeCache opens (or creates) the cache directory. Start indexes the entries left
// by a previous run. Returns nil when the cache is disabled.
func NewTranscodeCache(cfg config.TranscodeCacheConfig, logger *zap.Logger) (*TranscodeCache, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if strings.TrimSpace(cfg.Dir) == "" {
		return nil, errors.New("transcode cache directory is not set")
	}
	dir, err := filepath.Abs(cfg.Dir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve the transcode cache directory: %w", err)
	}
	if err := prepareCacheDir(dir); err != nil {
		return nil, err
	}

	maxSize := int64(cfg.MaxSizeGB * (1 << 30))
	if maxSize <= 0 {
		maxSize = 20 << 30
	}
	maxAge := cfg.MaxAge
	if maxAge <= 0 {
		maxAge = 24 * time.Hour
	}
	interval := cfg.CleanupInterval
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	readAhead := cfg.ReadAhead
	if readAhead <= 0 {
		readAhead = 60 * time.Second
	}
	idle := cfg.IdleTimeout
	if idle <= 0 {
		idle = 30 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())
	c := &TranscodeCache{
		dir:             dir,
		maxSize:         maxSize,
		maxAge:          maxAge,
		cleanupInterval: interval,
		readAhead:       readAhead.Seconds(),
		idleTimeout:     idle,
		logger:          logger,
		lifecycle:       lifecycle.NewManager(logger),
		now:             time.Now,
		entries:         make(map[string]*cacheEntry),
		baseCtx:         ctx,
		cancelBase:      cancel,
		stopLoop:        make(chan struct{}),
	}
	return c, nil
}

// prepareCacheDir creates the directory with its marker. An existing directory that is not
// empty must already be a transcode cache: the cache deletes files, so it refuses to run
// in a directory holding anything else (a media folder given by mistake).
func prepareCacheDir(dir string) error {
	marker := filepath.Join(dir, transcodeCacheMarker)
	if _, err := os.Stat(marker); err == nil {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create the transcode cache directory: %w", err)
	}
	items, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read the transcode cache directory: %w", err)
	}
	if len(items) > 0 {
		return fmt.Errorf("transcode cache directory %s is not empty and is not a transcode cache: refusing to use it", dir)
	}
	if err := os.WriteFile(marker, []byte("GoonHub transcode cache: every file here may be deleted.\n"), 0o644); err != nil {
		return fmt.Errorf("failed to create the transcode cache marker: %w", err)
	}
	return nil
}

// load indexes the entries of the directory; unfinished ones (no metadata: GoonHub
// stopped while writing them) and unreadable ones are deleted. Entries become usable as
// they are indexed.
func (c *TranscodeCache) load() {
	items, err := os.ReadDir(c.dir)
	if err != nil {
		c.logger.Warn("Failed to read the transcode cache directory", zap.Error(err))
		return
	}
	files := map[string]map[string]bool{}
	for _, item := range items {
		m := cacheFilePattern.FindStringSubmatch(item.Name())
		if m == nil || !item.Type().IsRegular() {
			continue
		}
		if files[m[1]] == nil {
			files[m[1]] = map[string]bool{}
		}
		files[m[1]][m[2]] = true
	}
	kept, dropped := 0, 0
	for id, exts := range files {
		mp4Path := filepath.Join(c.dir, id+".mp4")
		metaPath := filepath.Join(c.dir, id+".json")
		entry, err := c.loadEntry(id, exts["mp4"], exts["json"])
		if err != nil {
			c.logger.Debug("Dropping a transcode cache entry", zap.String("entry", id), zap.Error(err))
			removeCacheFile(c.logger, mp4Path)
			removeCacheFile(c.logger, metaPath)
			dropped++
			continue
		}
		c.mu.Lock()
		c.entries[id] = entry
		c.total += entry.size
		c.mu.Unlock()
		kept++
	}
	if kept > 0 || dropped > 0 {
		c.mu.Lock()
		total := c.total
		c.mu.Unlock()
		c.logger.Info("Transcode cache loaded",
			zap.String("dir", c.dir),
			zap.Int("entries", kept),
			zap.Int("dropped", dropped),
			zap.Int64("bytes", total),
		)
	}
}

func (c *TranscodeCache) loadEntry(id string, hasMP4, hasMeta bool) (*cacheEntry, error) {
	if !hasMP4 || !hasMeta {
		return nil, errors.New("unfinished entry")
	}
	path := filepath.Join(c.dir, id+".mp4")
	raw, err := os.ReadFile(strings.TrimSuffix(path, ".mp4") + ".json")
	if err != nil {
		return nil, fmt.Errorf("failed to read the metadata: %w", err)
	}
	var meta cacheMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, fmt.Errorf("invalid metadata: %w", err)
	}
	if meta.Format != transcodeCacheFormat {
		return nil, errors.New("old cache format")
	}
	info, err := os.Stat(strings.TrimSuffix(path, ".mp4") + ".json")
	if err != nil {
		return nil, fmt.Errorf("failed to stat the metadata: %w", err)
	}
	init, tracks, frags, size, err := indexMP4File(path)
	if err != nil {
		return nil, err
	}
	if len(frags) == 0 || (!meta.Complete && len(frags) < 2) {
		return nil, errors.New("entry without usable fragment")
	}
	state := cachePartial
	if meta.Complete {
		state = cacheComplete
	}
	return &cacheEntry{
		id:       id,
		meta:     meta,
		path:     path,
		init:     init,
		tracks:   tracks,
		frags:    frags,
		size:     size,
		state:    state,
		readers:  map[*CachedStream]float64{},
		lastUsed: info.ModTime(),
		changed:  make(chan struct{}),
	}, nil
}

// indexMP4File reads the init segment and the fragment index of a cached file. Bytes after
// the last complete fragment are ignored.
func indexMP4File(path string) ([]byte, []mp4Track, []cacheFragment, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, nil, 0, fmt.Errorf("failed to open: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, nil, nil, 0, fmt.Errorf("failed to stat: %w", err)
	}
	fileSize := info.Size()

	var (
		init    []byte
		tracks  []mp4Track
		frags   []cacheFragment
		pending *cacheFragment
		pos     int64
		end     int64
	)
	hdr := make([]byte, 16)
	for pos+8 <= fileSize {
		n, err := f.ReadAt(hdr, pos)
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, nil, nil, 0, fmt.Errorf("failed to read: %w", err)
		}
		typ, size, _, ok := mp4BoxHeader(hdr[:n])
		if !ok || size == 0 || int64(size) > fileSize-pos {
			break
		}
		switch typ {
		case "ftyp", "moov", "moof":
			if size > maxMP4HeaderBox {
				return nil, nil, nil, 0, fmt.Errorf("%w: box too large", errMP4Format)
			}
			box := make([]byte, size)
			if _, err := f.ReadAt(box, pos); err != nil {
				return nil, nil, nil, 0, fmt.Errorf("failed to read: %w", err)
			}
			switch typ {
			case "ftyp":
				init = append(init, box...)
			case "moov":
				_, _, hl, _ := mp4BoxHeader(box)
				if tracks, err = parseMP4Tracks(box[hl:]); err != nil {
					return nil, nil, nil, 0, err
				}
				init = append(init, box...)
				end = pos + int64(size)
			case "moof":
				if tracks == nil {
					return nil, nil, nil, 0, fmt.Errorf("%w: moof before moov", errMP4Format)
				}
				frag, err := fragmentFromMoof(box, tracks)
				if err != nil {
					return nil, nil, nil, 0, err
				}
				frag.offset = pos
				if off, n, ok := firstSample(box, videoTrackID(tracks)); ok {
					headEnd, tailStart := sampleKeyRanges(n)
					head := make([]byte, headEnd)
					tail := make([]byte, int64(n)-tailStart)
					_, errHead := f.ReadAt(head, pos+off)
					_, errTail := f.ReadAt(tail, pos+off+tailStart)
					if errHead == nil && errTail == nil {
						frag.key = sampleKey(n, head, tail)
					}
				}
				pending = &frag
			}
		case "mdat":
			if pending != nil {
				pending.size = pos + int64(size) - pending.offset
				frags = append(frags, *pending)
				pending = nil
				end = pos + int64(size)
			}
		}
		pos += int64(size)
	}
	if tracks == nil {
		return nil, nil, nil, 0, fmt.Errorf("%w: no init segment", errMP4Format)
	}
	return init, tracks, frags, end, nil
}

// videoTrackID is the id of the first video track (0: none).
func videoTrackID(tracks []mp4Track) uint32 {
	for _, tr := range tracks {
		if tr.Video {
			return tr.ID
		}
	}
	return 0
}

// fragmentFromMoof reads the times of a fragment.
func fragmentFromMoof(moof []byte, tracks []mp4Track) (cacheFragment, error) {
	times, err := parseMoof(moof)
	if err != nil {
		return cacheFragment{}, err
	}
	frag := cacheFragment{moofSize: len(moof), video: math.NaN(), min: math.Inf(1)}
	for _, dt := range times {
		for _, tr := range tracks {
			if tr.ID != dt.TrackID {
				continue
			}
			sec := float64(dt.Time) / float64(tr.Timescale)
			frag.min = math.Min(frag.min, sec)
			if tr.Video && math.IsNaN(frag.video) {
				frag.video = sec
			}
		}
	}
	if math.IsInf(frag.min, 1) {
		return cacheFragment{}, fmt.Errorf("%w: fragment of unknown tracks", errMP4Format)
	}
	if math.IsNaN(frag.video) {
		frag.video = frag.min
	}
	return frag, nil
}

// Start indexes the entries of a previous run, then runs the periodic cleanup (also once
// right away). Indexing reads the cache files: it runs in the background so that a large
// cache on a slow disk does not delay the startup.
func (c *TranscodeCache) Start() {
	c.startOnce.Do(func() {
		c.lifecycle.Go("transcode-cache-cleanup", func(done <-chan struct{}) {
			ticker := time.NewTicker(c.cleanupInterval)
			defer ticker.Stop()
			c.load()
			c.Cleanup()
			for {
				select {
				case <-done:
					return
				case <-c.stopLoop:
					return
				case <-ticker.C:
					c.Cleanup()
				}
			}
		})
	})
}

// Stop kills the running conversions and waits for them to finish writing their entry.
func (c *TranscodeCache) Stop() {
	c.stopOnce.Do(func() {
		c.mu.Lock()
		c.stopped = true
		c.mu.Unlock()
		close(c.stopLoop)
		c.cancelBase()
		if err := c.lifecycle.Shutdown(cacheShutdownWait); err != nil {
			c.logger.Warn("Transcode cache conversions did not stop in time", zap.Error(err))
		}
	})
}

// Cleanup applies the age and size limits.
func (c *TranscodeCache) Cleanup() {
	c.mu.Lock()
	paths := c.enforceLocked()
	c.mu.Unlock()
	removeCacheFiles(c.logger, paths)
}

// enforceLocked removes the entries unused for longer than the max age, then the least
// recently used ones while the cache is over its size. Entries being written or read are
// never removed. Returns the files to delete.
func (c *TranscodeCache) enforceLocked() []string {
	var paths []string
	cutoff := c.now().Add(-c.maxAge)
	var candidates []*cacheEntry
	for _, e := range c.entries {
		if e.pinned() {
			continue
		}
		if e.doomed || e.lastUsed.Before(cutoff) {
			paths = append(paths, c.removeLocked(e)...)
			continue
		}
		candidates = append(candidates, e)
	}
	for c.total > c.maxSize && len(candidates) > 0 {
		oldest := 0
		for i, e := range candidates {
			if e.lastUsed.Before(candidates[oldest].lastUsed) {
				oldest = i
			}
		}
		paths = append(paths, c.removeLocked(candidates[oldest])...)
		candidates = append(candidates[:oldest], candidates[oldest+1:]...)
	}
	return paths
}

// removeLocked drops an entry that nobody uses and returns its files.
func (c *TranscodeCache) removeLocked(e *cacheEntry) []string {
	if _, ok := c.entries[e.id]; !ok {
		return nil
	}
	delete(c.entries, e.id)
	c.total -= e.size
	e.doomed = true
	return []string{e.path, e.metaPath()}
}

// doomLocked removes an entry now if nobody uses it, or once released.
func (c *TranscodeCache) doomLocked(e *cacheEntry) []string {
	if e.pinned() {
		e.doomed = true
		return nil
	}
	return c.removeLocked(e)
}

// Clear removes every entry (those in use are removed as soon as they are released).
// Returns the number of entries and bytes removed or scheduled for removal.
func (c *TranscodeCache) Clear() (int, int64) {
	c.mu.Lock()
	count, size := 0, int64(0)
	var paths []string
	for _, e := range c.entries {
		if e.doomed {
			continue
		}
		count++
		size += e.size
		paths = append(paths, c.doomLocked(e)...)
	}
	c.mu.Unlock()
	removeCacheFiles(c.logger, paths)
	c.logger.Info("Transcode cache cleared", zap.Int("entries", count), zap.Int64("bytes", size))
	return count, size
}

// Stats returns the cache size.
func (c *TranscodeCache) Stats() TranscodeCacheStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	stats := TranscodeCacheStats{
		Enabled:      true,
		SizeBytes:    c.total,
		MaxSizeBytes: c.maxSize,
		MaxAgeSecs:   int64(c.maxAge.Seconds()),
	}
	for _, e := range c.entries {
		if e.doomed {
			continue
		}
		stats.Entries++
		if e.state == cacheProducing {
			stats.InProgress++
		}
	}
	return stats
}

// hasIdleProducer tells whether a conversion nobody watches could be stopped for a slot.
func (c *TranscodeCache) hasIdleProducer() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, e := range c.entries {
		if e.producer != nil && len(e.readers) == 0 {
			return true
		}
	}
	return false
}

// cacheKey identifies the cached output of a scene file converted with a plan.
type cacheKey struct {
	sceneID  uint
	fileSize int64
	fileMod  int64
	planKey  string
	variant  string
	mode     string
}

func newCacheKey(src TranscodeSource, planArgs []string) cacheKey {
	planSum := sha256.Sum256([]byte(transcodeCacheFormat + "\x00" + strings.Join(planArgs, "\x00")))
	planKey := hex.EncodeToString(planSum[:])[:16]
	variantSum := sha256.Sum256([]byte(strings.Join([]string{
		planKey,
		strconv.FormatUint(uint64(src.SceneID), 10),
		strconv.FormatInt(src.Size, 10),
		strconv.FormatInt(src.ModTime.UnixNano(), 10),
	}, "\x00")))
	return cacheKey{
		sceneID:  src.SceneID,
		fileSize: src.Size,
		fileMod:  src.ModTime.UnixNano(),
		planKey:  planKey,
		variant:  hex.EncodeToString(variantSum[:])[:16],
		mode:     string(src.Plan.Mode),
	}
}

// invalidateLocked removes the entries of the scene made from another version of its file,
// or with other settings for the same kind of conversion.
func (c *TranscodeCache) invalidateLocked(key cacheKey) []string {
	var paths []string
	for _, e := range c.entries {
		if e.doomed || e.meta.SceneID != key.sceneID {
			continue
		}
		fileChanged := e.meta.FileSize != key.fileSize || e.meta.FileMod != key.fileMod
		settingsChanged := e.meta.Mode == key.mode && e.meta.PlanKey != key.planKey
		if fileChanged || settingsChanged {
			c.logger.Debug("Invalidating a transcode cache entry",
				zap.String("entry", e.id), zap.Bool("file_changed", fileChanged))
			paths = append(paths, c.doomLocked(e)...)
		}
	}
	return paths
}

// newEntryLocked creates the file of a new entry.
func (c *TranscodeCache) newEntryLocked(key cacheKey, start float64) (*cacheEntry, *os.File, error) {
	now := c.now()
	id := fmt.Sprintf("%d-%s-%d", key.sceneID, key.variant, now.UnixNano())
	for c.entries[id] != nil {
		now = now.Add(time.Nanosecond)
		id = fmt.Sprintf("%d-%s-%d", key.sceneID, key.variant, now.UnixNano())
	}
	path := filepath.Join(c.dir, id+".mp4")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create the cache file: %w", err)
	}
	e := &cacheEntry{
		id: id,
		meta: cacheMeta{
			Format:   transcodeCacheFormat,
			SceneID:  key.sceneID,
			Variant:  key.variant,
			PlanKey:  key.planKey,
			Mode:     key.mode,
			FileSize: key.fileSize,
			FileMod:  key.fileMod,
			Start:    start,
			Created:  now,
		},
		path:     path,
		state:    cacheProducing,
		readers:  map[*CachedStream]float64{},
		lastUsed: now,
		changed:  make(chan struct{}),
	}
	c.entries[id] = e
	return e, f, nil
}

// writeMeta saves the metadata of a finished entry (written last: an entry without it is
// an unfinished one and is dropped at startup).
func writeCacheMeta(e *cacheEntry, meta cacheMeta) error {
	raw, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to encode the cache metadata: %w", err)
	}
	tmp := e.metaPath() + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("failed to write the cache metadata: %w", err)
	}
	if err := os.Rename(tmp, e.metaPath()); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("failed to save the cache metadata: %w", err)
	}
	return nil
}

func removeCacheFiles(logger *zap.Logger, paths []string) {
	for _, p := range paths {
		removeCacheFile(logger, p)
	}
}

// removeCacheFile deletes a file of the cache (only names the cache creates).
func removeCacheFile(logger *zap.Logger, path string) {
	if !cacheFilePattern.MatchString(filepath.Base(path)) {
		logger.Warn("Refusing to delete a file that is not a transcode cache entry", zap.String("path", path))
		return
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		logger.Warn("Failed to delete a transcode cache file", zap.String("path", path), zap.Error(err))
	}
}
