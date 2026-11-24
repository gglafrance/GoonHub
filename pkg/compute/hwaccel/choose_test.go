package hwaccel

import (
	"testing"

	"goonhub/pkg/compute"
)

// fakeReport proves the given (method, step, variant) triples.
func fakeReport(goos string, proven map[Method][]Candidate) *Report {
	r := &Report{OS: goos, Filters: []string{"scale_cuda"}}
	for _, m := range AllMethods {
		mr := MethodReport{Method: m}
		for _, c := range proven[m] {
			c.OK = true
			mr.Candidates = append(mr.Candidates, c)
		}
		r.Methods = append(r.Methods, mr)
	}
	return r
}

func all(m Method) []Candidate {
	var out []Candidate
	for _, s := range Steps {
		for _, v := range candidates(m, s) {
			out = append(out, Candidate{Step: s, Variant: v})
		}
	}
	return out
}

func TestChoose_NilReportIsSoftware(t *testing.T) {
	acc := Choose(nil, Prefs{})
	if acc.Label() != "none" || len(acc.Steps) != 0 {
		t.Fatalf("got %+v", acc)
	}
}

func TestChoose_NVIDIA(t *testing.T) {
	r := fakeReport("windows", map[Method][]Candidate{MethodNVIDIA: all(MethodNVIDIA), MethodAMD: all(MethodAMD), MethodD3D11VA: all(MethodD3D11VA)})

	acc := Choose(r, Prefs{})
	if acc.Name != "nvidia" {
		t.Fatalf("name %q", acc.Name)
	}
	if got := acc.For(compute.StepMarkerClip); got.Encoder.H264 != "h264_nvenc" || got.Scale.Filter != "" || got.Decode.HWAccel != "cuda" {
		t.Fatalf("balanced marker clip = %s", got)
	}
	if got := acc.For(compute.StepVisualHash); got.Decode.OutputFormat != "cuda" || got.Scale.DownloadFormat != "nv12" {
		t.Fatalf("balanced visual hash = %+v", got)
	}
	if !acc.For(compute.StepSpriteFrame).IsSoftware() {
		t.Fatal("sprite frames stay in software by default")
	}

	speed := Choose(r, Prefs{Prefer: PreferSpeed})
	if got := speed.For(compute.StepMarkerClip); got.Scale.Filter != "scale_cuda" {
		t.Fatalf("speed marker clip = %s", got)
	}
	if got := speed.For(compute.StepScenePreview); got.Decode.HWAccel != "" || got.Encoder.H264 != "h264_nvenc" {
		t.Fatalf("speed preview = %s (software decode, NVENC)", got)
	}

	quality := Choose(r, Prefs{Prefer: PreferQuality})
	if got := quality.For(compute.StepVisualHash); got.Decode.OutputFormat != "" || got.Scale.DownloadFormat != "" || got.Decode.HWAccel != "cuda" {
		t.Fatalf("quality visual hash must be decode-only: %+v", got)
	}

	sprites := Choose(r, Prefs{SpriteFrames: true})
	if got := sprites.For(compute.StepSpriteFrame); got.Decode.HWAccel != "cuda" {
		t.Fatalf("sprite frames = %s", got)
	}
}

func TestChoose_FallsToNextProvenMethod(t *testing.T) {
	r := fakeReport("windows", map[Method][]Candidate{
		MethodNVIDIA:  {{Step: compute.StepVisualHash, Variant: VariantDecode}},
		MethodAMD:     all(MethodAMD),
		MethodD3D11VA: all(MethodD3D11VA),
	})
	acc := Choose(r, Prefs{})
	// nvidia has no proven encode pipeline: d3d11va decode + libx264 before AMF
	if got := acc.For(compute.StepMarkerClip); got.Decode.HWAccel != "d3d11va" || got.Encoder.H264 != "" {
		t.Fatalf("marker clip = %s, want d3d11va decode + libx264", got)
	}
	// d3d11va has no preview pipeline (one decoder per segment): AMF encodes it
	if got := acc.For(compute.StepScenePreview); got.Encoder.H264 != "h264_amf" || got.Decode.HWAccel != "" {
		t.Fatalf("preview = %s, want amd", got)
	}
	// nvidia gpu_frames not proven: decode variant of nvidia (first method) wins
	if got := acc.For(compute.StepVisualHash); got.Decode.HWAccel != "cuda" || got.Decode.OutputFormat != "" {
		t.Fatalf("visual hash = %+v", got)
	}
	if acc.Name != "d3d11va" {
		t.Fatalf("label = %q (first accelerated step is the marker clip)", acc.Name)
	}

	acc = Choose(r, Prefs{Disable: []Method{MethodNVIDIA, MethodD3D11VA}})
	if got := acc.For(compute.StepMarkerClip); got.Encoder.H264 != "h264_amf" {
		t.Fatalf("marker clip = %s, want amd", got)
	}
	if !acc.For(compute.StepVisualHash).IsSoftware() || acc.Name != "amd" {
		t.Fatalf("accel = %+v", acc)
	}
}

func TestChoose_ForceAndDisableSteps(t *testing.T) {
	r := fakeReport("linux", map[Method][]Candidate{MethodNVIDIA: all(MethodNVIDIA)})

	if acc := Choose(r, Prefs{Force: MethodNone}); acc.Label() != "none" || len(acc.Steps) != 0 {
		t.Fatalf("force none = %+v", acc)
	}

	// forced but unproven: the method's pipelines anyway (software fallback stays)
	acc := Choose(r, Prefs{Force: MethodVAAPI})
	if got := acc.For(compute.StepMarkerClip); got.Encoder.H264 != "h264_vaapi" {
		t.Fatalf("forced vaapi marker = %s", got)
	}
	if acc.Name != "vaapi" || acc.NoSoftwareFallback {
		t.Fatalf("forced accel %+v", acc)
	}

	acc = Choose(r, Prefs{DisableSteps: []compute.Step{compute.StepVisualHash}, NoSoftwareFallback: true})
	if !acc.For(compute.StepVisualHash).IsSoftware() || acc.For(compute.StepMarkerClip).IsSoftware() || !acc.NoSoftwareFallback {
		t.Fatalf("disable steps = %+v", acc)
	}
}

func TestChoose_BenchVetoesSlowSteps(t *testing.T) {
	r := fakeReport("windows", map[Method][]Candidate{MethodNVIDIA: all(MethodNVIDIA)})
	base := Choose(r, Prefs{SpriteFrames: true})
	bench := &BenchReport{Steps: []StepBench{
		// slower and no CPU saved: vetoed
		{Step: compute.StepMarkerClip, Accel: base.For(compute.StepMarkerClip).String(),
			Software: Measure{WallMS: 1000, CPUMS: 4000}, Accelerated: &Measure{WallMS: 1300, CPUMS: 3500}},
		// slower but saves the CPU: kept
		{Step: compute.StepVisualHash, Accel: base.For(compute.StepVisualHash).String(),
			Software: Measure{WallMS: 1000, CPUMS: 8000}, Accelerated: &Measure{WallMS: 1400, CPUMS: 1000}},
		// faster: enables the sprite frames even without Prefs.SpriteFrames
		{Step: compute.StepSpriteFrame, Accel: base.For(compute.StepSpriteFrame).String(),
			Software: Measure{WallMS: 1000, CPUMS: 1000}, Accelerated: &Measure{WallMS: 800, CPUMS: 500}},
		// failed in the bench
		{Step: compute.StepScenePreview, Accel: base.For(compute.StepScenePreview).String(),
			Software: Measure{WallMS: 1000}, Accelerated: &Measure{}, Error: "boom"},
	}}
	acc := Choose(r, Prefs{Bench: bench})
	if !acc.For(compute.StepMarkerClip).IsSoftware() {
		t.Fatal("marker clip should be vetoed")
	}
	if !acc.For(compute.StepScenePreview).IsSoftware() {
		t.Fatal("failed preview should be vetoed")
	}
	if acc.For(compute.StepVisualHash).IsSoftware() {
		t.Fatal("visual hash saves CPU, should stay accelerated")
	}
	if acc.For(compute.StepSpriteFrame).IsSoftware() {
		t.Fatal("faster sprite frames should be enabled by the bench")
	}
	if acc.Name != "nvidia" {
		t.Fatalf("label %q", acc.Name)
	}

	// a bench of another pipeline does not apply
	other := &BenchReport{Steps: []StepBench{{Step: compute.StepMarkerClip, Accel: "decode=x scale=sw encode=y",
		Software: Measure{WallMS: 1}, Accelerated: &Measure{WallMS: 100, CPUMS: 100}}}}
	if Choose(r, Prefs{Bench: other}).For(compute.StepMarkerClip).IsSoftware() {
		t.Fatal("bench of another pipeline must be ignored")
	}
}

func TestStepBench_Worth(t *testing.T) {
	tests := []struct {
		name string
		s    StepBench
		want bool
	}{
		{"not accelerated", StepBench{Software: Measure{WallMS: 10}}, false},
		{"failed", StepBench{Software: Measure{WallMS: 10}, Accelerated: &Measure{WallMS: 1}, Error: "x"}, false},
		{"faster", StepBench{Software: Measure{WallMS: 100, CPUMS: 100}, Accelerated: &Measure{WallMS: 50, CPUMS: 100}}, true},
		{"10% slower", StepBench{Software: Measure{WallMS: 100, CPUMS: 100}, Accelerated: &Measure{WallMS: 110, CPUMS: 100}}, true},
		{"slower, cpu saved", StepBench{Software: Measure{WallMS: 100, CPUMS: 100}, Accelerated: &Measure{WallMS: 150, CPUMS: 50}}, true},
		{"much slower", StepBench{Software: Measure{WallMS: 100, CPUMS: 100}, Accelerated: &Measure{WallMS: 151, CPUMS: 10}}, false},
		{"slower, cpu not saved", StepBench{Software: Measure{WallMS: 100, CPUMS: 100}, Accelerated: &Measure{WallMS: 120, CPUMS: 60}}, false},
	}
	for _, tt := range tests {
		if got := tt.s.Worth(); got != tt.want {
			t.Fatalf("%s: Worth = %v", tt.name, got)
		}
	}
}

func TestCompareHashes(t *testing.T) {
	ref := []uint64{0, 0xFF, 0xFFFF, 1}
	got := []uint64{0, 0xFE, 0xFFFF00, 1, 7}
	d := CompareHashes(ref, got)
	if d.Frames != 5 || d.ReferenceFrames != 4 || d.Compared != 4 {
		t.Fatalf("counts %+v", d)
	}
	// distances 0, 1, 16, 0
	if d.Exact != 2 || d.Within3 != 3 || d.Within5 != 3 || d.Max != 16 || d.Mean != 17.0/4 {
		t.Fatalf("drift %+v", d)
	}
	if r := d.Within5Ratio(); r != 0.75 {
		t.Fatalf("ratio %v", r)
	}
	if (HashDrift{}).Within5Ratio() != 0 {
		t.Fatal("empty ratio")
	}
}

func TestReportHelpers(t *testing.T) {
	r := fakeReport("windows", map[Method][]Candidate{MethodAMD: all(MethodAMD)})
	r.Encoders = []string{"libx264", "h264_amf"}
	r.Method(MethodAMD).Probes = []Probe{{Name: ProbeEncodeH264, OK: true}}
	r.Method(MethodNVIDIA).Probes = []Probe{{Name: ProbeDecodeH264, OK: true}, {Name: ProbeEncodeH264, OK: false}}
	if got := r.ProvenHWAccels(); len(got) != 1 || got[0] != "cuda" {
		t.Fatalf("hwaccels %v", got)
	}
	if got := r.ProvenEncoders(); len(got) != 2 || got[0] != "h264_amf" || got[1] != "libx264" {
		t.Fatalf("encoders %v", got)
	}
	if got := r.Working(); len(got) != 1 || got[0] != MethodAMD {
		t.Fatalf("working %v", got)
	}
	if !r.Proven(MethodAMD, compute.StepMarkerClip, VariantQuality) || r.Proven(MethodAMD, compute.StepVisualHash, VariantDecode) {
		t.Fatal("Proven")
	}
	if r.Summary() == "" {
		t.Fatal("summary")
	}
}
