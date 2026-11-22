package streaming

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"goonhub/internal/config"

	"go.uber.org/zap"
)

// These tests run the real ffmpeg through the cache (skipped when it is not installed):
// a stream made of several cached conversions must play like a single one.

type realCacheFixture struct {
	tc    *Transcoder
	cache *TranscodeCache
	runs  atomic.Int32
	input string
}

func newRealCacheFixture(t *testing.T, input string) *realCacheFixture {
	t.Helper()
	cache, err := NewTranscodeCache(testCacheConfig(filepath.Join(t.TempDir(), "cache")), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	f := &realCacheFixture{cache: cache, input: input}
	f.tc = NewTranscoder(config.TranscodeConfig{MaxConcurrent: 2, Preset: "ultrafast", MaxHeight: 240}, nil, zap.NewNop())
	f.tc.SetCache(cache)
	f.tc.SetRunFunc(func(ctx context.Context, args []string, w io.Writer, buf []byte) error {
		f.runs.Add(1)
		return runFFmpeg(ctx, args, w, buf)
	})
	t.Cleanup(f.tc.Stop)
	return f
}

func (f *realCacheFixture) source(t *testing.T, plan PlaybackPlan) TranscodeSource {
	t.Helper()
	info, err := os.Stat(f.input)
	if err != nil {
		t.Fatal(err)
	}
	return TranscodeSource{SceneID: 1, Path: f.input, Size: info.Size(), ModTime: info.ModTime(), Plan: plan}
}

// play streams from start; with frags > 0 the viewer leaves after that many fragments.
func (f *realCacheFixture) play(t *testing.T, src TranscodeSource, start float64, frags int) (float64, string) {
	t.Helper()
	s, err := f.tc.OpenCached(context.Background(), src, start)
	if err != nil {
		t.Fatalf("OpenCached(%v): %v", start, err)
	}
	defer s.Close()
	out := filepath.Join(t.TempDir(), "out.mp4")
	if frags > 0 {
		w := &cutWriter{n: frags + 1}
		_, _ = s.WriteTo(w)
		return s.Start(), ""
	}
	var buf bytes.Buffer
	if _, err := s.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	if err := os.WriteFile(out, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return s.Start(), out
}

func (f *realCacheFixture) idle(t *testing.T) {
	t.Helper()
	waitFor(t, "the conversions to stop", func() bool { return f.cache.Stats().InProgress == 0 })
}

// checkPlayable decodes the stream (no error allowed) and checks that it covers want
// seconds with no hole in the audio, and none longer than maxVideoHole in the video.
func checkPlayable(t *testing.T, path string, want, maxVideoHole float64) {
	t.Helper()
	var stderr bytes.Buffer
	cmd := exec.Command("ffmpeg", "-nostdin", "-v", "error", "-i", path, "-f", "null", "-")
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil || strings.TrimSpace(stderr.String()) != "" {
		t.Fatalf("the stream does not decode cleanly: %v: %s", err, stderr.String())
	}
	for _, stream := range []string{"v", "a"} {
		out, err := exec.Command("ffprobe", "-v", "error", "-select_streams", stream,
			"-show_entries", "packet=pts_time,duration_time", "-of", "json", path).Output()
		if err != nil {
			t.Fatalf("ffprobe: %v", err)
		}
		var res struct {
			Packets []struct {
				Pts      string `json:"pts_time"`
				Duration string `json:"duration_time"`
			} `json:"packets"`
		}
		if err := json.Unmarshal(out, &res); err != nil {
			t.Fatal(err)
		}
		var pts []float64
		for _, p := range res.Packets {
			v, _ := strconv.ParseFloat(p.Pts, 64)
			pts = append(pts, v)
		}
		if len(pts) < 10 {
			t.Fatalf("%s: only %d packets", stream, len(pts))
		}
		sort.Float64s(pts)
		maxHole := 0.1
		if stream == "v" {
			maxHole = maxVideoHole
		}
		for i := 1; i < len(pts); i++ {
			if gap := pts[i] - pts[i-1]; gap > maxHole {
				t.Fatalf("%s: hole of %.3fs at %.3f", stream, gap, pts[i-1])
			}
			if pts[i] == pts[i-1] {
				t.Fatalf("%s: two packets at %.3f (overlap)", stream, pts[i])
			}
		}
		if span := pts[len(pts)-1] - pts[0]; math.Abs(span-want) > 0.6 {
			t.Fatalf("%s: covers %.2fs, want %.2fs", stream, span, want)
		}
	}
}

func testCachedJoins(t *testing.T, input string, plan PlaybackPlan, duration, maxVideoHole float64) {
	f := newRealCacheFixture(t, input)
	src := f.source(t, plan)

	// Watch the start then leave: the conversion stops a little ahead
	f.cache.readAhead = 4
	f.play(t, src, 0, 3)
	f.idle(t)
	if n := f.runs.Load(); n != 1 {
		t.Fatalf("runs %d", n)
	}

	// From the start to the end: the cached part, then a second ffmpeg from where it ends
	f.cache.readAhead = 3600
	start, out := f.play(t, src, 0, 0)
	if start != 0 {
		t.Fatalf("start %v", start)
	}
	checkPlayable(t, out, duration, maxVideoHole)
	if n := f.runs.Load(); n != 2 {
		t.Fatalf("expected the second run to continue the first one, runs %d", n)
	}
	f.idle(t)

	// Replay and seek back: no ffmpeg
	_, out = f.play(t, src, 0, 0)
	checkPlayable(t, out, duration, maxVideoHole)
	realStart, cached, _ := f.tc.CacheProbe(src, 7.5)
	if !cached || realStart > 7.5 || realStart < 3 {
		t.Fatalf("CacheProbe(7.5) = %v, %v", realStart, cached)
	}
	start, out = f.play(t, src, realStart, 0)
	checkPlayable(t, out, duration-start, maxVideoHole)
	if n := f.runs.Load(); n != 2 {
		t.Fatalf("replays must come from the cache, runs %d", n)
	}
}

func TestTranscodeCacheFFmpeg_TranscodeJoins(t *testing.T) {
	requireFFmpeg(t)
	// HEVC in MKV, AAC copied: ffmpeg seeks it exactly, the second run follows seamlessly
	input := makeClip(t, "hevc.mkv", 20, "-c:v", "libx265", "-preset", "ultrafast", "-x265-params", "log-level=error", "-c:a", "aac")
	plan := DecidePlayback(input, "hevc", "aac")
	if plan.Mode != PlaybackTranscode || !plan.CopyAudio {
		t.Fatalf("plan %+v", plan)
	}
	testCachedJoins(t, input, plan, 20, 0.1)
}

func TestTranscodeCacheFFmpeg_TranscodeJoinsMPEGPS(t *testing.T) {
	requireFFmpeg(t)
	// ffmpeg seeks MPEG-PS up to a GOP late (12 frames here): where the second run starts,
	// its first frame is held a little longer (as at the start of any conversion of these
	// files)
	input := makeClip(t, "clip.mpg", 20, "-c:v", "mpeg2video", "-q:v", "5", "-c:a", "mp2")
	testCachedJoins(t, input, DecidePlayback(input, "mpeg2video", "mp2"), 20, 0.5)
}

func TestTranscodeCacheFFmpeg_RemuxJoins(t *testing.T) {
	requireFFmpeg(t)
	// H.264 with B-frames, a keyframe every second, in MKV
	input := makeClip(t, "clip.mkv", 20, "-c:v", "libx264", "-preset", "ultrafast", "-bf", "2", "-g", "25", "-c:a", "aac")
	plan := DecidePlayback(input, "h264", "aac")
	if plan.Mode != PlaybackRemux {
		t.Fatalf("plan %+v", plan)
	}
	testCachedJoins(t, input, plan, 20, 0.1)
}

func TestTranscodeCacheFFmpeg_RemuxADTSAudio(t *testing.T) {
	requireFFmpeg(t)
	input := makeClip(t, "playlist.mkv", 12, "-c:v", "libx264", "-preset", "ultrafast", "-g", "25", "-c:a", "aac", "-f", "mpegts")
	testCachedJoins(t, input, DecidePlayback(input, "h264", "aac"), 12, 0.1)
}

func TestTranscodeCacheFFmpeg_SecondViewer(t *testing.T) {
	requireFFmpeg(t)
	input := makeClip(t, "clip.avi", 10, "-c:v", "mpeg4", "-c:a", "mp3")
	f := newRealCacheFixture(t, input)
	src := f.source(t, DecidePlayback(input, "mpeg4", "mp3"))

	first, err := f.tc.OpenCached(context.Background(), src, 0)
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.tc.OpenCached(context.Background(), src, 0)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	var b1 bytes.Buffer
	go func() {
		_, err := first.WriteTo(&b1)
		done <- err
	}()
	var b2 bytes.Buffer
	if _, err := second.WriteTo(&b2); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("first viewer did not finish")
	}
	first.Close()
	second.Close()
	if !bytes.Equal(b1.Bytes(), b2.Bytes()) {
		t.Fatal("both viewers must get the same stream")
	}
	out := filepath.Join(t.TempDir(), "out.mp4")
	if err := os.WriteFile(out, b1.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	checkPlayable(t, out, 10, 0.1)
	if n := f.runs.Load(); n != 1 {
		t.Fatalf("one ffmpeg for both viewers, got %d", n)
	}
}
