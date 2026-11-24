package handler

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"goonhub/internal/config"
	"goonhub/internal/streaming"

	"go.uber.org/zap"
)

// Minimal fragmented MP4 like ffmpeg writes: init segment then one fragment every 2 s.

func fmp4Box(typ string, payload ...[]byte) []byte {
	size := 8
	for _, p := range payload {
		size += len(p)
	}
	b := binary.BigEndian.AppendUint32(make([]byte, 0, size), uint32(size))
	b = append(b, typ...)
	for _, p := range payload {
		b = append(b, p...)
	}
	return b
}

func be32(v uint32) []byte { return binary.BigEndian.AppendUint32(nil, v) }

func fakeInitSegment() []byte {
	tkhd := append(make([]byte, 12), be32(1)...)
	tkhd = append(tkhd, make([]byte, 64)...)
	mdhd := append(make([]byte, 12), be32(1000)...)
	mdhd = append(mdhd, make([]byte, 8)...)
	hdlr := append(make([]byte, 8), []byte("vide")...)
	hdlr = append(hdlr, make([]byte, 13)...)
	trak := fmp4Box("trak", fmp4Box("tkhd", tkhd), fmp4Box("mdia", fmp4Box("mdhd", mdhd), fmp4Box("hdlr", hdlr)))
	return append(fmp4Box("ftyp", []byte("isom"), be32(512)), fmp4Box("moov", trak)...)
}

func fakeFragment(seq uint32, out float64) []byte {
	traf := fmp4Box("traf",
		fmp4Box("tfhd", be32(0x020000), be32(1)),
		fmp4Box("tfdt", be32(0x01000000), binary.BigEndian.AppendUint64(nil, uint64(out*1000))),
	)
	moof := fmp4Box("moof", fmp4Box("mfhd", be32(0), be32(seq)), traf)
	return append(moof, fmp4Box("mdat", []byte("frame"))...)
}

type cachedTranscodeFixture struct {
	*transcodeFixture
	mu   sync.Mutex
	runs []float64
}

func newCachedTranscodeFixture(t *testing.T, maxConcurrent int, duration float64) *cachedTranscodeFixture {
	t.Helper()
	f := &cachedTranscodeFixture{transcodeFixture: newTranscodeFixture(t, maxConcurrent)}
	cache, err := streaming.NewTranscodeCache(config.TranscodeCacheConfig{
		Enabled: true, Dir: filepath.Join(t.TempDir(), "cache"), MaxSizeGB: 1, MaxAge: time.Hour,
		CleanupInterval: time.Hour, ReadAhead: time.Hour, IdleTimeout: 20 * time.Millisecond,
	}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	f.transcoder.SetCache(cache)
	f.transcoder.SetRunFunc(func(ctx context.Context, args []string, w io.Writer, buf []byte) error {
		start := 0.0
		if i := slices.Index(args, "-ss"); i >= 0 {
			start, _ = strconv.ParseFloat(args[i+1], 64)
		}
		f.mu.Lock()
		f.runs = append(f.runs, start)
		f.lastArgs = args
		f.mu.Unlock()
		if _, err := w.Write(fakeInitSegment()); err != nil {
			return err
		}
		for k := 0; start+float64(k)*2 < duration; k++ {
			if _, err := w.Write(fakeFragment(uint32(k+1), float64(k)*2)); err != nil {
				return err
			}
		}
		return nil
	})
	f.router.GET("/api/v1/admin/transcode-cache", f.handler.GetTranscodeCache)
	f.router.DELETE("/api/v1/admin/transcode-cache", f.handler.ClearTranscodeCache)
	return f
}

func (f *cachedTranscodeFixture) runCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.runs)
}

func (f *cachedTranscodeFixture) waitIdle(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for f.transcoder.CacheStats().InProgress > 0 {
		if time.Now().After(deadline) {
			t.Fatal("conversion still running")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestTranscodeScene_CachedReplay(t *testing.T) {
	f := newCachedTranscodeFixture(t, 2, 20)
	f.enabled(true)
	f.scene(t, "a.avi", "mpeg4", "mp3", 20)

	first := f.do("GET", "/api/v1/scenes/7/transcode?sid=a")
	if first.Code != http.StatusOK || first.Header().Get("Content-Type") != "video/mp4" {
		t.Fatalf("got %d %q", first.Code, first.Header().Get("Content-Type"))
	}
	if string(first.Body.Bytes()[4:8]) != "ftyp" {
		t.Fatal("the stream must start with the init segment")
	}
	f.waitIdle(t)

	// Replay by another viewer and seek back: no new ffmpeg
	second := f.do("GET", "/api/v1/scenes/7/transcode?sid=b")
	if second.Code != http.StatusOK || second.Body.Len() != first.Body.Len() {
		t.Fatalf("replay: %d, %d bytes (first %d)", second.Code, second.Body.Len(), first.Body.Len())
	}
	head := f.do("HEAD", "/api/v1/scenes/7/transcode?sid=b&start=9.3")
	if head.Code != http.StatusOK || head.Header().Get("X-Transcode-Start") != "8.000" {
		t.Fatalf("HEAD: %d, start %q", head.Code, head.Header().Get("X-Transcode-Start"))
	}
	seek := f.do("GET", "/api/v1/scenes/7/transcode?sid=b&start=8.000")
	if seek.Code != http.StatusOK || seek.Header().Get("X-Transcode-Start") != "8.000" {
		t.Fatalf("seek: %d, start %q", seek.Code, seek.Header().Get("X-Transcode-Start"))
	}
	if n := f.runCount(); n != 1 {
		t.Fatalf("expected one ffmpeg run, got %v", f.runs)
	}

	// The cached part is served even when all slots are used
	lease, err := f.transcoder.Acquire(context.Background(), "someone-else")
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()
	lease2, err := f.transcoder.Acquire(context.Background(), "someone-else-2")
	if err != nil {
		t.Fatal(err)
	}
	defer lease2.Release()
	if head := f.do("HEAD", "/api/v1/scenes/7/transcode?sid=c&start=4"); head.Code != http.StatusOK {
		t.Fatalf("HEAD of a cached part with no free slot: %d", head.Code)
	}
	if got := f.do("GET", "/api/v1/scenes/7/transcode?sid=c&start=4"); got.Code != http.StatusOK {
		t.Fatalf("GET of a cached part with no free slot: %d", got.Code)
	}
}

func TestTranscodeScene_CachedHeadAlignsTranscodeStart(t *testing.T) {
	f := newCachedTranscodeFixture(t, 2, 600)
	f.enabled(true)
	f.scene(t, "a.avi", "mpeg4", "mp3", 600)

	head := f.do("HEAD", "/api/v1/scenes/7/transcode?sid=a&start=125.5")
	if head.Code != http.StatusOK || head.Header().Get("X-Transcode-Start") != "124.000" {
		t.Fatalf("HEAD: %d, start %q (want the keyframe grid point 124)", head.Code, head.Header().Get("X-Transcode-Start"))
	}
	got := f.do("GET", "/api/v1/scenes/7/transcode?sid=a&start=124.000")
	if got.Code != http.StatusOK {
		t.Fatalf("GET: %d", got.Code)
	}
	// The conversion keeps writing the cache after the response: let it end before the
	// TempDir cleanup.
	f.waitIdle(t)
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.runs) != 1 || f.runs[0] != 124 {
		t.Fatalf("runs %v", f.runs)
	}
}

func TestTranscodeScene_CachedLimitReached(t *testing.T) {
	f := newCachedTranscodeFixture(t, 1, 20)
	f.enabled(true)
	f.scene(t, "a.avi", "mpeg4", "mp3", 20)
	lease, err := f.transcoder.Acquire(context.Background(), "someone-else")
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()

	w := f.do("GET", "/api/v1/scenes/7/transcode?sid=abc")
	if w.Code != http.StatusServiceUnavailable || errorCode(t, w) != codeTranscodeLimit {
		t.Fatalf("expected 503 %s, got %d %s", codeTranscodeLimit, w.Code, w.Body.String())
	}
	if head := f.do("HEAD", "/api/v1/scenes/7/transcode?sid=abc"); head.Code != http.StatusServiceUnavailable {
		t.Fatalf("HEAD: expected 503, got %d", head.Code)
	}
}

func TestTranscodeScene_CachedFailureBeforeOutput(t *testing.T) {
	f := newCachedTranscodeFixture(t, 2, 20)
	f.enabled(true)
	f.scene(t, "a.avi", "mpeg4", "mp3", 20)
	f.transcoder.SetRunFunc(func(ctx context.Context, args []string, w io.Writer, buf []byte) error {
		return io.ErrUnexpectedEOF
	})
	w := f.do("GET", "/api/v1/scenes/7/transcode?sid=abc")
	if w.Code != http.StatusInternalServerError || errorCode(t, w) != codeTranscodeFailed {
		t.Fatalf("expected 500 %s, got %d %s", codeTranscodeFailed, w.Code, w.Body.String())
	}
}

func TestTranscodeCacheAdmin(t *testing.T) {
	f := newCachedTranscodeFixture(t, 2, 20)
	f.enabled(true)
	f.scene(t, "a.avi", "mpeg4", "mp3", 20)
	if w := f.do("GET", "/api/v1/scenes/7/transcode?sid=a"); w.Code != http.StatusOK {
		t.Fatalf("GET: %d", w.Code)
	}
	f.waitIdle(t)

	var stats streaming.TranscodeCacheStats
	w := f.do("GET", "/api/v1/admin/transcode-cache")
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil || w.Code != http.StatusOK {
		t.Fatalf("stats: %d %s", w.Code, w.Body.String())
	}
	if !stats.Enabled || stats.Entries != 1 || stats.SizeBytes <= 0 || stats.MaxSizeBytes != 1<<30 {
		t.Fatalf("stats %+v", stats)
	}

	var cleared transcodeCacheClearResponse
	w = f.do("DELETE", "/api/v1/admin/transcode-cache")
	if err := json.Unmarshal(w.Body.Bytes(), &cleared); err != nil || w.Code != http.StatusOK {
		t.Fatalf("clear: %d %s", w.Code, w.Body.String())
	}
	if cleared.Removed != 1 || cleared.RemovedBytes != stats.SizeBytes || cleared.Entries != 0 || cleared.SizeBytes != 0 {
		t.Fatalf("clear %+v", cleared)
	}
	if w := f.do("GET", "/api/v1/scenes/7/transcode?sid=a"); w.Code != http.StatusOK || f.runCount() != 2 {
		t.Fatalf("after clear the scene is converted again: %d, runs %d", w.Code, f.runCount())
	}
}

func TestTranscodeCacheAdmin_Disabled(t *testing.T) {
	f := newTranscodeFixture(t, 2)
	f.router.GET("/api/v1/admin/transcode-cache", f.handler.GetTranscodeCache)
	w := f.do("GET", "/api/v1/admin/transcode-cache")
	var stats streaming.TranscodeCacheStats
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil || w.Code != http.StatusOK || stats.Enabled {
		t.Fatalf("got %d %s", w.Code, w.Body.String())
	}
}
