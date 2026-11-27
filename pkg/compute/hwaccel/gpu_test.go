package hwaccel

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"goonhub/pkg/compute"
	"goonhub/pkg/dhash"
	"goonhub/pkg/workerproto"
)

func TestGPU_Detect(t *testing.T) {
	r := detected(t)
	logJSON(t, "report", r)
	t.Logf("summary: %s", r.Summary())
	t.Logf("choice (balanced): %s", describe(Choose(r, Prefs{})))
	t.Logf("choice (speed): %s", describe(Choose(r, Prefs{Prefer: PreferSpeed})))
	t.Logf("capabilities: hwaccels=%v encoders=%v", r.ProvenHWAccels(), r.ProvenEncoders())

	for _, m := range expectedMethods() {
		if mr := r.Method(m); mr == nil || mr.Skipped != "" {
			t.Fatalf("expected method %s was not probed: %+v", m, mr)
		}
		if !contains(methodsOf(r.Working()), string(m)) {
			t.Fatalf("expected method %s has no proven pipeline", m)
		}
	}
	// what the brief requires of an NVIDIA machine
	if contains(methodsOf(expectedMethods()), string(MethodNVIDIA)) {
		for _, c := range []struct {
			step compute.Step
			v    Variant
		}{
			{compute.StepVisualHash, VariantGPUFrames}, {compute.StepVisualHash, VariantDecode}, {compute.StepSpriteFrame, VariantDecode},
			{compute.StepMarkerClip, VariantQuality}, {compute.StepMarkerClip, VariantSpeed},
			{compute.StepScenePreview, VariantQuality},
		} {
			if !r.Proven(MethodNVIDIA, c.step, c.v) {
				t.Fatalf("nvidia %s/%s not proven", c.step, c.v)
			}
		}
		acc := Choose(r, Prefs{})
		if acc.Name != string(MethodNVIDIA) || acc.For(compute.StepMarkerClip).Encoder.H264 != "h264_nvenc" {
			t.Fatalf("Choose = %s", describe(acc))
		}
	}
}

func methodsOf(ms []Method) []string {
	var out []string
	for _, m := range ms {
		out = append(out, string(m))
	}
	return out
}

func describe(a compute.Accel) string {
	var parts []string
	for _, s := range Steps {
		parts = append(parts, fmt.Sprintf("%s[%s]", s, a.For(s)))
	}
	return a.Label() + ": " + strings.Join(parts, " ")
}

// softwareFullDecodeHashes are the hashes of a software decode without the skip options:
// what the server indexed before dhash skipped the loop filter and the non-reference frames.
func softwareFullDecodeHashes(ctx context.Context, input string) ([]uint64, error) {
	return dhash.RunFFmpeg(ctx, ffmpegBin(testFFmpeg()), []string{"-nostdin", "-i", input, "-an", "-sn", "-dn", "-vf", dhash.Filter, "-f", "rawvideo", "pipe:1"})
}

// TestGPU_VisualHashCompatibility: the accelerated hashes must stay usable against the
// indexed software ones. Reference: the drift that already exists between the indexed
// software hashes (skip_loop_filter + skip_frame noref) and a full software decode.
func TestGPU_VisualHashCompatibility(t *testing.T) {
	r := detected(t)
	pipes := provenPipelines(r, compute.StepVisualHash)
	if len(pipes) == 0 {
		t.Skip("no proven visual hash pipeline on this machine")
	}
	ctx := context.Background()
	var total = map[string]*HashDrift{}
	for _, sample := range testSamples(t) {
		sw, err := compute.Fingerprint(ctx, compute.Env{FFmpegPath: testFFmpeg()}, compute.Request{Input: sample}, workerproto.FingerprintParams{Mode: workerproto.FingerprintModeAudioOnly})
		if err != nil {
			t.Fatalf("software hashes of %s: %v", sample, err)
		}
		full, err := softwareFullDecodeHashes(ctx, sample)
		if err != nil {
			t.Fatalf("full decode hashes: %v", err)
		}
		base := CompareHashes(sw.VisualFingerprint, full)
		t.Logf("%-40s software full decode vs indexed software: %s", filepath.Base(sample), driftString(base))
		accumulate(total, "software_full_decode", base)

		for _, p := range pipes {
			start := time.Now()
			res, err := compute.Fingerprint(ctx, singleStep(p.Method, compute.StepVisualHash, p.Accel), compute.Request{Input: sample},
				workerproto.FingerprintParams{Mode: workerproto.FingerprintModeAudioOnly})
			if err != nil {
				t.Fatalf("%s/%s on %s: %v", p.Method, p.Variant, sample, err)
			}
			d := CompareHashes(sw.VisualFingerprint, res.VisualFingerprint)
			dFull := CompareHashes(full, res.VisualFingerprint)
			t.Logf("%-40s %s/%s (%s): vs software %s | vs full decode %s", filepath.Base(sample), p.Method, p.Variant,
				time.Since(start).Round(time.Millisecond), driftString(d), driftString(dFull))
			accumulate(total, string(p.Method)+"/"+string(p.Variant), d)

			if d.Frames != d.ReferenceFrames {
				t.Fatalf("%s/%s: %d hashes, software %d", p.Method, p.Variant, d.Frames, d.ReferenceFrames)
			}
			// per sample: no worse than the existing full-decode drift + 1 bit on average,
			// and never more than 3 bits on average
			if d.Mean > base.Mean+1.0 || d.Mean > 3 {
				t.Fatalf("%s/%s on %s drifts too much: %s (full decode baseline %s)", p.Method, p.Variant, sample, driftString(d), driftString(base))
			}
		}
	}
	for k, d := range total {
		t.Logf("TOTAL %-24s %s within5=%.1f%%", k, driftString(*d), 100*d.Within5Ratio())
	}
	for k, d := range total {
		if k == "software_full_decode" {
			continue
		}
		base := total["software_full_decode"]
		if d.Within5Ratio() < base.Within5Ratio()-0.05 || d.Within5Ratio() < 0.85 {
			t.Fatalf("%s: only %.1f%% of frames within %d bits (baseline %.1f%%)", k, 100*d.Within5Ratio(), MatchHammingMax, 100*base.Within5Ratio())
		}
	}
}

func driftString(d HashDrift) string {
	return fmt.Sprintf("frames %d/%d mean %.2f max %d exact %d <=3 %d <=5 %d", d.Frames, d.ReferenceFrames, d.Mean, d.Max, d.Exact, d.Within3, d.Within5)
}

func accumulate(total map[string]*HashDrift, key string, d HashDrift) {
	t := total[key]
	if t == nil {
		t = &HashDrift{}
		total[key] = t
	}
	sum := t.Mean*float64(t.Compared) + d.Mean*float64(d.Compared)
	t.Frames += d.Frames
	t.ReferenceFrames += d.ReferenceFrames
	t.Compared += d.Compared
	t.Exact += d.Exact
	t.Within3 += d.Within3
	t.Within5 += d.Within5
	if d.Max > t.Max {
		t.Max = d.Max
	}
	if t.Compared > 0 {
		t.Mean = sum / float64(t.Compared)
	}
}

// TestGPU_EncodeOutputs runs the marker clips and the scene preview of every proven pipeline
// on every sample and compares them with the software files.
func TestGPU_EncodeOutputs(t *testing.T) {
	r := detected(t)
	ctx := context.Background()
	probe := ffprobePath(ffmpegBin(testFFmpeg()))
	for _, sample := range testSamples(t) {
		info, err := ProbeVideo(ctx, probe, sample)
		if err != nil {
			t.Fatal(err)
		}
		dur := int(info.Duration)
		hasAudio := hasAudioStream(ctx, probe, sample)
		params := func(crfMarker, crfPreview int) workerproto.AnimatedThumbnailsParams {
			return workerproto.AnimatedThumbnailsParams{
				SceneID: 1, MarkerWidth: 480, MarkerCRF: crfMarker,
				Markers: []workerproto.MarkerClipSpec{{MarkerID: 1, TimestampSec: dur / 4, DurationSec: 10, Output: workerproto.MarkerClipName(1)}},
				Preview: &workerproto.ScenePreviewSpec{Output: workerproto.ScenePreviewName(1), Segments: 12, SegmentDuration: 1, Width: 480, CRF: crfPreview, WithAudio: hasAudio},
			}
		}
		req := func(out string) compute.Request {
			ac := ""
			if hasAudio {
				ac = "aac"
			}
			return compute.Request{Input: sample, Video: workerproto.VideoInfo{DurationSec: dur, AudioCodec: ac}, OutDir: out}
		}

		swDir := t.TempDir()
		swRes, err := compute.AnimatedThumbnails(ctx, compute.Env{FFmpegPath: testFFmpeg()}, req(swDir), params(32, 27))
		if err != nil || len(swRes.Markers) != 1 || swRes.Preview == nil {
			t.Fatalf("software animated thumbnails: %v %+v", err, swRes)
		}
		swClip, swPrev := filepath.Join(swDir, "marker_1.mp4"), filepath.Join(swDir, "1_preview.mp4")
		swClipInfo, _ := ProbeVideo(ctx, probe, swClip)
		swPrevInfo, _ := ProbeVideo(ctx, probe, swPrev)
		swSSIM, err := SSIMAgainstSource(ctx, testFFmpeg(), sample, swClip, dur/4, 10, 480)
		if err != nil {
			t.Fatal(err)
		}

		// one entry per (method, variant), with the pipeline of each step (a step the
		// variant does not have stays in software)
		type combo struct {
			Method       Method
			Variant      Variant
			Marker, Prev compute.StepAccel
		}
		index := map[string]int{}
		var pipes []combo
		for _, step := range []compute.Step{compute.StepMarkerClip, compute.StepScenePreview} {
			for _, p := range provenPipelines(r, step) {
				key := string(p.Method) + "/" + string(p.Variant)
				i, ok := index[key]
				if !ok {
					i = len(pipes)
					index[key] = i
					pipes = append(pipes, combo{Method: p.Method, Variant: p.Variant})
				}
				if step == compute.StepMarkerClip {
					pipes[i].Marker = p.Accel
				} else {
					pipes[i].Prev = p.Accel
				}
			}
		}
		for _, p := range pipes {
			out := t.TempDir()
			env := compute.Env{FFmpegPath: testFFmpeg(), Accel: compute.Accel{Name: string(p.Method), NoSoftwareFallback: true,
				Steps: map[compute.Step]compute.StepAccel{compute.StepMarkerClip: p.Marker, compute.StepScenePreview: p.Prev}}}
			res, err := compute.AnimatedThumbnails(ctx, env, req(out), params(32, 27))
			if err != nil || len(res.Markers) != 1 || res.Preview == nil {
				t.Fatalf("%s/%s on %s: %v %+v", p.Method, p.Variant, sample, err, res)
			}
			if res.Preview.WithAudio != swRes.Preview.WithAudio {
				t.Fatalf("%s/%s: preview audio %v, software %v", p.Method, p.Variant, res.Preview.WithAudio, swRes.Preview.WithAudio)
			}
			clip, prev := filepath.Join(out, "marker_1.mp4"), filepath.Join(out, "1_preview.mp4")
			ci, err := ProbeVideo(ctx, probe, clip)
			if err != nil {
				t.Fatal(err)
			}
			pi, err := ProbeVideo(ctx, probe, prev)
			if err != nil {
				t.Fatal(err)
			}
			if ci.Width != swClipInfo.Width || ci.Height != swClipInfo.Height || pi.Width != swPrevInfo.Width || pi.Height != swPrevInfo.Height {
				t.Fatalf("%s/%s dims clip %dx%d preview %dx%d, software %dx%d / %dx%d", p.Method, p.Variant, ci.Width, ci.Height, pi.Width, pi.Height,
					swClipInfo.Width, swClipInfo.Height, swPrevInfo.Width, swPrevInfo.Height)
			}
			if math.Abs(ci.Duration-swClipInfo.Duration) > 0.2 || math.Abs(pi.Duration-swPrevInfo.Duration) > 0.2 {
				t.Fatalf("%s/%s durations clip %.2f preview %.2f, software %.2f / %.2f", p.Method, p.Variant, ci.Duration, pi.Duration, swClipInfo.Duration, swPrevInfo.Duration)
			}
			if ci.PixFmt != "yuv420p" || pi.PixFmt != "yuv420p" {
				t.Fatalf("%s/%s pixel formats %s / %s, want yuv420p (browsers)", p.Method, p.Variant, ci.PixFmt, pi.PixFmt)
			}
			for _, f := range []string{clip, prev} {
				if _, err := run(ctx, ffmpegBin(testFFmpeg()), "-hide_banner", "-v", "error", "-nostdin", "-i", f, "-f", "null", "-"); err != nil {
					t.Fatalf("%s does not decode: %v", f, err)
				}
			}
			ssim, err := SSIMAgainstSource(ctx, testFFmpeg(), sample, clip, dur/4, 10, 480)
			if err != nil {
				t.Fatal(err)
			}
			clipRatio := float64(fileSize(clip)) / float64(fileSize(swClip))
			prevRatio := float64(fileSize(prev)) / float64(fileSize(swPrev))
			t.Logf("%-40s %-18s clip %7d B (%.2fx sw) ssim %.4f (%+.4f) | preview %7d B (%.2fx sw) [clip %s | preview %s]", filepath.Base(sample), p.Method+"/"+Method(p.Variant),
				fileSize(clip), clipRatio, ssim, ssim-swSSIM, fileSize(prev), prevRatio, p.Marker, p.Prev)
			if clipRatio < 0.25 || clipRatio > 4 || prevRatio < 0.25 || prevRatio > 4 {
				t.Fatalf("%s/%s sizes far from software: clip %.2fx preview %.2fx", p.Method, p.Variant, clipRatio, prevRatio)
			}
			// hardware encoders lose some quality on sharp synthetic content (testsrc2 text):
			// AMF on the Radeon iGPU -0.042 there, within +-0.01 on filmed content
			if ssim < swSSIM-0.05 {
				t.Fatalf("%s/%s marker clip SSIM %.4f, software %.4f", p.Method, p.Variant, ssim, swSSIM)
			}
		}
	}
}

// TestGPU_Sprites: accelerated sprite sheets have the software's layout.
func TestGPU_Sprites(t *testing.T) {
	r := detected(t)
	pipes := provenPipelines(r, compute.StepSpriteFrame)
	if len(pipes) == 0 {
		t.Skip("no proven sprite pipeline")
	}
	ctx := context.Background()
	sample := testSamples(t)[0]
	params := workerproto.SpritesParams{SceneID: 3, TileWidth: 320, TileHeight: 180, FrameInterval: 2, FrameQuality: 75, GridCols: 4, GridRows: 2, Concurrency: 4}
	sw, err := compute.Sprites(ctx, compute.Env{FFmpegPath: testFFmpeg()}, compute.Request{Input: sample, Video: workerproto.VideoInfo{DurationSec: 20}, OutDir: t.TempDir()}, params)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pipes {
		out := t.TempDir()
		res, err := compute.Sprites(ctx, singleStep(p.Method, compute.StepSpriteFrame, p.Accel), compute.Request{Input: sample, Video: workerproto.VideoInfo{DurationSec: 20}, OutDir: out}, params)
		if err != nil {
			t.Fatalf("%s: %v", p.Method, err)
		}
		if fmt.Sprint(res.SpriteSheets) != fmt.Sprint(sw.SpriteSheets) {
			t.Fatalf("%s sheets %v, software %v", p.Method, res.SpriteSheets, sw.SpriteSheets)
		}
		info, err := ProbeVideo(ctx, ffprobePath(ffmpegBin(testFFmpeg())), filepath.Join(out, res.SpriteSheets[0]))
		if err != nil || info.Width != 1280 || info.Height != 360 {
			t.Fatalf("%s sheet %+v %v", p.Method, info, err)
		}
	}
}

// TestGPU_Bench measures every step, software vs the balanced and speed choices.
func TestGPU_Bench(t *testing.T) {
	r := detected(t)
	ctx := context.Background()
	for _, pref := range []Preference{PreferBalanced, PreferSpeed} {
		acc := Choose(r, Prefs{Prefer: pref, SpriteFrames: true})
		if acc.Label() == "none" {
			t.Skip("nothing accelerated on this machine")
		}
		for _, sample := range testSamples(t) {
			b, err := Bench(ctx, testFFmpeg(), acc, Sample{Path: sample})
			if err != nil {
				t.Fatalf("Bench %s: %v", sample, err)
			}
			for _, s := range b.Steps {
				line := fmt.Sprintf("[%s] %-40s %-14s sw %6d ms cpu %6d ms", prefName(pref), b.Sample, s.Step, s.Software.WallMS, s.Software.CPUMS)
				if s.Accelerated != nil {
					line += fmt.Sprintf(" | hw %6d ms cpu %6d ms | x%.2f cpu %.2f", s.Accelerated.WallMS, s.Accelerated.CPUMS, s.Speedup, s.CPURatio)
					if s.SizeRatio > 0 {
						line += fmt.Sprintf(" size %.2fx", s.SizeRatio)
					}
					if s.SSIMDelta != 0 {
						line += fmt.Sprintf(" ssim %+.4f", s.SSIMDelta)
					}
					if s.HashDrift != nil {
						line += " hash " + driftString(*s.HashDrift)
					}
					line += fmt.Sprintf(" worth=%v", s.Worth())
				}
				if s.Error != "" {
					line += " ERROR " + s.Error
				}
				t.Log(line)
				if s.Error != "" {
					t.Fatalf("accelerated %s failed: %s", s.Step, s.Error)
				}
			}
		}
	}
}

func prefName(p Preference) string {
	if p == "" {
		return "balanced"
	}
	return string(p)
}

// TestGPU_FallbackOnBrokenPipeline: a proven pipeline that fails on an input (here a forced
// bogus device) is retried in software through pkg/compute.
func TestGPU_FallbackOnBrokenPipeline(t *testing.T) {
	r := detected(t)
	pipes := provenPipelines(r, compute.StepMarkerClip)
	if len(pipes) == 0 {
		t.Skip("no proven marker pipeline")
	}
	acc := pipes[0].Accel
	acc.Encoder.ExtraArgs = append(append([]string{}, acc.Encoder.ExtraArgs...), "-goonhub_no_such_option", "1")
	env := compute.Env{FFmpegPath: testFFmpeg(), Accel: compute.Accel{Name: "broken", Steps: map[compute.Step]compute.StepAccel{compute.StepMarkerClip: acc}}}
	out := filepath.Join(t.TempDir(), "m.mp4")
	if err := compute.MarkerClip(context.Background(), env, testSamples(t)[0], out, 2, 2, 320, 30, 0); err != nil {
		t.Fatalf("software fallback failed: %v", err)
	}
	if st, err := os.Stat(out); err != nil || st.Size() == 0 {
		t.Fatal("no output after fallback")
	}
}

func hasAudioStream(ctx context.Context, probe, path string) bool {
	res, err := run(ctx, probe, "-v", "error", "-select_streams", "a", "-show_entries", "stream=codec_type", "-of", "csv=p=0", path)
	return err == nil && strings.TrimSpace(string(res.Stdout)) != ""
}
