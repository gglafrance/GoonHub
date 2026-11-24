package hwaccel

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"goonhub/pkg/compute"
)

// GPU tests run real hardware pipelines. They are off unless GOONHUB_HWACCEL_TESTS=1.
//
//	GOONHUB_HWACCEL_TESTS=1        enable them
//	GOONHUB_HWACCEL_FFMPEG=<path>  ffmpeg to test ("" = ffmpeg from PATH)
//	GOONHUB_HWACCEL_SAMPLES=<dir>  use the *.mp4 of dir as samples instead of generated ones
//	GOONHUB_HWACCEL_EXPECT=a,b     methods that must be proven (e.g. "nvidia,d3d11va")
//	GOONHUB_HWACCEL_SAMPLE_CACHE=<dir> where generated samples are kept between runs
//	                               (default: <os temp>/goonhub-hwaccel-samples)

func requireGPUTests(t *testing.T) {
	t.Helper()
	if os.Getenv("GOONHUB_HWACCEL_TESTS") != "1" {
		t.Skip("hardware tests disabled: set GOONHUB_HWACCEL_TESTS=1")
	}
}

func testFFmpeg() string { return os.Getenv("GOONHUB_HWACCEL_FFMPEG") }

var (
	reportOnce sync.Once
	report     *Report
	reportErr  error
)

// detected runs Detect once per test binary.
func detected(t *testing.T) *Report {
	t.Helper()
	requireGPUTests(t)
	reportOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		report, reportErr = DetectWith(ctx, testFFmpeg(), DetectOptions{Sequential: true})
	})
	if reportErr != nil {
		t.Fatalf("Detect failed: %v", reportErr)
	}
	return report
}

// testSamples are the samples of the GPU tests: H.264 and HEVC, 1080p and 4K, ~20 s.
var defaultSampleSpecs = []SampleSpec{
	{Codec: "h264", Width: 1920, Height: 1080, Seconds: 20, Source: "mandelbrot"},
	{Codec: "hevc", Width: 1920, Height: 1080, Seconds: 20, Source: "mandelbrot"},
	{Codec: "h264", Width: 3840, Height: 2160, Seconds: 20, Source: "mandelbrot"},
	{Codec: "hevc", Width: 3840, Height: 2160, Seconds: 20, Source: "mandelbrot", TenBit: true},
	{Codec: "h264", Width: 1920, Height: 1080, Seconds: 20, Source: "testsrc2", Audio: true},
}

var (
	samplesOnce sync.Once
	samples     []string
	samplesErr  error
)

func testSamples(t *testing.T) []string {
	t.Helper()
	requireGPUTests(t)
	samplesOnce.Do(func() {
		if dir := os.Getenv("GOONHUB_HWACCEL_SAMPLES"); dir != "" {
			samples, samplesErr = filepath.Glob(filepath.Join(dir, "*.mp4"))
			sort.Strings(samples)
			return
		}
		cache := os.Getenv("GOONHUB_HWACCEL_SAMPLE_CACHE")
		if cache == "" {
			cache = filepath.Join(os.TempDir(), "goonhub-hwaccel-samples")
		}
		if samplesErr = os.MkdirAll(cache, 0o755); samplesErr != nil {
			return
		}
		for _, spec := range defaultSampleSpecs {
			path := filepath.Join(cache, spec.Name())
			if st, err := os.Stat(path); err != nil || st.Size() == 0 {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
				err := GenerateSample(ctx, testFFmpeg(), path+".tmp.mp4", spec)
				cancel()
				if err != nil {
					samplesErr = err
					return
				}
				if samplesErr = os.Rename(path+".tmp.mp4", path); samplesErr != nil {
					return
				}
			}
			samples = append(samples, path)
		}
	})
	if samplesErr != nil {
		t.Fatalf("samples: %v", samplesErr)
	}
	if len(samples) == 0 {
		t.Fatal("no samples")
	}
	return samples
}

// provenPipelines lists the proven (method, variant) pairs of step.
func provenPipelines(r *Report, step compute.Step) []struct {
	Method  Method
	Variant Variant
	Accel   compute.StepAccel
} {
	var out []struct {
		Method  Method
		Variant Variant
		Accel   compute.StepAccel
	}
	for _, mr := range r.Methods {
		for _, c := range mr.Candidates {
			if c.Step == step && c.OK {
				acc, _ := Pipeline(mr.Method, step, c.Variant, r.pipelineOptions())
				out = append(out, struct {
					Method  Method
					Variant Variant
					Accel   compute.StepAccel
				}{mr.Method, c.Variant, acc})
			}
		}
	}
	return out
}

func logJSON(t *testing.T, label string, v any) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", label, err)
	}
	t.Logf("%s:\n%s", label, b)
}

func expectedMethods() []Method {
	var out []Method
	for _, s := range strings.Split(os.Getenv("GOONHUB_HWACCEL_EXPECT"), ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, Method(s))
		}
	}
	return out
}

func singleStep(name Method, step compute.Step, acc compute.StepAccel) compute.Env {
	return compute.Env{
		FFmpegPath:  testFFmpeg(),
		FFprobePath: ffprobePath(ffmpegBin(testFFmpeg())),
		Accel:       compute.Accel{Name: string(name), Steps: map[compute.Step]compute.StepAccel{step: acc}, NoSoftwareFallback: true},
	}
}
