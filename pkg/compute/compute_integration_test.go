package compute

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"goonhub/pkg/chromaprint"
	"goonhub/pkg/ffmpeg"
	"goonhub/pkg/workerproto"
)

// These tests run real ffmpeg on clips rendered with lavfi. They skip when ffmpeg/ffprobe
// are not on PATH (the golang image has none: install ffmpeg, and libchromaprint-tools for
// the audio fingerprint).

func requireFFmpeg(t *testing.T) {
	t.Helper()
	if err := ffmpeg.CheckInstallation(); err != nil {
		t.Skip("ffmpeg/ffprobe not on PATH: install ffmpeg to run the compute integration tests")
	}
}

// makeTestClip renders a synthetic video (optionally with a tone) of the given length.
func makeTestClip(t *testing.T, path string, seconds int, withAudio bool) {
	t.Helper()
	args := []string{"-f", "lavfi", "-i", fmt.Sprintf("testsrc=duration=%d:size=320x240:rate=25", seconds)}
	if withAudio {
		args = append(args, "-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=440:duration=%d", seconds), "-c:a", "aac")
	}
	args = append(args, "-c:v", "libx264", "-pix_fmt", "yuv420p", "-y", path)
	if out, err := exec.Command(ffmpeg.FFMpegPath(), args...).CombinedOutput(); err != nil {
		t.Fatalf("failed to create test clip: %v, output: %s", err, out)
	}
}

// countStreams returns how many streams of the given type ("a" or "v") the file has.
func countStreams(t *testing.T, path, kind string) int {
	t.Helper()
	out, err := exec.Command(ffmpeg.FFprobePath(), "-v", "error", "-select_streams", kind,
		"-show_entries", "stream=codec_type", "-of", "csv=p=0", path).Output()
	if err != nil {
		t.Fatalf("ffprobe failed on %s: %v", path, err)
	}
	return len(strings.Fields(string(out)))
}

var (
	clipsOnce sync.Once
	clipsDir  string
	clipsErr  error
)

// sharedClips renders the test clips once per test binary: a 30 s clip with audio and a
// silent one.
func sharedClips(t *testing.T) (withAudio, silent string) {
	t.Helper()
	requireFFmpeg(t)
	clipsOnce.Do(func() {
		clipsDir, clipsErr = os.MkdirTemp("", "goonhub-compute-test-*")
		if clipsErr != nil {
			return
		}
		makeTestClip(t, filepath.Join(clipsDir, "with_audio.mp4"), 30, true)
		makeTestClip(t, filepath.Join(clipsDir, "silent.mp4"), 30, false)
	})
	if clipsErr != nil {
		t.Fatalf("failed to prepare clips: %v", clipsErr)
	}
	return filepath.Join(clipsDir, "with_audio.mp4"), filepath.Join(clipsDir, "silent.mp4")
}

func TestMain(m *testing.M) {
	code := m.Run()
	if clipsDir != "" {
		os.RemoveAll(clipsDir)
	}
	os.Exit(code)
}

func TestSprites_Integration(t *testing.T) {
	_, silent := sharedClips(t)
	out := t.TempDir()

	var mu sync.Mutex
	var progress []int
	res, err := Sprites(context.Background(), Env{}, Request{
		Input:  silent,
		Video:  workerproto.VideoInfo{DurationSec: 30, Width: 320, Height: 240},
		OutDir: out,
		Progress: func(p int) {
			mu.Lock()
			progress = append(progress, p)
			mu.Unlock()
		},
	}, workerproto.SpritesParams{
		SceneID: 42, TileWidth: 160, TileHeight: 120, FrameInterval: 2, FrameQuality: 75,
		GridCols: 4, GridRows: 2, Concurrency: 4,
	})
	if err != nil {
		t.Fatalf("Sprites failed: %v", err)
	}

	// 30 s / 2 s = 15 frames, 8 per sheet = 2 sheets
	wantSheets := []string{"42_sheet_001.webp", "42_sheet_002.webp"}
	if fmt.Sprint(res.SpriteSheets) != fmt.Sprint(wantSheets) || res.SheetCount != 2 {
		t.Fatalf("sheets = %v (count %d), want %v", res.SpriteSheets, res.SheetCount, wantSheets)
	}
	if res.VTT != "42_thumbnails.vtt" || res.FrameInterval != 2 {
		t.Fatalf("vtt = %q interval = %d", res.VTT, res.FrameInterval)
	}
	if err := res.Validate(42); err != nil {
		t.Fatalf("result does not validate: %v", err)
	}
	for _, name := range append(wantSheets, res.VTT) {
		if st, err := os.Stat(filepath.Join(out, name)); err != nil || st.Size() == 0 {
			t.Fatalf("output %s missing or empty: %v", name, err)
		}
	}
	vtt, err := os.ReadFile(filepath.Join(out, res.VTT))
	if err != nil {
		t.Fatalf("read vtt: %v", err)
	}
	if !strings.HasPrefix(string(vtt), "WEBVTT") || !strings.Contains(string(vtt), "/sprites/42_sheet_002.webp#xywh=") {
		t.Fatalf("unexpected VTT:\n%s", vtt)
	}
	entries, _ := os.ReadDir(out)
	if len(entries) != 3 {
		t.Fatalf("output dir must hold only the sheets and the VTT, got %d entries", len(entries))
	}
	mu.Lock()
	last := progress[len(progress)-1]
	mu.Unlock()
	if last != 100 {
		t.Fatalf("last progress = %d, want 100", last)
	}
}

func TestSprites_ShortSceneGetsDenserFrames(t *testing.T) {
	_, silent := sharedClips(t)
	res, err := Sprites(context.Background(), Env{}, Request{
		Input:  silent,
		Video:  workerproto.VideoInfo{DurationSec: 30},
		OutDir: t.TempDir(),
	}, workerproto.SpritesParams{
		SceneID: 1, TileWidth: 64, TileHeight: 48, FrameInterval: 10, FrameQuality: 50,
		GridCols: 12, GridRows: 2,
	})
	if err != nil {
		t.Fatalf("Sprites failed: %v", err)
	}
	if res.FrameInterval != 2 { // 30 s / 12 cols
		t.Fatalf("effective interval = %d, want 2", res.FrameInterval)
	}
}

func TestSprites_Cancelled(t *testing.T) {
	_, silent := sharedClips(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Sprites(ctx, Env{}, Request{Input: silent, Video: workerproto.VideoInfo{DurationSec: 30}, OutDir: t.TempDir()},
		workerproto.SpritesParams{SceneID: 1, TileWidth: 64, TileHeight: 48, FrameInterval: 2, FrameQuality: 50, GridCols: 4, GridRows: 2})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestAnimatedThumbnails_Integration(t *testing.T) {
	withAudio, silent := sharedClips(t)

	tests := []struct {
		name          string
		input         string
		audioCodec    string
		segments      int
		wantPreviewAu int
	}{
		{"normal preview with audio", withAudio, "aac", 6, 1},
		{"short preview with audio", withAudio, "aac", 60, 1},
		{"silent source", silent, "", 6, 0},
		// The scene claims audio but has none: the preview with audio fails and is retried silent
		{"audio retry silent", silent, "aac", 6, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := t.TempDir()
			params := workerproto.AnimatedThumbnailsParams{
				SceneID:     9,
				MarkerWidth: 160,
				MarkerCRF:   32,
				Markers: []workerproto.MarkerClipSpec{
					{MarkerID: 3, TimestampSec: 2, DurationSec: 2, Output: workerproto.MarkerClipName(3)},
					{MarkerID: 4, TimestampSec: 20, DurationSec: 2, Output: workerproto.MarkerClipName(4)},
				},
				Preview: &workerproto.ScenePreviewSpec{
					Output: workerproto.ScenePreviewName(9), Segments: tt.segments, SegmentDuration: 1.0,
					Width: 160, CRF: 30, WithAudio: tt.audioCodec != "",
				},
			}
			res, err := AnimatedThumbnails(context.Background(), Env{}, Request{
				Input: tt.input, Video: workerproto.VideoInfo{DurationSec: 30, AudioCodec: tt.audioCodec}, OutDir: out,
			}, params)
			if err != nil {
				t.Fatalf("AnimatedThumbnails failed: %v", err)
			}
			if err := res.Validate(&params); err != nil {
				t.Fatalf("result does not validate: %v", err)
			}
			if len(res.Markers) != 2 || len(res.MarkerFailures) != 0 {
				t.Fatalf("markers = %+v failures = %+v", res.Markers, res.MarkerFailures)
			}
			if res.Preview == nil || res.PreviewError != "" {
				t.Fatalf("preview = %+v, error = %q", res.Preview, res.PreviewError)
			}
			preview := filepath.Join(out, "9_preview.mp4")
			if got := countStreams(t, preview, "v"); got != 1 {
				t.Fatalf("preview video streams = %d, want 1", got)
			}
			if got := countStreams(t, preview, "a"); got != tt.wantPreviewAu {
				t.Fatalf("preview audio streams = %d, want %d", got, tt.wantPreviewAu)
			}
			if res.Preview.WithAudio != (tt.wantPreviewAu == 1) {
				t.Fatalf("preview with_audio = %v", res.Preview.WithAudio)
			}
			for _, m := range res.Markers {
				clip := filepath.Join(out, m.File)
				if countStreams(t, clip, "v") != 1 || countStreams(t, clip, "a") != 0 {
					t.Fatalf("marker clip %s must be video only", m.File)
				}
			}
		})
	}
}

func TestAnimatedThumbnails_MarkerFailureIsBestEffort(t *testing.T) {
	res, err := AnimatedThumbnails(context.Background(), Env{}, Request{
		Input: filepath.Join(t.TempDir(), "missing.mp4"), Video: workerproto.VideoInfo{DurationSec: 30}, OutDir: t.TempDir(),
	}, workerproto.AnimatedThumbnailsParams{
		SceneID: 1, MarkerWidth: 160, MarkerCRF: 32,
		Markers: []workerproto.MarkerClipSpec{{MarkerID: 5, TimestampSec: 1, DurationSec: 1, Output: "marker_5.mp4"}},
		Preview: &workerproto.ScenePreviewSpec{Output: "1_preview.mp4", Segments: 3, SegmentDuration: 1, Width: 160, CRF: 30},
	})
	if err != nil {
		t.Fatalf("a missing source is a per-item failure, got %v", err)
	}
	if len(res.Markers) != 0 || len(res.MarkerFailures) != 1 || !strings.Contains(res.MarkerFailures[0].Error, "scene file not found") {
		t.Fatalf("markers = %+v failures = %+v", res.Markers, res.MarkerFailures)
	}
	if res.Preview != nil || !strings.Contains(res.PreviewError, "scene file not found") {
		t.Fatalf("preview = %+v error = %q", res.Preview, res.PreviewError)
	}
}

func TestAnimatedThumbnails_CancelledKeepsFinishedClips(t *testing.T) {
	_, silent := sharedClips(t)
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	res, err := AnimatedThumbnails(ctx, Env{}, Request{
		Input: silent, Video: workerproto.VideoInfo{DurationSec: 30}, OutDir: t.TempDir(),
		Progress: func(int) {
			calls++
			if calls == 1 {
				cancel() // after the first clip
			}
		},
	}, workerproto.AnimatedThumbnailsParams{
		SceneID: 1, MarkerWidth: 160, MarkerCRF: 32,
		Markers: []workerproto.MarkerClipSpec{
			{MarkerID: 1, TimestampSec: 1, DurationSec: 1, Output: "marker_1.mp4"},
			{MarkerID: 2, TimestampSec: 5, DurationSec: 1, Output: "marker_2.mp4"},
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if res == nil || len(res.Markers) != 1 || res.Markers[0].MarkerID != 1 {
		t.Fatalf("partial result = %+v, want marker 1 only", res)
	}
}

func TestMarkerClip_HardwareFailureFallsBackToSoftware(t *testing.T) {
	_, silent := sharedClips(t)
	env := Env{Accel: Accel{Name: "bogus", Steps: map[Step]StepAccel{
		StepMarkerClip: {Decode: DecodeAccel{HWAccel: "goonhub_no_such_hwaccel"}},
	}}}
	out := filepath.Join(t.TempDir(), "marker_1.mp4")
	if err := MarkerClip(context.Background(), env, silent, out, 1, 1, 160, 32, 0); err != nil {
		t.Fatalf("software fallback failed: %v", err)
	}
	if countStreams(t, out, "v") != 1 {
		t.Fatal("fallback clip missing")
	}

	env.Accel.NoSoftwareFallback = true
	if err := MarkerClip(context.Background(), env, silent, out, 1, 1, 160, 32, 0); err == nil {
		t.Fatal("without fallback the bogus hwaccel must fail")
	}
}

func TestMarkerClip_Timeout(t *testing.T) {
	_, silent := sharedClips(t)
	err := MarkerClip(context.Background(), Env{}, silent, filepath.Join(t.TempDir(), "m.mp4"), 0, 25, 320, 18, time.Millisecond)
	if err == nil {
		t.Fatal("a 1ms clip timeout must fail")
	}
}

func TestFingerprint_VisualIntegration(t *testing.T) {
	_, silent := sharedClips(t)
	res, err := Fingerprint(context.Background(), Env{}, Request{
		Input: silent, Video: workerproto.VideoInfo{DurationSec: 30},
	}, workerproto.FingerprintParams{Mode: workerproto.FingerprintModeAudioOnly})
	if err != nil {
		t.Fatalf("Fingerprint failed: %v", err)
	}
	// no audio track: visual only, one hash every 2 s
	if len(res.AudioFingerprint) != 0 || len(res.VisualFingerprint) < 14 || len(res.VisualFingerprint) > 16 {
		t.Fatalf("audio = %d hashes, visual = %d hashes", len(res.AudioFingerprint), len(res.VisualFingerprint))
	}
	if res.FingerprintTypeLabel() != "visual" {
		t.Fatalf("label = %q", res.FingerprintTypeLabel())
	}
}

func TestFingerprint_AudioIntegration(t *testing.T) {
	withAudio, _ := sharedClips(t)
	if err := chromaprint.CheckInstallation(); err != nil {
		t.Skip("fpcalc not on PATH: install libchromaprint-tools to run the audio fingerprint test")
	}
	res, err := Fingerprint(context.Background(), Env{}, Request{
		Input: withAudio, Video: workerproto.VideoInfo{DurationSec: 30, AudioCodec: "aac"},
	}, workerproto.FingerprintParams{Mode: workerproto.FingerprintModeDual})
	if err != nil {
		t.Fatalf("Fingerprint failed: %v", err)
	}
	if len(res.AudioFingerprint) == 0 || len(res.VisualFingerprint) == 0 || res.FingerprintTypeLabel() != "dual" {
		t.Fatalf("audio = %d, visual = %d, label %q", len(res.AudioFingerprint), len(res.VisualFingerprint), res.FingerprintTypeLabel())
	}
}

func TestFingerprint_DualKeepsVisualWhenAudioFails(t *testing.T) {
	_, silent := sharedClips(t)
	res, err := Fingerprint(context.Background(), Env{FpcalcPath: filepath.Join(t.TempDir(), "no-fpcalc")}, Request{
		Input: silent, Video: workerproto.VideoInfo{DurationSec: 30, AudioCodec: "aac"},
	}, workerproto.FingerprintParams{Mode: workerproto.FingerprintModeDual})
	if err != nil {
		t.Fatalf("dual mode must keep the visual fingerprint: %v", err)
	}
	if res.AudioError == "" || len(res.VisualFingerprint) == 0 || res.FingerprintTypeLabel() != "visual" {
		t.Fatalf("result = audio error %q, %d visual hashes", res.AudioError, len(res.VisualFingerprint))
	}

	// audio_only mode with audio: the audio failure fails the job
	_, err = Fingerprint(context.Background(), Env{FpcalcPath: filepath.Join(t.TempDir(), "no-fpcalc")}, Request{
		Input: silent, Video: workerproto.VideoInfo{DurationSec: 30, AudioCodec: "aac"},
	}, workerproto.FingerprintParams{Mode: workerproto.FingerprintModeAudioOnly})
	if err == nil || FailedStep(err) != StepAudioFingerprint {
		t.Fatalf("err = %v (step %q), want an audio step failure", err, FailedStep(err))
	}
}

func TestRun_Dispatch(t *testing.T) {
	if _, err := Run(context.Background(), Env{}, Request{}, workerproto.PhaseSprites, workerproto.PhaseParams{
		Fingerprint: &workerproto.FingerprintParams{Mode: "dual"},
	}); err == nil {
		t.Fatal("params of another phase must be refused")
	}
}
