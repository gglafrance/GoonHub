package hwaccel

import "goonhub/pkg/compute"

// Preference orients the choice between pipelines.
type Preference string

const (
	// PreferBalanced (default): encode steps decode on the GPU, scale with the software
	// scaler, encode on the GPU; the visual hash keeps frames on the GPU until fps=1/2.
	PreferBalanced Preference = ""
	// PreferQuality: like balanced, but the visual hash downloads every decoded frame
	// (hashes bit-identical to a software full decode, at a higher bus/CPU cost).
	PreferQuality Preference = "quality"
	// PreferSpeed: encode steps also scale on the GPU (frames never leave the GPU).
	PreferSpeed Preference = "speed"
)

// Prefs are the worker's choices.
type Prefs struct {
	Prefer Preference
	// Disable lists methods never used.
	Disable []Method
	// Force uses only this method, even for pipelines Detect did not prove (a failing run
	// still falls back to software). MethodNone forces software everywhere.
	Force Method
	// DisableSteps keeps these steps in software.
	DisableSteps []compute.Step
	// SpriteFrames enables hardware decoding of the sprite frames. Off by default: every
	// frame is its own short ffmpeg run, where the GPU initialisation costs more than the
	// decode it saves (measured on an RTX 2080 Ti: 1080p 262 ms -> 508 ms per frame, 4K 563
	// -> 801 ms). A Bench showing it faster enables it too.
	SpriteFrames bool
	// Bench, when set, vetoes the steps it measured as not worth it (see Worth); a step
	// whose accelerated run failed in the bench stays in software.
	Bench *BenchReport
	// NoSoftwareFallback disables the software retry of a failed accelerated run.
	NoSoftwareFallback bool
}

func (p Prefs) disabled(m Method) bool {
	for _, d := range p.Disable {
		if d == m {
			return true
		}
	}
	return false
}

func (p Prefs) stepDisabled(s compute.Step) bool {
	for _, d := range p.DisableSteps {
		if d == s {
			return true
		}
	}
	return false
}

// variantOrder is the order in which the variants of a step are tried.
func variantOrder(step compute.Step, pref Preference) []Variant {
	switch step {
	case compute.StepVisualHash:
		if pref == PreferQuality {
			return []Variant{VariantDecode}
		}
		return []Variant{VariantGPUFrames, VariantDecode}
	case compute.StepSpriteFrame:
		return []Variant{VariantDecode}
	case compute.StepMarkerClip, compute.StepScenePreview:
		if pref == PreferSpeed {
			return []Variant{VariantSpeed, VariantQuality, VariantDecode}
		}
		return []Variant{VariantQuality, VariantSpeed, VariantDecode}
	}
	return nil
}

// Steps are the accelerated steps, in the order Choose labels the Accel.
var Steps = []compute.Step{compute.StepMarkerClip, compute.StepScenePreview, compute.StepVisualHash, compute.StepSpriteFrame}

// Choose picks the acceleration of every step from a Detect report: for each step, the
// first method (AllMethods order) with a proven pipeline, the variant following
// prefs.Prefer. A step without a proven pipeline runs in software. A nil report gives
// software (unless Force). The Accel's Name is the method of the first accelerated step
// ("none" when everything is software).
func Choose(r *Report, prefs Prefs) compute.Accel {
	acc := compute.Accel{Name: string(MethodNone), Steps: map[compute.Step]compute.StepAccel{}, NoSoftwareFallback: prefs.NoSoftwareFallback}
	if prefs.Force == MethodNone {
		return acc
	}
	var opts PipelineOptions
	if r != nil {
		opts = r.pipelineOptions()
	}
	methodsToTry := AllMethods
	if prefs.Force != "" {
		methodsToTry = []Method{prefs.Force}
	}

	for _, step := range Steps {
		if prefs.stepDisabled(step) {
			continue
		}
		benchEnabled := false
		if step == compute.StepSpriteFrame && !prefs.SpriteFrames {
			// only a bench that measured the sprite frames faster enables them
			if prefs.Bench == nil {
				continue
			}
			benchEnabled = true
		}
		m, sa, ok := pick(r, prefs, opts, methodsToTry, step)
		if !ok {
			continue
		}
		if prefs.Bench != nil {
			if sb := prefs.Bench.Step(step); sb != nil && sb.Accel == sa.String() {
				if !sb.Worth() {
					continue
				}
			} else if benchEnabled {
				continue
			}
		}
		acc.Steps[step] = sa
		if acc.Name == string(MethodNone) {
			acc.Name = string(m)
		}
	}
	return acc
}

// pick returns the first usable pipeline of step.
func pick(r *Report, prefs Prefs, opts PipelineOptions, methodsToTry []Method, step compute.Step) (Method, compute.StepAccel, bool) {
	for _, m := range methodsToTry {
		if prefs.disabled(m) {
			continue
		}
		order := variantOrder(step, prefs.Prefer)
		for _, v := range order {
			sa, ok := Pipeline(m, step, v, opts)
			if ok && r != nil && r.Proven(m, step, v) {
				return m, sa, true
			}
		}
		if prefs.Force == m {
			// forced: the first pipeline the method has, proven or not
			for _, v := range order {
				if sa, ok := Pipeline(m, step, v, opts); ok {
					return m, sa, true
				}
			}
		}
	}
	return "", compute.StepAccel{}, false
}
