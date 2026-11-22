package streaming

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"goonhub/internal/config"

	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// Synthetic fragmented MP4 (what ffmpeg writes, reduced to the boxes the cache reads)
// ---------------------------------------------------------------------------

const (
	testVideoScale = 1000
	testAudioScale = 48000
)

func mp4Box(typ string, payload ...[]byte) []byte {
	size := 8
	for _, p := range payload {
		size += len(p)
	}
	b := make([]byte, 8, size)
	binary.BigEndian.PutUint32(b, uint32(size))
	copy(b[4:], typ)
	for _, p := range payload {
		b = append(b, p...)
	}
	return b
}

func u32(v uint32) []byte { return binary.BigEndian.AppendUint32(nil, v) }
func u64(v uint64) []byte { return binary.BigEndian.AppendUint64(nil, v) }

func testTrak(id, timescale uint32, handler string) []byte {
	tkhd := append(append(u32(0), u32(0)...), u32(0)...) // version/flags, ctime, mtime
	tkhd = append(tkhd, u32(id)...)
	tkhd = append(tkhd, make([]byte, 64)...)
	mdhd := append(append(u32(0), u32(0)...), u32(0)...)
	mdhd = append(mdhd, u32(timescale)...)
	mdhd = append(mdhd, make([]byte, 8)...)
	hdlr := append(u32(0), u32(0)...)
	hdlr = append(hdlr, []byte(handler)...)
	hdlr = append(hdlr, make([]byte, 13)...)
	return mp4Box("trak", mp4Box("tkhd", tkhd), mp4Box("mdia", mp4Box("mdhd", mdhd), mp4Box("hdlr", hdlr)))
}

func testInitSegment() []byte {
	ftyp := mp4Box("ftyp", []byte("isom"), u32(512), []byte("isomiso6mp41"))
	moov := mp4Box("moov", mp4Box("mvhd", make([]byte, 100)), testTrak(1, testVideoScale, "vide"), testTrak(2, testAudioScale, "soun"))
	return append(ftyp, moov...)
}

func testTraf(track uint32, time uint64) []byte {
	return mp4Box("traf",
		mp4Box("tfhd", u32(0x020000), u32(track)),
		mp4Box("tfdt", u32(0x01000000), u64(time)),
		mp4Box("trun", u32(0), u32(0)),
	)
}

// testFragment is a fragment at out seconds in its run whose mdat says which source second
// it holds.
func testFragment(seq uint32, out, src float64) []byte {
	videoTime := uint64(math.Round(out * testVideoScale))
	audioOut := out - 0.01
	if audioOut < 0 {
		audioOut = 0
	}
	moof := mp4Box("moof",
		mp4Box("mfhd", u32(0), u32(seq)),
		testTraf(1, videoTime),
		testTraf(2, uint64(math.Round(audioOut*testAudioScale))),
	)
	return append(moof, mp4Box("mdat", []byte(fmt.Sprintf("src=%09.3f", src)))...)
}

// servedFragment is a fragment of a stream as the player gets it.
type servedFragment struct {
	seq   uint32
	video float64 // seconds
	audio float64
	src   float64
}

func parseServed(t *testing.T, b []byte) ([]byte, []servedFragment) {
	t.Helper()
	var init []byte
	var frags []servedFragment
	var cur *servedFragment
	err := eachMP4Box(b, func(typ string, off int, payload []byte) error {
		switch typ {
		case "ftyp", "moov":
			_, size, _, _ := mp4BoxHeader(b[off:])
			init = append(init, b[off:off+int(size)]...)
		case "moof":
			_, size, _, _ := mp4BoxHeader(b[off:])
			moof := b[off : off+int(size)]
			times, err := parseMoof(moof)
			if err != nil {
				return err
			}
			f := servedFragment{seq: binary.BigEndian.Uint32(moof[20:24])}
			for _, dt := range times {
				if dt.TrackID == 1 {
					f.video = float64(dt.Time) / testVideoScale
				} else {
					f.audio = float64(dt.Time) / testAudioScale
				}
			}
			frags = append(frags, f)
			cur = &frags[len(frags)-1]
		case "mdat":
			src, err := strconv.ParseFloat(strings.TrimPrefix(string(payload), "src="), 64)
			if err != nil || cur == nil {
				return fmt.Errorf("bad mdat %q", payload)
			}
			cur.src = src
			cur = nil
		}
		return nil
	})
	if err != nil {
		t.Fatalf("invalid stream: %v", err)
	}
	return init, frags
}

// checkContinuous checks that the stream plays the scene from start without gap: each
// fragment holds the source second matching its time.
func checkContinuous(t *testing.T, frags []servedFragment, start, end float64) {
	t.Helper()
	if len(frags) == 0 {
		t.Fatal("empty stream")
	}
	for i, f := range frags {
		if f.seq != uint32(i+1) {
			t.Fatalf("fragment %d has sequence %d", i, f.seq)
		}
		if math.Abs(start+f.video-f.src) > 0.002 {
			t.Fatalf("fragment %d at %.3f holds source %.3f (stream from %.3f)", i, f.video, f.src, start)
		}
		if i > 0 && math.Abs(f.video-frags[i-1].video-2) > 0.002 {
			t.Fatalf("gap between fragments %d (%.3f) and %d (%.3f)", i-1, frags[i-1].video, i, f.video)
		}
		if f.audio > f.video+0.001 || f.video-f.audio > 0.011 {
			t.Fatalf("fragment %d: audio %.4f out of sync with video %.4f", i, f.audio, f.video)
		}
	}
	if last := frags[len(frags)-1].src; math.Abs(last-end) > 0.002 {
		t.Fatalf("stream ends at source %.3f, want %.3f", last, end)
	}
}

// fakeFFmpeg writes a synthetic conversion of a scene with one fragment every 2 s.
type fakeFFmpeg struct {
	duration float64

	mu        sync.Mutex
	runs      []float64
	failAfter int           // fail after this many fragments (0: never)
	gate      chan struct{} // when set, each fragment after the first waits for a token
}

func (f *fakeFFmpeg) run(ctx context.Context, args []string, w io.Writer, _ []byte) error {
	start := 0.0
	if s := argValue(args, "-ss"); s != "" {
		start, _ = strconv.ParseFloat(s, 64)
	}
	f.mu.Lock()
	f.runs = append(f.runs, start)
	failAfter, gate := f.failAfter, f.gate
	f.mu.Unlock()

	// written in odd pieces to exercise the box parser
	write := func(b []byte) error {
		for len(b) > 0 {
			n := min(7, len(b))
			if _, err := w.Write(b[:n]); err != nil {
				return err
			}
			b = b[n:]
			if len(b) > 0 {
				if _, err := w.Write(b[:min(1000, len(b))]); err != nil {
					return err
				}
				b = b[min(1000, len(b)):]
			}
		}
		return nil
	}
	if err := write(testInitSegment()); err != nil {
		return err
	}
	for k := 0; start+float64(k)*2 < f.duration; k++ {
		if k > 0 && gate != nil {
			select {
			case <-gate:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if failAfter > 0 && k >= failAfter {
			return errors.New("ffmpeg failed: broken file")
		}
		if err := write(testFragment(uint32(k+1), float64(k)*2, start+float64(k)*2)); err != nil {
			return err
		}
	}
	// ffmpeg ends with an mfra box, which the cache skips
	return write(mp4Box("mfra", make([]byte, 16)))
}

func (f *fakeFFmpeg) runCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.runs)
}

func (f *fakeFFmpeg) runStarts() []float64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]float64(nil), f.runs...)
}

// ---------------------------------------------------------------------------
// Fixture
// ---------------------------------------------------------------------------

type cacheFixture struct {
	tc    *Transcoder
	cache *TranscodeCache
	ff    *fakeFFmpeg
	dir   string
	file  string
	clock *testClock
}

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

func testCacheConfig(dir string) config.TranscodeCacheConfig {
	return config.TranscodeCacheConfig{
		Enabled: true, Dir: dir, MaxSizeGB: 1, MaxAge: time.Hour, CleanupInterval: time.Hour,
		ReadAhead: time.Hour, IdleTimeout: 30 * time.Millisecond,
	}
}

func newCacheFixture(t *testing.T, maxConcurrent int, duration float64) *cacheFixture {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "cache")
	f := &cacheFixture{ff: &fakeFFmpeg{duration: duration}, dir: dir, clock: &testClock{now: time.Now()}}
	f.file = filepath.Join(t.TempDir(), "scene.avi")
	if err := os.WriteFile(f.file, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.open(t, maxConcurrent)
	return f
}

// open (re)creates the transcoder and its cache on the fixture directory.
func (f *cacheFixture) open(t *testing.T, maxConcurrent int) {
	t.Helper()
	cache, err := NewTranscodeCache(testCacheConfig(f.dir), zap.NewNop())
	if err != nil {
		t.Fatalf("NewTranscodeCache: %v", err)
	}
	cache.now = f.clock.Now
	cache.load()
	tc := NewTranscoder(config.TranscodeConfig{MaxConcurrent: maxConcurrent}, nil, zap.NewNop())
	tc.SetCache(cache)
	tc.SetRunFunc(f.ff.run)
	t.Cleanup(tc.Stop)
	f.tc, f.cache = tc, cache
}

func (f *cacheFixture) source(t *testing.T, sceneID uint) TranscodeSource {
	t.Helper()
	info, err := os.Stat(f.file)
	if err != nil {
		t.Fatal(err)
	}
	return TranscodeSource{SceneID: sceneID, Path: f.file, Size: info.Size(), ModTime: info.ModTime(), Plan: FullTranscodePlan()}
}

// play streams src from start to the end and returns what the player got.
func (f *cacheFixture) play(t *testing.T, src TranscodeSource, start float64) (float64, []servedFragment) {
	t.Helper()
	s, err := f.tc.OpenCached(context.Background(), src, start)
	if err != nil {
		t.Fatalf("OpenCached(%v): %v", start, err)
	}
	defer s.Close()
	var out bytes.Buffer
	if _, err := s.WriteTo(&out); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	init, frags := parseServed(t, out.Bytes())
	if !bytes.Equal(init, testInitSegment()) {
		t.Fatal("the stream does not start with the init segment")
	}
	return s.Start(), frags
}

// cutWriter accepts n writes (the init segment, then one per fragment) then fails, like a
// player seeking elsewhere.
type cutWriter struct {
	n   int
	buf bytes.Buffer
}

func (w *cutWriter) Write(p []byte) (int, error) {
	if w.n <= 0 {
		return 0, errors.New("client gone")
	}
	w.n--
	return w.buf.Write(p)
}

// playPart streams src from start and stops after frags fragments.
func (f *cacheFixture) playPart(t *testing.T, src TranscodeSource, start float64, frags int) {
	t.Helper()
	s, err := f.tc.OpenCached(context.Background(), src, start)
	if err != nil {
		t.Fatalf("OpenCached(%v): %v", start, err)
	}
	w := &cutWriter{n: frags + 1}
	if _, err := s.WriteTo(w); err == nil {
		t.Fatal("expected the cut stream to stop early")
	}
	s.Close()
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (f *cacheFixture) idle(t *testing.T) {
	t.Helper()
	waitFor(t, "the conversions to stop", func() bool { return f.cache.Stats().InProgress == 0 })
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestFMP4RewriteMoof(t *testing.T) {
	frag := testFragment(9, 10, 10)
	moofSize := int(binary.BigEndian.Uint32(frag))
	moof := frag[:moofSize]
	if err := rewriteMoof(moof, 3, func(track uint32, old uint64) uint64 { return old / 2 }); err != nil {
		t.Fatal(err)
	}
	_, frags := parseServed(t, frag)
	if frags[0].seq != 3 || frags[0].video != 5 || math.Abs(frags[0].audio-4.995) > 1e-6 {
		t.Fatalf("got %+v", frags[0])
	}
	init := testInitSegment()
	_, ftypSize, _, _ := mp4BoxHeader(init)
	tracks, err := parseMP4Tracks(init[ftypSize+8:])
	if err != nil || len(tracks) != 2 || !tracks[0].Video || tracks[1].Timescale != testAudioScale {
		t.Fatalf("tracks %+v, %v", tracks, err)
	}
}

func TestCacheKey(t *testing.T) {
	tc := NewTranscoder(config.TranscodeConfig{}, nil, zap.NewNop())
	defer tc.Stop()
	mod := time.Unix(1700000000, 0)
	base := TranscodeSource{SceneID: 1, Path: "/a.avi", Size: 100, ModTime: mod, Plan: FullTranscodePlan()}
	key := tc.cacheKey(base)
	if again := tc.cacheKey(base); again != key {
		t.Fatal("the key must be stable")
	}
	moved := base
	moved.Path = "/elsewhere/a.avi"
	if tc.cacheKey(moved).variant != key.variant {
		t.Fatal("the path is not part of the key (the file is identified by scene, size and date)")
	}
	changes := map[string]func(*TranscodeSource){
		"scene":   func(s *TranscodeSource) { s.SceneID = 2 },
		"size":    func(s *TranscodeSource) { s.Size = 101 },
		"mtime":   func(s *TranscodeSource) { s.ModTime = mod.Add(time.Second) },
		"plan":    func(s *TranscodeSource) { s.Plan = DecidePlayback("/a.mkv", "h264", "aac") },
		"copyAAC": func(s *TranscodeSource) { s.Plan = DecidePlayback("/a.mkv", "hevc", "aac") },
	}
	for name, change := range changes {
		src := base
		change(&src)
		if tc.cacheKey(src).variant == key.variant {
			t.Fatalf("%s change must change the key", name)
		}
	}
	other := NewTranscoder(config.TranscodeConfig{CRF: 30}, nil, zap.NewNop())
	defer other.Stop()
	if k := other.cacheKey(base); k.variant == key.variant || k.planKey == key.planKey {
		t.Fatal("other conversion settings must change the key")
	}
}

func TestAlignStart(t *testing.T) {
	f := newCacheFixture(t, 2, 30)
	plan := FullTranscodePlan()
	for in, want := range map[float64]float64{0: 0, 1.9: 0, 2: 2, 3.99: 2, 3.9995: 4, 125.5: 124} {
		if got := f.tc.AlignStart(plan, in); got != want {
			t.Fatalf("AlignStart(%v) = %v, want %v", in, got, want)
		}
	}
	if got := f.tc.AlignStart(DecidePlayback("/a.mkv", "h264", "aac"), 13.3); got != 13.3 {
		t.Fatalf("a remux is not aligned (keyframes of the file), got %v", got)
	}
}

func TestCache_ReplayAndSeekBackServedFromCache(t *testing.T) {
	f := newCacheFixture(t, 2, 30)
	src := f.source(t, 7)

	start, frags := f.play(t, src, 0)
	checkContinuous(t, frags, start, 28)
	if f.ff.runCount() != 1 {
		t.Fatalf("expected one ffmpeg run, got %v", f.ff.runStarts())
	}
	f.idle(t)

	// Replay
	start, frags = f.play(t, src, 0)
	checkContinuous(t, frags, start, 28)
	// Seek back into the converted part: the stream starts with the fragment holding the
	// position (its audio begins 10 ms before its video at 10)
	got, cached, _ := f.tc.CacheProbe(src, 11.2)
	if !cached || math.Abs(got-9.99) > 1e-6 {
		t.Fatalf("CacheProbe(11.2) = %v, %v; want the fragment at 9.99", got, cached)
	}
	start, frags = f.play(t, src, got)
	if start != got || frags[0].audio != 0 || math.Abs(frags[0].video-0.01) > 1e-6 {
		t.Fatalf("stream from %v starts at %+v, want it rebased to 0", start, frags[0])
	}
	checkContinuous(t, frags, start, 28)
	if f.ff.runCount() != 1 {
		t.Fatalf("replays must not run ffmpeg again, runs %v", f.ff.runStarts())
	}
	stats := f.cache.Stats()
	if stats.Entries != 1 || stats.SizeBytes <= 0 || stats.InProgress != 0 {
		t.Fatalf("stats %+v", stats)
	}
}

func TestCache_SecondViewerSharesTheConversion(t *testing.T) {
	f := newCacheFixture(t, 2, 20)
	f.ff.gate = make(chan struct{})
	src := f.source(t, 7)

	type result struct {
		start float64
		out   bytes.Buffer
		err   error
	}
	results := make([]*result, 2)
	var wg sync.WaitGroup
	for i := range results {
		r := &result{}
		results[i] = r
		s, err := f.tc.OpenCached(context.Background(), src, 0)
		if err != nil {
			t.Fatalf("viewer %d: %v", i, err)
		}
		r.start = s.Start()
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer s.Close()
			_, r.err = s.WriteTo(&r.out)
		}()
	}
	// The conversion goes on fragment by fragment while both watch
	for i := 0; i < 9; i++ {
		f.ff.gate <- struct{}{}
	}
	wg.Wait()
	for i, r := range results {
		if r.err != nil {
			t.Fatalf("viewer %d: %v", i, r.err)
		}
		_, frags := parseServed(t, r.out.Bytes())
		checkContinuous(t, frags, r.start, 18)
	}
	if n := f.ff.runCount(); n != 1 {
		t.Fatalf("two viewers of the same part must share one ffmpeg run, got %d", n)
	}
}

func TestCache_PartialEntryIsContinuedAndReused(t *testing.T) {
	f := newCacheFixture(t, 2, 30)
	f.cache.readAhead = 1
	src := f.source(t, 7)

	// Watch 0-6 then leave: the conversion stops (idle) and keeps what it made
	f.playPart(t, src, 0, 3)
	f.idle(t)
	if f.ff.runCount() != 1 {
		t.Fatalf("runs %v", f.ff.runStarts())
	}
	f.cache.mu.Lock()
	var partial *cacheEntry
	for _, e := range f.cache.entries {
		partial = e
	}
	state, n := partial.state, len(partial.frags)
	f.cache.mu.Unlock()
	if state != cachePartial || n < 2 {
		t.Fatalf("expected a partial entry, got %v with %d fragments", state, n)
	}
	if _, err := os.Stat(partial.metaPath()); err != nil {
		t.Fatalf("a stopped conversion is kept: %v", err)
	}

	// Seek back to 2: served from the partial entry, then ffmpeg goes on from where it ends
	f.cache.readAhead = 3600
	start, frags := f.play(t, src, 2)
	checkContinuous(t, frags, start, 28)
	runs := f.ff.runStarts()
	joinAt := float64((n - 1) * 2)
	if len(runs) != 2 || runs[1] != joinAt {
		t.Fatalf("expected a second run from the end of the partial entry (%v), got %v", joinAt, runs)
	}

	// Watching it all again needs no ffmpeg at all: both entries follow each other
	start, frags = f.play(t, src, 0)
	checkContinuous(t, frags, start, 28)
	if f.ff.runCount() != 2 {
		t.Fatalf("runs %v", f.ff.runStarts())
	}
}

func TestCache_SeekAheadStartsAlignedConversionAndJoinsIt(t *testing.T) {
	f := newCacheFixture(t, 2, 40)
	f.cache.readAhead = 1
	src := f.source(t, 7)

	// Viewer seeks to 21.3: conversion from the keyframe grid point 20
	if got := f.tc.AlignStart(src.Plan, 21.3); got != 20 {
		t.Fatalf("aligned start %v", got)
	}
	f.cache.readAhead = 3600
	start, frags := f.play(t, src, 20)
	checkContinuous(t, frags, start, 38)
	f.idle(t)

	// Then watches from 0 and stops at 4
	f.cache.readAhead = 1
	f.playPart(t, src, 0, 2)
	f.idle(t)

	// From 0 to the end: the partial entry, a new run until 20, then the entry made by the
	// seek (the new run is left there)
	f.cache.readAhead = 1
	start, frags = f.play(t, src, 0)
	checkContinuous(t, frags, start, 38)
	runs := f.ff.runStarts()
	if len(runs) != 3 {
		t.Fatalf("expected 3 runs (seek, start, continuation), got %v", runs)
	}
	f.idle(t)
	f.cache.mu.Lock()
	defer f.cache.mu.Unlock()
	for _, e := range f.cache.entries {
		if e.meta.Start == runs[2] && (e.state != cachePartial || e.lastSrc() > 24) {
			t.Fatalf("the continuation should stop near 20 where the cached part begins, it went to %v (%v)", e.lastSrc(), e.state)
		}
	}
}

func TestCache_FailedConversionIsNotKept(t *testing.T) {
	f := newCacheFixture(t, 2, 30)
	f.ff.failAfter = 3
	src := f.source(t, 7)

	s, err := f.tc.OpenCached(context.Background(), src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	_, err = s.WriteTo(&out)
	s.Close()
	if !errors.Is(err, ErrCachedStreamBroken) {
		t.Fatalf("expected the stream to break off, got %v", err)
	}
	if _, frags := parseServed(t, out.Bytes()); len(frags) != 3 {
		t.Fatalf("the fragments made before the failure are sent, got %d", len(frags))
	}
	f.idle(t)
	if stats := f.cache.Stats(); stats.Entries != 0 || stats.SizeBytes != 0 {
		t.Fatalf("a failed conversion must not stay in the cache: %+v", stats)
	}
	assertNoEntryFiles(t, f.dir)

	f.ff.mu.Lock()
	f.ff.failAfter = 0
	f.ff.mu.Unlock()
	start, frags := f.play(t, src, 0)
	checkContinuous(t, frags, start, 28)
	if f.ff.runCount() != 2 {
		t.Fatalf("runs %v", f.ff.runStarts())
	}
}

func TestCache_FailureBeforeFirstFragment(t *testing.T) {
	f := newCacheFixture(t, 2, 30)
	f.tc.SetRunFunc(func(ctx context.Context, args []string, w io.Writer, buf []byte) error {
		return errors.New("ffmpeg failed: invalid data")
	})
	_, err := f.tc.OpenCached(context.Background(), f.source(t, 7), 0)
	if !errors.Is(err, ErrCachedStreamBroken) {
		t.Fatalf("expected a failure, got %v", err)
	}
	f.idle(t)
	if f.tc.Stats().Active != 0 {
		t.Fatal("the slot must be released")
	}
	assertNoEntryFiles(t, f.dir)
}

func assertNoEntryFiles(t *testing.T, dir string) {
	t.Helper()
	items, _ := os.ReadDir(dir)
	for _, item := range items {
		if item.Name() != transcodeCacheMarker {
			t.Fatalf("unexpected file left in the cache: %s", item.Name())
		}
	}
}

func TestCache_InvalidatedWhenFileOrSettingsChange(t *testing.T) {
	f := newCacheFixture(t, 2, 10)
	src := f.source(t, 7)
	f.play(t, src, 0)
	f.idle(t)

	// The file changed: its conversion is dropped and made again
	changed := src
	changed.ModTime = src.ModTime.Add(time.Minute)
	if _, cached, _ := f.tc.CacheProbe(changed, 0); cached {
		t.Fatal("a changed file must not be served from the old conversion")
	}
	f.play(t, changed, 0)
	f.idle(t)
	if f.ff.runCount() != 2 {
		t.Fatalf("runs %v", f.ff.runStarts())
	}
	if stats := f.cache.Stats(); stats.Entries != 1 {
		t.Fatalf("the old conversion must be removed, stats %+v", stats)
	}

	// Other quality settings: same
	tc2 := NewTranscoder(config.TranscodeConfig{MaxConcurrent: 2, CRF: 30}, nil, zap.NewNop())
	tc2.SetCache(f.cache)
	tc2.SetRunFunc(f.ff.run)
	s, err := tc2.OpenCached(context.Background(), changed, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.WriteTo(io.Discard); err != nil {
		t.Fatal(err)
	}
	s.Close()
	f.idle(t)
	if f.ff.runCount() != 3 || f.cache.Stats().Entries != 1 {
		t.Fatalf("runs %v, stats %+v", f.ff.runStarts(), f.cache.Stats())
	}

	// A forced full transcode and a remux of the same scene are both kept
	remux := changed
	remux.Plan = DecidePlayback("/a.mkv", "h264", "aac")
	s, err = tc2.OpenCached(context.Background(), remux, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = s.WriteTo(io.Discard)
	s.Close()
	f.idle(t)
	if f.cache.Stats().Entries != 2 {
		t.Fatalf("stats %+v", f.cache.Stats())
	}
}

// completeEntries plays n different scenes so that the cache holds n complete entries,
// one second apart in last use.
func (f *cacheFixture) completeEntries(t *testing.T, n int) []TranscodeSource {
	t.Helper()
	var srcs []TranscodeSource
	for i := 0; i < n; i++ {
		src := f.source(t, uint(100+i))
		f.play(t, src, 0)
		f.idle(t)
		f.clock.Advance(time.Second)
		srcs = append(srcs, src)
	}
	return srcs
}

func TestCache_EvictsLeastRecentlyUsedOverSize(t *testing.T) {
	f := newCacheFixture(t, 2, 10)
	srcs := f.completeEntries(t, 3)
	entrySize := f.cache.Stats().SizeBytes / 3

	// Use the first one again: the second is now the least recently used
	f.play(t, srcs[0], 0)
	f.clock.Advance(time.Second)
	// Hold the third (being read): never evicted
	reader, err := f.tc.OpenCached(context.Background(), srcs[2], 0)
	if err != nil {
		t.Fatal(err)
	}

	f.cache.maxSize = entrySize // room for one entry
	f.cache.Cleanup()
	if _, cached, _ := f.tc.CacheProbe(srcs[1], 0); cached {
		t.Fatal("the least recently used entry must be evicted")
	}
	if _, cached, _ := f.tc.CacheProbe(srcs[2], 0); !cached {
		t.Fatal("an entry being read must never be evicted")
	}
	if _, cached, _ := f.tc.CacheProbe(srcs[0], 0); cached {
		t.Fatal("still over the size: the next least recently used goes too")
	}
	reader.Close()
	f.cache.Cleanup()
	if stats := f.cache.Stats(); stats.Entries != 1 || stats.SizeBytes != entrySize {
		t.Fatalf("stats %+v", stats)
	}
}

func TestCache_ExpiresByAge(t *testing.T) {
	f := newCacheFixture(t, 2, 10)
	srcs := f.completeEntries(t, 2)
	reader, err := f.tc.OpenCached(context.Background(), srcs[0], 0)
	if err != nil {
		t.Fatal(err)
	}
	f.clock.Advance(2 * time.Hour)
	f.cache.Cleanup()
	if _, cached, _ := f.tc.CacheProbe(srcs[1], 0); cached {
		t.Fatal("an entry unused for longer than the max age must be removed")
	}
	if _, cached, _ := f.tc.CacheProbe(srcs[0], 0); !cached {
		t.Fatal("an entry being read must never be removed")
	}
	reader.Close()
	f.clock.Advance(2 * time.Hour)
	f.cache.Cleanup()
	if f.cache.Stats().Entries != 0 {
		t.Fatalf("stats %+v", f.cache.Stats())
	}
	assertNoEntryFiles(t, f.dir)
}

func TestCache_ProducingEntryIsNeverEvicted(t *testing.T) {
	f := newCacheFixture(t, 2, 20)
	f.ff.gate = make(chan struct{})
	src := f.source(t, 7)
	s, err := f.tc.OpenCached(context.Background(), src, 0)
	if err != nil {
		t.Fatal(err)
	}
	s.Close() // nobody watches, but ffmpeg still runs (idle timeout)
	f.cache.idleTimeout = time.Hour
	f.cache.maxSize = 1
	f.clock.Advance(48 * time.Hour)
	f.cache.Cleanup()
	if f.cache.Stats().Entries != 1 {
		t.Fatal("an entry being written must not be evicted")
	}
	close(f.ff.gate)
	f.idle(t)
	f.cache.Cleanup()
	if f.cache.Stats().Entries != 0 {
		t.Fatalf("over size once finished: %+v", f.cache.Stats())
	}
}

func TestCache_ClearKeepsEntriesInUseUntilReleased(t *testing.T) {
	f := newCacheFixture(t, 2, 10)
	srcs := f.completeEntries(t, 2)
	reader, err := f.tc.OpenCached(context.Background(), srcs[0], 0)
	if err != nil {
		t.Fatal(err)
	}
	if n, size := f.tc.ClearCache(); n != 2 || size <= 0 {
		t.Fatalf("Clear = %d, %d", n, size)
	}
	if stats := f.cache.Stats(); stats.Entries != 0 {
		t.Fatalf("cleared entries are not offered anymore: %+v", stats)
	}
	// The viewer keeps watching what it opened
	var out bytes.Buffer
	if _, err := reader.WriteTo(&out); err != nil {
		t.Fatal(err)
	}
	_, frags := parseServed(t, out.Bytes())
	checkContinuous(t, frags, 0, 8)
	reader.Close()
	assertNoEntryFiles(t, f.dir)
}

func TestCache_PersistsAcrossRestarts(t *testing.T) {
	f := newCacheFixture(t, 2, 10)
	src := f.source(t, 7)
	f.play(t, src, 0)
	// A conversion cut by the shutdown (no metadata) and a foreign file
	if err := os.WriteFile(filepath.Join(f.dir, "7-0123456789abcdef-1.mp4"), testInitSegment(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.dir, "notes.txt"), []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.tc.Stop()

	f.open(t, 2)
	if _, err := os.Stat(filepath.Join(f.dir, "7-0123456789abcdef-1.mp4")); !os.IsNotExist(err) {
		t.Fatal("an unfinished entry must be deleted at startup")
	}
	if _, err := os.Stat(filepath.Join(f.dir, "notes.txt")); err != nil {
		t.Fatal("files the cache did not write are never deleted")
	}
	start, frags := f.play(t, src, 4)
	checkContinuous(t, frags, start, 8)
	if f.ff.runCount() != 1 {
		t.Fatalf("the cache must survive a restart, runs %v", f.ff.runStarts())
	}
}

func TestCache_ShutdownKeepsPartialEntry(t *testing.T) {
	f := newCacheFixture(t, 2, 30)
	f.cache.readAhead = 1
	f.cache.idleTimeout = time.Hour
	src := f.source(t, 7)
	f.playPart(t, src, 0, 3)
	f.tc.Stop()

	f.open(t, 2)
	if got, cached, _ := f.tc.CacheProbe(src, 2.5); !cached || math.Abs(got-1.99) > 1e-6 {
		t.Fatalf("the part converted before the shutdown is kept, got %v %v", got, cached)
	}
}

func TestCache_RefusesForeignDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "movie.mp4"), []byte("media"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewTranscodeCache(testCacheConfig(dir), zap.NewNop()); err == nil {
		t.Fatal("a non-empty directory that is not a cache must be refused")
	}
	if _, err := os.Stat(filepath.Join(dir, "movie.mp4")); err != nil {
		t.Fatal("the file must be untouched")
	}
	disabled, err := NewTranscodeCache(config.TranscodeCacheConfig{Enabled: false, Dir: dir}, zap.NewNop())
	if disabled != nil || err != nil {
		t.Fatalf("disabled cache: %v %v", disabled, err)
	}
}

func TestCache_SlotsAndIdleConversions(t *testing.T) {
	f := newCacheFixture(t, 1, 20)
	f.ff.gate = make(chan struct{})
	f.cache.idleTimeout = time.Hour
	a, b := f.source(t, 1), f.source(t, 2)

	sa, err := f.tc.OpenCached(context.Background(), a, 0)
	if err != nil {
		t.Fatal(err)
	}
	// The only slot is used by a watched conversion
	if _, err := f.tc.OpenCached(context.Background(), b, 0); !errors.Is(err, ErrTranscodeLimit) {
		t.Fatalf("expected ErrTranscodeLimit, got %v", err)
	}
	if f.tc.Available("other") {
		t.Fatal("no slot is available")
	}
	// Its viewer leaves: the conversion idles and gives its slot to the next one
	sa.Close()
	if !f.tc.Available("other") {
		t.Fatal("an idle conversion frees its slot on demand")
	}
	sb, err := f.tc.OpenCached(context.Background(), b, 0)
	if err != nil {
		t.Fatalf("expected the idle conversion to be stopped for b: %v", err)
	}
	defer sb.Close()
	if f.tc.Stats().Active != 1 {
		t.Fatalf("active %d", f.tc.Stats().Active)
	}
	// a was stopped after one fragment: nothing usable is kept
	if _, cached, _ := f.tc.CacheProbe(a, 0); cached {
		t.Fatal("a partial entry with one fragment has nothing usable")
	}
}

func TestCache_SeekBackIntoRunningConversion(t *testing.T) {
	f := newCacheFixture(t, 1, 40)
	f.cache.readAhead = 6
	f.cache.idleTimeout = time.Hour
	src := f.source(t, 7)

	// Watching from 0: stops after 5 fragments, the conversion idles a little ahead
	f.playPart(t, src, 0, 5)
	// Seek back to 4 while it is still running: no new ffmpeg, the same run goes on
	start, frags := f.play(t, src, 4)
	checkContinuous(t, frags, start, 38)
	if f.ff.runCount() != 1 {
		t.Fatalf("runs %v", f.ff.runStarts())
	}
}
