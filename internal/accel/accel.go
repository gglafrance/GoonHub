// Package accel chooses the hardware acceleration the worker uses for its computations.
//
// HOOK: the detection and the profiles live in goonhub/pkg/compute/hwaccel (Detect,
// Choose, Bench), built in another worktree. Until it lands this package always returns
// software (compute.Accel{}), which produces exactly the server's ffmpeg commands. To wire
// it, only Select and Bench below change; the rest of the worker takes the compute.Accel
// they return.
package accel

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"goonhub/pkg/compute"
	"goonhub/pkg/workerproto"

	"goonhub-worker/internal/tools"
)

// Mode is the config value: "auto" or "none".
type Mode string

const (
	ModeAuto Mode = "auto"
	ModeNone Mode = "none"
)

// Selection is the chosen acceleration and how it was chosen (for logs and `detect`).
type Selection struct {
	Accel compute.Accel
	// Reason explains the choice in one line.
	Reason string
}

// Select returns the acceleration to use. Single wiring point for pkg/compute/hwaccel:
//
//	caps := hwaccel.Detect(ctx, t.FFmpeg)          // probe encoders/decoders that really work
//	acc  := hwaccel.Choose(caps, prefs)             // per-step profile (decode/scale/encode)
//	return Selection{Accel: acc, Reason: ...}
//
// pkg/compute retries any failed accelerated step in software, so a wrong choice costs
// speed, never a result.
func Select(ctx context.Context, t *tools.Tools, mode Mode) Selection {
	_ = ctx
	_ = t
	if mode == ModeNone {
		return Selection{Accel: compute.Accel{}, Reason: "accel: none in the config (software)"}
	}
	return Selection{Accel: compute.Accel{}, Reason: "hardware acceleration profiles are not built in yet (pkg/compute/hwaccel): software"}
}

// BenchResult is one measured step.
type BenchResult struct {
	Name     string
	Duration time.Duration
	Err      error
}

// Bench measures the computations on a sample clip with the given acceleration.
// HOOK: replace with hwaccel.Bench (software vs every detected profile) once available;
// until then it times the software path of each phase on the clip.
func Bench(ctx context.Context, t *tools.Tools, sel Selection, clip string, video workerproto.VideoInfo, outDir string) []BenchResult {
	env := compute.Env{FFmpegPath: t.FFmpeg, FFprobePath: t.FFprobe, FpcalcPath: t.Fpcalc, Accel: sel.Accel, TempDir: outDir}
	var res []BenchResult
	measure := func(name string, fn func() error) {
		start := time.Now()
		err := fn()
		res = append(res, BenchResult{Name: name, Duration: time.Since(start), Err: err})
	}
	measure("sprites (12 frames)", func() error {
		_, err := compute.Sprites(ctx, env, compute.Request{Input: clip, Video: video, OutDir: outDir},
			workerproto.SpritesParams{SceneID: 1, TileWidth: 160, TileHeight: 90, FrameInterval: 2,
				FrameQuality: 75, GridCols: 12, GridRows: 12})
		return err
	})
	measure("marker clip (5 s)", func() error {
		return compute.MarkerClip(ctx, env, clip, filepath.Join(outDir, "bench_marker.mp4"), 2, 5, 320, 28, time.Minute)
	})
	measure("visual fingerprint", func() error {
		v := video
		v.AudioCodec = ""
		_, err := compute.Fingerprint(ctx, env, compute.Request{Input: clip, Video: v},
			workerproto.FingerprintParams{Mode: workerproto.FingerprintModeAudioOnly})
		return err
	})
	if t.Fpcalc != "" {
		measure("audio fingerprint", func() error {
			v := video
			if v.AudioCodec == "" {
				return fmt.Errorf("sample clip has no audio")
			}
			_, err := compute.Fingerprint(ctx, env, compute.Request{Input: clip, Video: v},
				workerproto.FingerprintParams{Mode: workerproto.FingerprintModeAudioOnly})
			return err
		})
	}
	return res
}
