package streaming

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"time"

	"go.uber.org/zap"
)

// ErrCachedStreamBroken is returned when a cached stream can't be continued (the
// conversion failed, or the next part could not be produced).
var ErrCachedStreamBroken = errors.New("the conversion could not be continued")

// TranscodeSource is the scene file to convert and how.
type TranscodeSource struct {
	SceneID uint
	Path    string
	// size and modification time of the file: a change invalidates its cached conversions
	Size    int64
	ModTime time.Time
	Plan    PlaybackPlan
}

// SetCache makes the transcoder keep its conversions in c (nil: no cache).
func (t *Transcoder) SetCache(c *TranscodeCache) { t.cache = c }

// CacheEnabled tells whether conversions go through the cache.
func (t *Transcoder) CacheEnabled() bool { return t.cache != nil }

// CacheStats returns the cache size (Enabled false without cache).
func (t *Transcoder) CacheStats() TranscodeCacheStats {
	if t.cache == nil {
		return TranscodeCacheStats{}
	}
	return t.cache.Stats()
}

// ClearCache removes every cached conversion. Returns the entries and bytes removed.
func (t *Transcoder) ClearCache() (int, int64) {
	if t.cache == nil {
		return 0, 0
	}
	return t.cache.Clear()
}

func (t *Transcoder) tryAcquireSlot() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.stopped || t.active >= t.settings.MaxConcurrent {
		return false
	}
	t.active++
	return true
}

func (t *Transcoder) releaseSlot() {
	t.mu.Lock()
	t.active--
	t.mu.Unlock()
}

func (t *Transcoder) cacheKey(src TranscodeSource) cacheKey {
	return newCacheKey(src, BuildTranscodeArgs("", src.Plan, 0, t.settings))
}

// AlignStart is where a new conversion of a cached stream starts for a requested
// position: re-encodes start on the keyframe grid (every keyframe_interval seconds of the
// scene), so that the fragments of all the conversions of a scene line up and can follow
// each other. A remux starts on the keyframe of the file (see KeyframeStart).
func (t *Transcoder) AlignStart(plan PlaybackPlan, start float64) float64 {
	if t.cache == nil || plan.CopyVideo || start <= 0 {
		return start
	}
	k := float64(t.settings.KeyframeInterval)
	return math.Floor((start+startEpsilon)/k) * k
}

// CacheProbe tells, for HEAD requests, where a stream of src from start would begin when
// served from the cache (cached true), and whether it would run without a new ffmpeg
// (cached, or produced by a conversion already running).
func (t *Transcoder) CacheProbe(src TranscodeSource, start float64) (realStart float64, cached, shared bool) {
	if t.cache == nil {
		return start, false, false
	}
	c := t.cache
	key := t.cacheKey(src)
	c.mu.Lock()
	defer c.mu.Unlock()
	best, bestStart := -1, 0.0
	for _, e := range c.entries {
		if e.doomed || e.state == cacheFailed || e.meta.Variant != key.variant {
			continue
		}
		idx, res := e.locate(start)
		switch res {
		case locateHit:
			if s := e.srcMin(idx); best < 0 || s > bestStart {
				best, bestStart = idx, s
			}
		case locatePending:
			shared = true
		}
	}
	if best >= 0 {
		return bestStart, true, true
	}
	return start, false, shared
}

// cacheProducer runs ffmpeg into a cache entry. It implements io.Writer for the ffmpeg
// output: it splits it into boxes, writes them to the file and publishes each complete
// fragment, and blocks (pausing ffmpeg) when it is too far ahead of its viewers.
type cacheProducer struct {
	t      *Transcoder
	c      *TranscodeCache
	entry  *cacheEntry
	file   *os.File
	args   []string
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	wake   chan struct{}
	idle   *time.Timer // guarded by c.mu

	// output parsing (producer goroutine only)
	hdr     [16]byte
	hdrN    int
	inBody  bool
	boxType string
	boxLeft uint64
	box     []byte
	init    []byte
	written int64
	pending *cacheFragment
	tracks  []mp4Track
	// file ranges of the head and tail of the first video sample of the pending fragment
	keyHead, keyHeadEnd int64
	keyTail, keyTailEnd int64
	keySize             uint32
	keyHeadBuf          []byte
	keyTailBuf          []byte
	writeErr            error
	startedAt           time.Time
}

// signal wakes the producer (a viewer moved, came or left). Non-blocking.
func (p *cacheProducer) signal() {
	select {
	case p.wake <- struct{}{}:
	default:
	}
}

// run converts until the end of the scene, an error, or the cancellation of the producer
// (idle, slot needed, shutdown).
func (p *cacheProducer) run() {
	defer close(p.done)
	defer p.cancel()
	// finish also runs if ffmpeg handling panics, so that viewers and the slot are released
	err := errors.New("the conversion panicked")
	defer func() { p.finish(err) }()
	err = p.t.run(p.ctx, p.args, p, nil)
}

func (p *cacheProducer) Write(b []byte) (int, error) {
	total := len(b)
	for len(b) > 0 {
		if p.writeErr != nil {
			return 0, p.writeErr
		}
		if !p.inBody {
			need := 8
			if p.hdrN >= 8 && binary.BigEndian.Uint32(p.hdr[:4]) == 1 {
				need = 16
			}
			n := copy(p.hdr[p.hdrN:need], b)
			p.hdrN += n
			b = b[n:]
			if p.hdrN < need || (need == 8 && binary.BigEndian.Uint32(p.hdr[:4]) == 1) {
				continue
			}
			if err := p.startBox(); err != nil {
				p.writeErr = err
				return 0, err
			}
			if p.boxLeft == 0 {
				if err := p.endBox(); err != nil {
					p.writeErr = err
					return 0, err
				}
			}
			continue
		}
		n := len(b)
		if uint64(n) > p.boxLeft {
			n = int(p.boxLeft)
		}
		chunk := b[:n]
		b = b[n:]
		p.boxLeft -= uint64(n)
		switch p.boxType {
		case "ftyp", "moov", "moof":
			p.box = append(p.box, chunk...)
		case "mdat":
			if err := p.writeFile(chunk); err != nil {
				p.writeErr = err
				return 0, err
			}
		}
		if p.boxLeft == 0 {
			if err := p.endBox(); err != nil {
				p.writeErr = err
				return 0, err
			}
		}
	}
	return total, nil
}

func (p *cacheProducer) startBox() error {
	hdr := p.hdr[:p.hdrN]
	typ, size, hdrLen, _ := mp4BoxHeader(hdr)
	p.hdrN = 0
	if size == 0 || size < uint64(hdrLen) {
		return fmt.Errorf("%w: box %q of unsupported size", errMP4Format, typ)
	}
	p.boxType = typ
	p.boxLeft = size - uint64(hdrLen)
	p.inBody = true
	switch typ {
	case "ftyp", "moov", "moof":
		if size > maxMP4HeaderBox {
			return fmt.Errorf("%w: box %q too large", errMP4Format, typ)
		}
		p.box = append(make([]byte, 0, size), hdr...)
	case "mdat":
		if p.pending == nil {
			return fmt.Errorf("%w: mdat without moof", errMP4Format)
		}
		return p.writeFile(hdr)
	}
	return nil
}

func (p *cacheProducer) endBox() error {
	p.inBody = false
	switch p.boxType {
	case "ftyp":
		p.init = append(p.init, p.box...)
	case "moov":
		_, _, hdrLen, _ := mp4BoxHeader(p.box)
		tracks, err := parseMP4Tracks(p.box[hdrLen:])
		if err != nil {
			return err
		}
		p.tracks = tracks
		p.init = append(p.init, p.box...)
		if err := p.writeFile(p.init); err != nil {
			return err
		}
		c := p.c
		c.mu.Lock()
		p.entry.init = p.init
		p.entry.tracks = tracks
		p.entry.size = p.written
		c.total += p.written
		p.entry.broadcast()
		c.mu.Unlock()
	case "moof":
		if p.tracks == nil {
			return fmt.Errorf("%w: moof before moov", errMP4Format)
		}
		frag, err := fragmentFromMoof(p.box, p.tracks)
		if err != nil {
			return err
		}
		frag.offset = p.written
		p.pending = &frag
		p.keyHeadBuf, p.keyTailBuf = p.keyHeadBuf[:0], p.keyTailBuf[:0]
		p.keyHead, p.keyHeadEnd, p.keyTail, p.keyTailEnd = 0, 0, 0, 0
		if off, n, ok := firstSample(p.box, videoTrackID(p.tracks)); ok {
			headEnd, tailStart := sampleKeyRanges(n)
			start := frag.offset + off
			p.keyHead, p.keyHeadEnd = start, start+headEnd
			p.keyTail, p.keyTailEnd = start+tailStart, start+int64(n)
			p.keySize = n
		}
		return p.writeFile(p.box)
	case "mdat":
		frag := *p.pending
		p.pending = nil
		frag.size = p.written - frag.offset
		if p.keyHeadEnd > p.keyHead && int64(len(p.keyHeadBuf)) == p.keyHeadEnd-p.keyHead &&
			int64(len(p.keyTailBuf)) == p.keyTailEnd-p.keyTail {
			frag.key = sampleKey(p.keySize, p.keyHeadBuf, p.keyTailBuf)
		}
		return p.commit(frag)
	}
	p.box = nil
	return nil
}

func (p *cacheProducer) writeFile(b []byte) error {
	// keep the head and tail of the first video sample (keyframe key)
	end := p.written + int64(len(b))
	if lo, hi := max(p.keyHead, p.written), min(p.keyHeadEnd, end); lo < hi {
		p.keyHeadBuf = append(p.keyHeadBuf, b[lo-p.written:hi-p.written]...)
	}
	if lo, hi := max(p.keyTail, p.written), min(p.keyTailEnd, end); lo < hi {
		p.keyTailBuf = append(p.keyTailBuf, b[lo-p.written:hi-p.written]...)
	}
	n, err := p.file.Write(b)
	p.written += int64(n)
	if err != nil {
		return fmt.Errorf("failed to write the cache file: %w", err)
	}
	return nil
}

// commit publishes a complete fragment, then waits while the conversion is too far ahead
// of its viewers (or has none: it is then stopped after the idle timeout).
func (p *cacheProducer) commit(frag cacheFragment) error {
	c, e := p.c, p.entry
	c.mu.Lock()
	e.frags = append(e.frags, frag)
	c.total += frag.size
	e.size += frag.size
	e.broadcast()
	src := e.srcMin(len(e.frags) - 1)
	c.mu.Unlock()

	for {
		c.mu.Lock()
		ahead := len(e.readers) == 0 || src-e.maxReaderPos() > c.readAhead
		c.mu.Unlock()
		if !ahead {
			return nil
		}
		select {
		case <-p.wake:
		case <-p.ctx.Done():
			return p.ctx.Err()
		}
	}
}

// finish records how the conversion ended and frees its slot.
func (p *cacheProducer) finish(runErr error) {
	c, e := p.c, p.entry
	stopped := p.ctx.Err() != nil

	c.mu.Lock()
	switch {
	case p.writeErr != nil && !errors.Is(p.writeErr, context.Canceled):
		e.state = cacheFailed
	case runErr == nil:
		e.state = cacheComplete
	case stopped:
		e.state = cachePartial
	default:
		e.state = cacheFailed
	}
	if p.idle != nil {
		p.idle.Stop()
		p.idle = nil
	}
	e.producer = nil
	size := e.size
	keep := (e.state == cacheComplete && len(e.frags) > 0) || (e.state == cachePartial && len(e.frags) >= 2)
	meta := e.meta
	meta.Complete = e.state == cacheComplete
	c.mu.Unlock()

	truncErr := p.file.Truncate(size)
	closeErr := p.file.Close()
	if keep && (truncErr != nil || closeErr != nil) {
		keep = false
	}
	if keep {
		if err := writeCacheMeta(e, meta); err != nil {
			c.logger.Warn("Failed to save a transcode cache entry", zap.String("entry", e.id), zap.Error(err))
			keep = false
		}
	}
	p.t.releaseSlot()

	c.mu.Lock()
	if !keep {
		e.doomed = true
	}
	e.broadcast()
	paths := c.enforceLocked()
	if !keep && !e.pinned() {
		paths = append(paths, c.removeLocked(e)...)
	}
	frags := len(e.frags)
	state := e.state
	c.mu.Unlock()
	removeCacheFiles(c.logger, paths)

	fields := []zap.Field{
		zap.Uint("scene_id", meta.SceneID),
		zap.String("entry", e.id),
		zap.String("state", state.String()),
		zap.Int("fragments", frags),
		zap.Int64("bytes", size),
		zap.Duration("elapsed", time.Since(p.startedAt)),
	}
	switch {
	case state == cacheFailed && p.writeErr != nil:
		c.logger.Warn("On-the-fly transcode failed", append(fields, zap.Error(p.writeErr))...)
	case state == cacheFailed:
		c.logger.Warn("On-the-fly transcode failed", append(fields, zap.Error(runErr))...)
	case state == cachePartial:
		c.logger.Info("On-the-fly transcode stopped", fields...)
	default:
		c.logger.Info("On-the-fly transcode finished", fields...)
	}
}

// startProducerLocked starts ffmpeg into a new entry from start (the caller holds a slot).
// A copy starts on the keyframe at or before start, which is where the entry begins: start
// is a keyframe position (see KeyframeStart).
func (t *Transcoder) startProducerLocked(key cacheKey, src TranscodeSource, start float64) (*cacheEntry, error) {
	c := t.cache
	e, f, err := c.newEntryLocked(key, start)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(c.baseCtx)
	p := &cacheProducer{
		t:         t,
		c:         c,
		entry:     e,
		file:      f,
		args:      t.Args(src.Path, src.Plan, start),
		ctx:       ctx,
		cancel:    cancel,
		done:      make(chan struct{}),
		wake:      make(chan struct{}, 1),
		startedAt: time.Now(),
	}
	e.producer = p
	c.lifecycle.Go("transcode-cache-producer", func(<-chan struct{}) { p.run() })
	c.logger.Info("On-the-fly transcode started",
		zap.Uint("scene_id", src.SceneID),
		zap.String("entry", e.id),
		zap.String("mode", string(src.Plan.Mode)),
		zap.Bool("copy_video", src.Plan.CopyVideo),
		zap.Bool("copy_audio", src.Plan.CopyAudio),
		zap.Float64("start", start),
	)
	return e, nil
}

// CachedStream is one response served from the cache: the init segment, then fragments
// from one entry after another with their decode times rebased.
type CachedStream struct {
	t   *Transcoder
	c   *TranscodeCache
	src TranscodeSource
	key cacheKey
	ctx context.Context

	entry *cacheEntry
	file  *os.File
	next  int
	// seconds removed from the decode times of the current entry
	shift float64
	init  []byte
	// tracks of the init segment sent
	entryTracks []mp4Track
	start       float64
	seq         uint32
	// what the response sent: fragments from cache (no ffmpeg run for them) and joins
	hits, joins int
}

// Start is the scene position where the stream begins.
func (s *CachedStream) Start() float64 { return s.start }

// OpenCached opens the stream of src from start, served from the cache, from a conversion
// already running, or from a new one. It waits for the first fragment. Returns
// ErrTranscodeLimit when a new conversion is needed and no slot is free.
func (t *Transcoder) OpenCached(ctx context.Context, src TranscodeSource, start float64) (*CachedStream, error) {
	if t.cache == nil {
		return nil, errors.New("transcode cache disabled")
	}
	s := &CachedStream{t: t, c: t.cache, src: src, key: t.cacheKey(src), ctx: ctx}
	locate := func(e *cacheEntry) (int, locateResult) { return e.locate(start) }
	e, created, err := s.acquire(locate, t.AlignStart(src.Plan, start), start)
	if err != nil {
		return nil, err
	}
	idx, err := s.wait(e, locate)
	if err != nil {
		s.Close()
		return nil, err
	}
	s.c.mu.Lock()
	frag := e.frags[idx]
	s.shift = frag.min
	s.start = e.srcMin(idx)
	s.init = e.init
	s.entryTracks = e.tracks
	s.next = idx
	shared := e.producer != nil
	s.c.mu.Unlock()
	if !created {
		msg := "On-the-fly transcode served from cache"
		if shared {
			msg = "On-the-fly transcode shared with a running conversion"
		}
		s.c.logger.Info(msg,
			zap.Uint("scene_id", src.SceneID),
			zap.String("entry", e.id),
			zap.Float64("start", s.start),
		)
	}
	return s, nil
}

// acquire attaches the stream to the best entry for find, or starts a conversion from
// newStart when none fits (stopping an idle conversion when all slots are used).
func (s *CachedStream) acquire(find func(*cacheEntry) (int, locateResult), newStart, readerPos float64) (*cacheEntry, bool, error) {
	c, t := s.c, s.t
	for attempt := 0; attempt < 3; attempt++ {
		c.mu.Lock()
		if c.stopped {
			c.mu.Unlock()
			return nil, false, ErrTranscoderStopped
		}
		paths := c.invalidateLocked(s.key)
		var hit, pending *cacheEntry
		hitStart := 0.0
		for _, e := range c.entries {
			if e.doomed || e.state == cacheFailed || e.meta.Variant != s.key.variant {
				continue
			}
			idx, res := find(e)
			switch res {
			case locateHit:
				src := e.srcMin(idx)
				if hit == nil || src > hitStart || (src == hitStart && e.state == cacheComplete) {
					hit, hitStart = e, src
				}
			case locatePending:
				if pending == nil {
					pending = e
				}
			}
		}
		chosen := hit
		if chosen == nil {
			chosen = pending
		}
		if chosen != nil {
			s.attachLocked(chosen, readerPos)
			c.mu.Unlock()
			removeCacheFiles(c.logger, paths)
			return chosen, false, nil
		}
		if t.tryAcquireSlot() {
			paths = append(paths, c.enforceLocked()...)
			e, err := t.startProducerLocked(s.key, s.src, newStart)
			if err != nil {
				c.mu.Unlock()
				t.releaseSlot()
				removeCacheFiles(c.logger, paths)
				return nil, false, err
			}
			s.attachLocked(e, readerPos)
			c.mu.Unlock()
			removeCacheFiles(c.logger, paths)
			return e, true, nil
		}
		// All slots used: stop the conversions nobody watches
		var idle []*cacheProducer
		for _, e := range c.entries {
			if e.producer != nil && len(e.readers) == 0 {
				idle = append(idle, e.producer)
				e.producer.cancel()
			}
		}
		c.mu.Unlock()
		removeCacheFiles(c.logger, paths)
		if len(idle) == 0 {
			return nil, false, ErrTranscodeLimit
		}
		for _, p := range idle {
			select {
			case <-p.done:
			case <-time.After(replaceWait):
			case <-s.ctx.Done():
				return nil, false, s.ctx.Err()
			}
		}
	}
	return nil, false, ErrTranscodeLimit
}

// attachLocked makes the stream a viewer of e at source position pos.
func (s *CachedStream) attachLocked(e *cacheEntry, pos float64) {
	e.readers[s] = pos
	e.lastUsed = s.c.now()
	if p := e.producer; p != nil {
		if p.idle != nil {
			p.idle.Stop()
			p.idle = nil
		}
		p.signal()
	}
	s.entry = e
}

// detachLocked stops viewing the current entry. A conversion left without viewers keeps
// running (paused) for the idle timeout, then stops. Returns files to delete.
func (s *CachedStream) detachLocked() []string {
	e := s.entry
	if e == nil {
		return nil
	}
	s.entry = nil
	delete(e.readers, s)
	e.lastUsed = s.c.now()
	if p := e.producer; p != nil {
		p.signal()
		if len(e.readers) == 0 && p.idle == nil {
			c := s.c
			p.idle = time.AfterFunc(c.idleTimeout, func() {
				c.mu.Lock()
				defer c.mu.Unlock()
				if e.producer == p && len(e.readers) == 0 {
					p.cancel()
				}
			})
		}
	}
	if e.doomed && !e.pinned() {
		return s.c.removeLocked(e)
	}
	return nil
}

// wait waits until find locates the fragment in e.
func (s *CachedStream) wait(e *cacheEntry, find func(*cacheEntry) (int, locateResult)) (int, error) {
	for {
		s.c.mu.Lock()
		idx, res := find(e)
		if res == locateHit {
			s.c.mu.Unlock()
			return idx, nil
		}
		if res == locateMiss || e.state != cacheProducing {
			state := e.state
			s.c.mu.Unlock()
			if state == cacheFailed {
				return -1, fmt.Errorf("%w: ffmpeg failed", ErrCachedStreamBroken)
			}
			return -1, fmt.Errorf("%w: position not produced", ErrCachedStreamBroken)
		}
		ch := e.changed
		s.c.mu.Unlock()
		select {
		case <-ch:
		case <-s.ctx.Done():
			return -1, s.ctx.Err()
		}
	}
}

// Close releases the stream.
func (s *CachedStream) Close() {
	s.c.mu.Lock()
	e := s.entry
	paths := s.detachLocked()
	finished := e != nil && e.state != cacheProducing && !e.doomed
	s.c.mu.Unlock()
	removeCacheFiles(s.c.logger, paths)
	if s.file != nil {
		_ = s.file.Close()
		s.file = nil
	}
	if finished {
		// last use, kept across restarts
		now := s.c.now()
		_ = os.Chtimes(e.metaPath(), now, now)
	}
}

// WriteTo sends the stream to w until the end of the scene, an error or the end of the
// request context.
func (s *CachedStream) WriteTo(w io.Writer) (int64, error) {
	var sent int64
	n, err := w.Write(s.init)
	sent += int64(n)
	if err != nil {
		return sent, fmt.Errorf("failed to send the stream: %w", err)
	}
	var data []byte
	for {
		frag, ok, err := s.nextFragment()
		if err != nil {
			return sent, err
		}
		if !ok {
			return sent, nil
		}
		if s.file == nil {
			f, err := os.Open(s.entry.path)
			if err != nil {
				return sent, fmt.Errorf("failed to open the cache file: %w", err)
			}
			s.file = f
		}
		if int64(cap(data)) < frag.size {
			data = make([]byte, frag.size)
		}
		data = data[:frag.size]
		if _, err := s.file.ReadAt(data, frag.offset); err != nil {
			return sent, fmt.Errorf("failed to read the cache file: %w", err)
		}
		s.seq++
		if err := rewriteMoof(data[:frag.moofSize], s.seq, s.rebase); err != nil {
			return sent, err
		}
		n, err := w.Write(data)
		sent += int64(n)
		if err != nil {
			return sent, fmt.Errorf("failed to send the stream: %w", err)
		}
	}
}

// rebase shifts a decode time of the current entry to the stream timeline.
func (s *CachedStream) rebase(trackID uint32, old uint64) uint64 {
	for _, tr := range s.entry.tracks {
		if tr.ID != trackID {
			continue
		}
		v := int64(old) - int64(math.Round(s.shift*float64(tr.Timescale)))
		if v < 0 {
			return 0
		}
		return uint64(v)
	}
	return old
}

// sameInit tells whether the fragments of e can follow an init segment: same tracks. The
// init segments of two runs of the same plan only differ in bitrate estimates (MP4 esds of
// copied audio), so they have the same size.
func sameInit(e *cacheEntry, init []byte, tracks []mp4Track) bool {
	if len(e.init) != len(init) || len(e.tracks) != len(tracks) {
		return false
	}
	for i := range tracks {
		if e.tracks[i] != tracks[i] {
			return false
		}
	}
	return true
}

// betterEntryLocked tells whether another entry already holds the keyframe at source
// position at of the running conversion e: a finished one, or a running one further ahead
// (never one behind, so that two running conversions don't take turns).
func (c *TranscodeCache) betterEntryLocked(s *CachedStream, e *cacheEntry, at float64, key uint64) bool {
	for _, o := range c.entries {
		if o == e || o.doomed || o.state == cacheFailed || o.meta.Variant != e.meta.Variant {
			continue
		}
		if o.state == cacheProducing && o.lastSrc() <= e.lastSrc() {
			continue
		}
		if !sameInit(o, s.init, s.entryTracks) {
			continue
		}
		if _, res := o.locateJoin(at, key, false); res == locateHit {
			return true
		}
	}
	return false
}

// nextFragment returns the next fragment to send (ok false at the end of the scene),
// waiting for the conversion, or continuing in another entry where a partial one ends.
func (s *CachedStream) nextFragment() (cacheFragment, bool, error) {
	c := s.c
	for {
		c.mu.Lock()
		e := s.entry
		if s.next < e.usable() && e.state == cacheProducing && c.betterEntryLocked(s, e, e.srcVideo(s.next), s.keyOf(e.frags[s.next])) {
			// The rest was already converted: leave this conversion (it stops when idle)
			at := e.srcVideo(s.next)
			outVideo := e.frags[s.next].video - s.shift
			key := s.keyOf(e.frags[s.next])
			c.mu.Unlock()
			if err := s.join(at, outVideo, key); err != nil {
				return cacheFragment{}, false, err
			}
			continue
		}
		if s.next < e.usable() {
			frag := e.frags[s.next]
			e.readers[s] = e.srcMin(s.next)
			s.next++
			if p := e.producer; p != nil {
				p.signal()
			} else {
				s.hits++
			}
			c.mu.Unlock()
			return frag, true, nil
		}
		switch e.state {
		case cacheProducing:
			ch := e.changed
			c.mu.Unlock()
			select {
			case <-ch:
			case <-s.ctx.Done():
				return cacheFragment{}, false, s.ctx.Err()
			}
			continue
		case cacheComplete:
			c.mu.Unlock()
			return cacheFragment{}, false, nil
		case cacheFailed:
			c.mu.Unlock()
			return cacheFragment{}, false, fmt.Errorf("%w: ffmpeg failed", ErrCachedStreamBroken)
		}
		// Partial entry: continue at its last keyframe in another entry
		if s.next >= len(e.frags) {
			c.mu.Unlock()
			return cacheFragment{}, false, fmt.Errorf("%w: end of a partial conversion", ErrCachedStreamBroken)
		}
		at := e.srcVideo(s.next)
		outVideo := e.frags[s.next].video - s.shift
		key := s.keyOf(e.frags[s.next])
		c.mu.Unlock()
		if err := s.join(at, outVideo, key); err != nil {
			return cacheFragment{}, false, err
		}
	}
}

// keyOf is the key matching a keyframe of the stream in other entries: the copied sample
// for stream copies, none (times are exact) for re-encodes.
func (s *CachedStream) keyOf(f cacheFragment) uint64 {
	if !s.src.Plan.CopyVideo {
		return 0
	}
	return f.key
}

// join moves the stream to the entry holding the keyframe at source position at (and with
// that key for stream copies), placed at outVideo in the stream timeline.
func (s *CachedStream) join(at, outVideo float64, key uint64) error {
	c := s.c
	old := s.entry
	oldFile := s.file
	s.file = nil
	c.mu.Lock()
	paths := s.detachLocked()
	c.mu.Unlock()
	removeCacheFiles(c.logger, paths)
	if oldFile != nil {
		_ = oldFile.Close()
	}

	existing := func(e *cacheEntry) (int, locateResult) {
		if e == old {
			return -1, locateMiss
		}
		// leaving a running conversion: not for another one behind it
		if old.state == cacheProducing && e.state == cacheProducing && e.lastSrc() <= old.lastSrc() {
			return -1, locateMiss
		}
		return e.locateJoin(at, key, false)
	}
	start := at
	if s.src.Plan.CopyVideo {
		// The stream copy must start exactly on that keyframe: its decode time (what the
		// cache knows) is a little before its presentation time (what -ss looks at)
		start = s.t.KeyframeStart(s.ctx, s.src.Path, at+joinTolerance)
	}
	e, created, err := s.acquire(existing, start, at)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrCachedStreamBroken, err)
	}
	idx, err := s.wait(e, func(e *cacheEntry) (int, locateResult) { return e.locateJoin(at, key, created) })
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !sameInit(e, s.init, s.entryTracks) {
		return fmt.Errorf("%w: the conversions do not have the same format", ErrCachedStreamBroken)
	}
	frag := e.frags[idx]
	if created && key != 0 && frag.key == key {
		// The copy started before its -ss: its times are now known from the matched keyframe
		e.meta.Start += at - e.srcVideo(idx)
	}
	s.shift = frag.video - (outVideo + (e.srcVideo(idx) - at))
	s.next = idx
	s.joins++
	c.logger.Debug("On-the-fly transcode continued in another cache entry",
		zap.Uint("scene_id", s.src.SceneID),
		zap.String("entry", e.id),
		zap.Bool("new_conversion", created),
		zap.Float64("at", at),
	)
	return nil
}

// Stats of the response (for the logs): fragments served from finished cache entries and
// number of entries joined.
func (s *CachedStream) Stats() (hits, joins int) { return s.hits, s.joins }
