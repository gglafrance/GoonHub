package streaming

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"goonhub/internal/config"

	"go.uber.org/zap"
)

// These tests run the real ffmpeg/ffprobe (skipped when they are not installed): they
// generate small clips browsers can't play, convert them and check the result with ffprobe.

func requireFFmpeg(t *testing.T) {
	t.Helper()
	for _, bin := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not installed", bin)
		}
	}
}

// makeClip generates a test clip with ffmpeg (testsrc video + sine audio).
func makeClip(t *testing.T, name string, seconds int, codecArgs ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc=size=640x360:rate=25:duration=" + strconv.Itoa(seconds),
		"-f", "lavfi", "-i", "sine=frequency=440:duration=" + strconv.Itoa(seconds),
		"-shortest"}
	args = append(args, codecArgs...)
	args = append(args, "-y", path)
	if out, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
		t.Fatalf("failed to generate %s: %v: %s", name, err, out)
	}
	return path
}

type probeResult struct {
	Streams []struct {
		CodecType string `json:"codec_type"`
		CodecName string `json:"codec_name"`
		Height    int    `json:"height"`
		PixFmt    string `json:"pix_fmt"`
	} `json:"streams"`
	Format struct {
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
	} `json:"format"`
}

func probe(t *testing.T, path string) probeResult {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", "-show_format", path).Output()
	if err != nil {
		t.Fatalf("ffprobe %s: %v", path, err)
	}
	var res probeResult
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("ffprobe output: %v", err)
	}
	return res
}

func streamCodecs(res probeResult) (video, audio string, height int, pixFmt string) {
	for _, s := range res.Streams {
		switch s.CodecType {
		case "video":
			video, height, pixFmt = s.CodecName, s.Height, s.PixFmt
		case "audio":
			audio = s.CodecName
		}
	}
	return
}

func transcodeToFile(t *testing.T, tc *Transcoder, input string, plan PlaybackPlan, start float64) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "out.mp4")
	f, err := os.Create(out)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer f.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := tc.Stream(ctx, f, tc.Args(input, plan, start), nil); err != nil {
		t.Fatalf("stream: %v", err)
	}
	return out
}

func TestTranscodeFFmpeg_MPEG2(t *testing.T) {
	requireFFmpeg(t)
	input := makeClip(t, "clip.mpg", 6, "-c:v", "mpeg2video", "-q:v", "5", "-c:a", "mp2")
	tc := NewTranscoder(config.TranscodeConfig{MaxHeight: 240, Preset: "ultrafast"}, nil, zap.NewNop())
	t.Cleanup(tc.Stop)

	plan := DecidePlayback(input, "mpeg2video", "mp2")
	if plan.Mode != PlaybackTranscode {
		t.Fatalf("mode %q", plan.Mode)
	}
	res := probe(t, transcodeToFile(t, tc, input, plan, 2))

	video, audio, height, pixFmt := streamCodecs(res)
	if video != "h264" || audio != "aac" || pixFmt != "yuv420p" {
		t.Fatalf("got video %q (%s) audio %q", video, pixFmt, audio)
	}
	if height != 240 {
		t.Fatalf("expected downscale to 240p, got %d", height)
	}
	if duration, _ := strconv.ParseFloat(res.Format.Duration, 64); duration < 3.5 || duration > 4.5 {
		t.Fatalf("expected about 4s (6s from 2s), got %q", res.Format.Duration)
	}
}

func TestTranscodeFFmpeg_HEVC(t *testing.T) {
	requireFFmpeg(t)
	input := makeClip(t, "clip.mp4", 3, "-c:v", "libx265", "-preset", "ultrafast", "-x265-params", "log-level=error", "-c:a", "aac")
	tc := NewTranscoder(config.TranscodeConfig{Preset: "ultrafast"}, nil, zap.NewNop())
	t.Cleanup(tc.Stop)

	plan := DecidePlayback(input, "hevc", "aac")
	if plan.Mode != PlaybackTranscode || !plan.CopyAudio {
		t.Fatalf("plan %+v", plan)
	}
	video, audio, height, _ := streamCodecs(probe(t, transcodeToFile(t, tc, input, plan, 0)))
	if video != "h264" || audio != "aac" || height != 360 {
		t.Fatalf("got video %q audio %q height %d", video, audio, height)
	}
}

func TestTranscodeFFmpeg_RemuxMKV(t *testing.T) {
	requireFFmpeg(t)
	input := makeClip(t, "clip.mkv", 4, "-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac")
	tc := NewTranscoder(config.TranscodeConfig{}, nil, zap.NewNop())
	t.Cleanup(tc.Stop)

	plan := DecidePlayback(input, "h264", "aac")
	if plan.Mode != PlaybackRemux {
		t.Fatalf("mode %q", plan.Mode)
	}
	res := probe(t, transcodeToFile(t, tc, input, plan, 0))
	video, audio, _, _ := streamCodecs(res)
	if video != "h264" || audio != "aac" {
		t.Fatalf("got video %q audio %q", video, audio)
	}
	if res.Format.FormatName == "" || !bytes.Contains([]byte(res.Format.FormatName), []byte("mp4")) {
		t.Fatalf("expected an mp4, got %q", res.Format.FormatName)
	}
}

// An HLS download saved with a .mkv name is really MPEG-TS with ADTS AAC: without the
// aac_adtstoasc filter the MP4 muxer gave up after the first second, and the player looped
// on that second.
func TestTranscodeFFmpeg_RemuxADTSAudio(t *testing.T) {
	requireFFmpeg(t)
	input := makeClip(t, "playlist.mkv", 6, "-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac", "-f", "mpegts")
	tc := NewTranscoder(config.TranscodeConfig{}, nil, zap.NewNop())
	t.Cleanup(tc.Stop)

	plan := DecidePlayback(input, "h264", "aac")
	if plan.Mode != PlaybackRemux || !plan.CopyAudio {
		t.Fatalf("expected a remux copying the audio, got %+v", plan)
	}
	res := probe(t, transcodeToFile(t, tc, input, plan, 0))
	video, audio, _, _ := streamCodecs(res)
	if video != "h264" || audio != "aac" {
		t.Fatalf("got video %q audio %q", video, audio)
	}
	duration, err := strconv.ParseFloat(res.Format.Duration, 64)
	if err != nil || duration < 5 {
		t.Fatalf("the remux stopped early: duration %q", res.Format.Duration)
	}
}

func TestTranscodeFFmpeg_RemuxKeyframeStart(t *testing.T) {
	requireFFmpeg(t)
	// One keyframe every 5 s
	input := makeClip(t, "gop.mkv", 30, "-c:v", "libx264", "-preset", "ultrafast", "-g", "125", "-keyint_min", "125", "-sc_threshold", "0", "-c:a", "aac")
	tc := NewTranscoder(config.TranscodeConfig{}, nil, zap.NewNop())
	t.Cleanup(tc.Stop)

	start := tc.KeyframeStart(context.Background(), input, 17)
	if start < 14.9 || start > 15.1 {
		t.Fatalf("expected the keyframe at 15s, got %v", start)
	}
	// The copy starting there lasts exactly until the end
	res := probe(t, transcodeToFile(t, tc, input, DecidePlayback(input, "h264", "aac"), start))
	if duration, _ := strconv.ParseFloat(res.Format.Duration, 64); duration < 14.8 || duration > 15.2 {
		t.Fatalf("expected about 15s (30s from 15s), got %q", res.Format.Duration)
	}
}

// failingWriter accepts the first write then fails, like a client that went away.
type failingWriter struct{ writes int }

func (w *failingWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes > 1 {
		return 0, errors.New("client gone")
	}
	return len(p), nil
}

// cancelOnWrite cancels the request context on the first write.
type cancelOnWrite struct{ cancel context.CancelFunc }

func (w *cancelOnWrite) Write(p []byte) (int, error) {
	w.cancel()
	return len(p), nil
}

func TestTranscodeFFmpeg_StopsWhenClientGoes(t *testing.T) {
	requireFFmpeg(t)
	input := makeClip(t, "long.mpg", 60, "-c:v", "mpeg2video", "-c:a", "mp2")
	tc := NewTranscoder(config.TranscodeConfig{Preset: "ultrafast"}, nil, zap.NewNop())
	t.Cleanup(tc.Stop)

	// Client write failure
	startedAt := time.Now()
	err := tc.Stream(context.Background(), &failingWriter{}, tc.Args(input, FullTranscodePlan(), 0), make([]byte, 4096))
	if err == nil {
		t.Fatal("expected an error when the client goes away")
	}
	if time.Since(startedAt) > 20*time.Second {
		t.Fatal("ffmpeg was not stopped when the client went away")
	}

	// Request context cancelled (client disconnect) once the stream has started
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startedAt = time.Now()
	err = tc.Stream(ctx, &cancelOnWrite{cancel: cancel}, tc.Args(input, FullTranscodePlan(), 0), make([]byte, 4096))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if time.Since(startedAt) > 10*time.Second {
		t.Fatal("ffmpeg was not killed on cancel")
	}
}

func TestTranscodeFFmpeg_BadInput(t *testing.T) {
	requireFFmpeg(t)
	input := filepath.Join(t.TempDir(), "broken.avi")
	if err := os.WriteFile(input, []byte("not a video"), 0o644); err != nil {
		t.Fatal(err)
	}
	tc := NewTranscoder(config.TranscodeConfig{}, nil, zap.NewNop())
	t.Cleanup(tc.Stop)
	var out bytes.Buffer
	err := tc.Stream(context.Background(), &out, tc.Args(input, FullTranscodePlan(), 0), nil)
	if err == nil || out.Len() != 0 {
		t.Fatalf("expected an ffmpeg error and no output, got %v (%d bytes)", err, out.Len())
	}
}
