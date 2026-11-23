// Package accel chooses the hardware acceleration the worker uses for its computations.
//
// The detection and the profiles live in goonhub/pkg/compute/hwaccel: Detect proves every
// pipeline by running it on tiny clips, Choose picks per step the first proven method.
// pkg/compute retries any failed accelerated step in software, so a wrong choice costs
// speed, never a result.
package accel

import (
	"context"
	"fmt"
	"strings"
	"time"

	"goonhub/pkg/compute"
	"goonhub/pkg/compute/hwaccel"

	"goonhub-worker/internal/tools"
)

// Mode is the config value: "auto", "none" or a method name (nvidia, videotoolbox,
// intel, vaapi, d3d11va, amd) to force it.
type Mode string

const (
	ModeAuto Mode = "auto"
	ModeNone Mode = "none"
)

// ValidMode reports whether s is a valid accel config value.
func ValidMode(s string) bool {
	if s == string(ModeAuto) || s == string(ModeNone) {
		return true
	}
	return hwaccel.Method(s).Valid()
}

// Selection is the chosen acceleration and how it was chosen (for logs and `detect`).
type Selection struct {
	Accel compute.Accel
	// Reason explains the choice in one line.
	Reason string
	// Report is the detection (nil in mode none or when detection failed).
	Report *hwaccel.Report
}

// Options tune Select.
type Options struct {
	// VAAPIDevice is the DRM render node for VA-API ("" = /dev/dri/renderD128).
	VAAPIDevice string
	// TempDir holds the probe clips ("" = the OS temp dir).
	TempDir string
}

// Select detects the working pipelines and returns the acceleration to use. Any detection
// error gives software.
func Select(ctx context.Context, t *tools.Tools, mode Mode, o Options) Selection {
	if mode == ModeNone {
		return Selection{Accel: compute.Accel{}, Reason: "accel: none in the config (software)"}
	}
	dctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	opts := hwaccel.DetectOptions{VAAPIDevice: o.VAAPIDevice, TempDir: o.TempDir}
	prefs := hwaccel.Prefs{}
	if mode != ModeAuto {
		opts.Methods = []hwaccel.Method{hwaccel.Method(mode)}
		prefs.Force = hwaccel.Method(mode)
	}
	report, err := hwaccel.DetectWith(dctx, t.FFmpeg, opts)
	if err != nil {
		return Selection{Accel: compute.Accel{}, Reason: fmt.Sprintf("hardware detection failed (%v): software", err)}
	}
	acc := hwaccel.Choose(report, prefs)
	if len(acc.Steps) == 0 {
		return Selection{Accel: compute.Accel{}, Report: report,
			Reason: fmt.Sprintf("no proven hardware pipeline (%s): software", report.Summary())}
	}
	return Selection{Accel: acc, Report: report,
		Reason: fmt.Sprintf("detected in %d ms: %s", report.ElapsedMS, describe(acc))}
}

// describe lists the accelerated steps ("marker_clip=nvidia..., visual_hash=...").
func describe(acc compute.Accel) string {
	var parts []string
	for _, step := range hwaccel.Steps {
		if sa, ok := acc.Steps[step]; ok {
			parts = append(parts, string(step)+"="+sa.String())
		}
	}
	return strings.Join(parts, ", ")
}

// Bench times every step in software and with the selection on sample (an existing video
// when sample.Path is set, else a generated clip).
func Bench(ctx context.Context, t *tools.Tools, sel Selection, sample hwaccel.Sample) (*hwaccel.BenchReport, error) {
	acc := sel.Accel
	if len(acc.Steps) == 0 && sel.Report != nil {
		// measure what auto would pick, sprite frames included, to show the comparison
		acc = hwaccel.Choose(sel.Report, hwaccel.Prefs{SpriteFrames: true})
	}
	return hwaccel.Bench(ctx, t.FFmpeg, acc, sample)
}
