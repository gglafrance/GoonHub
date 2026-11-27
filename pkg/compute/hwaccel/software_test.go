package hwaccel

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"goonhub/pkg/compute"
)

// These tests need ffmpeg/ffprobe on PATH (skipped otherwise) but no GPU.

func requireFFmpegOnPath(t *testing.T) {
	t.Helper()
	for _, b := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(b); err != nil {
			t.Skipf("%s not on PATH", b)
		}
	}
}

func TestGenerateSample(t *testing.T) {
	requireFFmpegOnPath(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "s.mp4")
	if err := GenerateSample(ctx, "", path, SampleSpec{Width: 320, Height: 240, Seconds: 3, Audio: true}); err != nil {
		t.Fatal(err)
	}
	info, err := ProbeVideo(ctx, "ffprobe", path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Codec != "h264" || info.Width != 320 || info.Height != 240 || info.Duration < 2.9 || info.Duration > 3.2 || info.PixFmt != "yuv420p" {
		t.Fatalf("sample %+v", info)
	}
	if err := GenerateSample(ctx, "", path, SampleSpec{Codec: "av1"}); err == nil {
		t.Fatal("unsupported codec must fail")
	}
	if got := (SampleSpec{Codec: "hevc", TenBit: true, Width: 3840, Height: 2160}).Name(); got != "hevc10_3840x2160_20s_testsrc2.mp4" {
		t.Fatalf("name %q", got)
	}
}

func TestDetect_OtherOSMethodsAreSkipped(t *testing.T) {
	requireFFmpegOnPath(t)
	other := MethodVideoToolbox
	if runtime.GOOS == "darwin" {
		other = MethodD3D11VA
	}
	r, err := DetectWith(context.Background(), "", DetectOptions{Methods: []Method{other}})
	if err != nil {
		t.Fatal(err)
	}
	if r.FFmpegVersion == "" || len(r.Encoders) == 0 || len(r.Methods) != 1 {
		t.Fatalf("report %+v", r)
	}
	if !strings.Contains(r.Methods[0].Skipped, "not available on") {
		t.Fatalf("skipped = %q", r.Methods[0].Skipped)
	}
	if acc := Choose(r, Prefs{}); acc.Label() != "none" {
		t.Fatalf("nothing proven, got %s", acc.Label())
	}
}

func TestDetect_BadFFmpeg(t *testing.T) {
	if _, err := Detect(context.Background(), filepath.Join(t.TempDir(), "no-ffmpeg")); err == nil {
		t.Fatal("a missing ffmpeg must fail")
	}
}

func TestBench_SoftwareAndBrokenAccel(t *testing.T) {
	requireFFmpegOnPath(t)
	ctx := context.Background()
	broken := compute.Accel{Name: "broken", Steps: map[compute.Step]compute.StepAccel{
		compute.StepMarkerClip: {Decode: compute.DecodeAccel{HWAccel: "goonhub_no_such_hwaccel"}},
	}}
	b, err := Bench(ctx, "", broken, Sample{
		Spec: SampleSpec{Width: 320, Height: 240, Seconds: 6}, SpriteFrames: 3, Runs: 1, MarkerWidth: 160, PreviewWidth: 160, PreviewSegments: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Steps) != 4 || b.Accel != "broken" || b.Video.Width != 320 {
		t.Fatalf("report %+v", b)
	}
	for _, s := range b.Steps {
		if s.Software.WallMS <= 0 {
			t.Fatalf("%s: no software time", s.Step)
		}
		if s.Step == compute.StepMarkerClip {
			if s.Accelerated == nil || s.Error == "" || s.Worth() {
				t.Fatalf("broken marker clip must be reported failed: %+v", s)
			}
			if s.Software.SSIM < 0.5 || s.Software.OutputBytes == 0 {
				t.Fatalf("software marker measure %+v", s.Software)
			}
			continue
		}
		if s.Accelerated != nil {
			t.Fatalf("%s is software in the accel, got %+v", s.Step, s)
		}
		if s.Step != compute.StepVisualHash && s.Software.OutputBytes == 0 {
			t.Fatalf("%s: no output", s.Step)
		}
	}
	if b.Step(compute.StepVisualHash) == nil || (*BenchReport)(nil).Step(compute.StepVisualHash) != nil {
		t.Fatal("Step lookup")
	}
}
