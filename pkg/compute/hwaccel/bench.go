package hwaccel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"goonhub/pkg/compute"
	"goonhub/pkg/dhash"
)

// SampleSpec describes a generated sample video.
type SampleSpec struct {
	Codec   string // "h264" (default, libx264) or "hevc" (libx265)
	Width   int    // default 1920
	Height  int    // default 1080
	Seconds int    // default 20
	FPS     int    // default 30
	TenBit  bool   // 10-bit (HEVC Main10)
	// Source is the lavfi video source ("" = "testsrc2"; "mandelbrot" moves and has smooth
	// gradients, closer to filmed content for the hashes).
	Source string
	// Audio adds an AAC tone.
	Audio bool
}

func (s SampleSpec) withDefaults() SampleSpec {
	if s.Codec == "" {
		s.Codec = "h264"
	}
	if s.Width <= 0 {
		s.Width = 1920
	}
	if s.Height <= 0 {
		s.Height = 1080
	}
	if s.Seconds <= 0 {
		s.Seconds = 20
	}
	if s.FPS <= 0 {
		s.FPS = 30
	}
	if s.Source == "" {
		s.Source = "testsrc2"
	}
	return s
}

// Name is a file name for the sample ("h264_1920x1080_20s_testsrc2.mp4").
func (s SampleSpec) Name() string {
	s = s.withDefaults()
	bits := ""
	if s.TenBit {
		bits = "10"
	}
	return fmt.Sprintf("%s%s_%dx%d_%ds_%s.mp4", s.Codec, bits, s.Width, s.Height, s.Seconds, s.Source)
}

// GenerateSample renders a sample video at path with ffmpegPath.
func GenerateSample(ctx context.Context, ffmpegPath, path string, spec SampleSpec) error {
	s := spec.withDefaults()
	args := []string{"-hide_banner", "-v", "error", "-nostdin",
		"-f", "lavfi", "-i", fmt.Sprintf("%s=size=%dx%d:rate=%d", s.Source, s.Width, s.Height, s.FPS)}
	if s.Audio {
		args = append(args, "-f", "lavfi", "-i", "sine=frequency=440")
	}
	args = append(args, "-t", strconv.Itoa(s.Seconds))
	pix := "yuv420p"
	if s.TenBit {
		pix = "yuv420p10le"
	}
	switch s.Codec {
	case "h264":
		args = append(args, "-c:v", "libx264", "-preset", "veryfast", "-crf", "20")
	case "hevc":
		args = append(args, "-c:v", "libx265", "-preset", "fast", "-crf", "22", "-x265-params", "log-level=none", "-tag:v", "hvc1")
	default:
		return fmt.Errorf("unsupported sample codec %q", s.Codec)
	}
	args = append(args, "-pix_fmt", pix)
	if s.Audio {
		args = append(args, "-c:a", "aac", "-b:a", "96k")
	}
	if _, err := run(ctx, ffmpegBin(ffmpegPath), append(args, "-y", path)...); err != nil {
		return fmt.Errorf("failed to generate sample %s: %w", filepath.Base(path), err)
	}
	return nil
}

// Sample is the input of Bench and its workload.
type Sample struct {
	// Path is an existing video; "" generates one from Spec in TempDir.
	Path string
	Spec SampleSpec
	// Steps are the steps measured (nil = Steps).
	Steps []compute.Step
	// SpriteFrames is the number of sprite frames extracted, one ffmpeg run each, in
	// sequence (default 10).
	SpriteFrames int
	// Marker clip: width, CRF, length (defaults 480, 32, 10 s capped to half the sample).
	MarkerWidth, MarkerCRF, MarkerSeconds int
	// Scene preview: width, CRF, segments of 1 s (defaults 480, 27, 12).
	PreviewWidth, PreviewCRF, PreviewSegments int
	// Runs is the number of runs per measure, the fastest is kept (default 2).
	Runs int
	// TempDir holds the outputs ("" = the OS temp dir).
	TempDir string
	// SkipSSIM skips the SSIM of the marker clips.
	SkipSSIM bool
}

// Measure is one measured run (the fastest of Sample.Runs).
type Measure struct {
	WallMS      int64   `json:"wall_ms"`
	CPUMS       int64   `json:"cpu_ms"` // user + system time of ffmpeg
	OutputBytes int64   `json:"output_bytes,omitempty"`
	SSIM        float64 `json:"ssim,omitempty"` // marker clip vs the source scaled in software
}

// StepBench compares one step in software and accelerated.
type StepBench struct {
	Step        compute.Step `json:"step"`
	Accel       string       `json:"accel"` // StepAccel.String() of the accelerated run
	Software    Measure      `json:"software"`
	Accelerated *Measure     `json:"accelerated,omitempty"` // nil: the step is software in the Accel
	// Error is the failure of the accelerated run.
	Error string `json:"error,omitempty"`
	// Speedup = software wall / accelerated wall; CPURatio = accelerated CPU / software CPU;
	// SizeRatio = accelerated output / software output; SSIMDelta = accelerated - software.
	Speedup   float64    `json:"speedup,omitempty"`
	CPURatio  float64    `json:"cpu_ratio,omitempty"`
	SizeRatio float64    `json:"size_ratio,omitempty"`
	SSIMDelta float64    `json:"ssim_delta,omitempty"`
	HashDrift *HashDrift `json:"hash_drift,omitempty"` // visual hash vs software
}

// Worth reports whether the accelerated run is worth using: it succeeded and is not slower
// than software by more than 10 %, or it is at most 1.5x slower while saving at least half
// of the CPU time (a worker shares the machine).
func (s *StepBench) Worth() bool {
	if s.Accelerated == nil || s.Error != "" {
		return false
	}
	a, sw := float64(s.Accelerated.WallMS), float64(s.Software.WallMS)
	if a <= sw*1.10 {
		return true
	}
	return a <= sw*1.5 && float64(s.Accelerated.CPUMS) <= float64(s.Software.CPUMS)*0.5
}

// BenchReport is the result of Bench.
type BenchReport struct {
	Sample    string      `json:"sample"`
	Video     VideoProbe  `json:"video"`
	Accel     string      `json:"accel"`
	Steps     []StepBench `json:"steps"`
	ElapsedMS int64       `json:"elapsed_ms"`
}

// Step returns the measure of step (nil if not measured).
func (b *BenchReport) Step(step compute.Step) *StepBench {
	if b == nil {
		return nil
	}
	for i := range b.Steps {
		if b.Steps[i].Step == step {
			return &b.Steps[i]
		}
	}
	return nil
}

// Bench times each step in software and with accel on sample, with the exact ffmpeg
// commands of pkg/compute (accelerated runs without software fallback, so a failure shows).
func Bench(ctx context.Context, ffmpegPath string, accel compute.Accel, sample Sample) (*BenchReport, error) {
	start := time.Now()
	bin := ffmpegBin(ffmpegPath)
	dir, err := os.MkdirTemp(sample.TempDir, "goonhub-hwaccel-bench-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create bench directory: %w", err)
	}
	defer os.RemoveAll(dir)

	input := sample.Path
	if input == "" {
		input = filepath.Join(dir, sample.Spec.Name())
		if err := GenerateSample(ctx, bin, input, sample.Spec); err != nil {
			return nil, err
		}
	}
	info, err := ProbeVideo(ctx, ffprobePath(bin), input)
	if err != nil {
		return nil, err
	}
	if info.Duration < 2 {
		return nil, fmt.Errorf("sample too short (%.1fs)", info.Duration)
	}
	b := &benchRun{bin: bin, dir: dir, input: input, info: info, s: sample.withDefaults()}
	rep := &BenchReport{Sample: filepath.Base(input), Video: info, Accel: accel.Label()}

	steps := sample.Steps
	if steps == nil {
		steps = Steps
	}
	for _, step := range steps {
		sb := StepBench{Step: step}
		acc := accel.For(step)
		sw, swHashes, err := b.measure(ctx, step, compute.StepAccel{}, "sw")
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("software %s failed: %w", step, err)
		}
		sb.Software = sw
		if !acc.IsSoftware() {
			sb.Accel = acc.String()
			hw, hwHashes, err := b.measure(ctx, step, acc, "hw")
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			sb.Accelerated = &hw
			if err != nil {
				sb.Error = tail(err.Error(), 400)
			} else {
				sb.Speedup = ratio(sw.WallMS, hw.WallMS)
				sb.CPURatio = ratio(hw.CPUMS, sw.CPUMS)
				if sw.OutputBytes > 0 && hw.OutputBytes > 0 {
					sb.SizeRatio = float64(hw.OutputBytes) / float64(sw.OutputBytes)
				}
				if sw.SSIM > 0 && hw.SSIM > 0 {
					sb.SSIMDelta = hw.SSIM - sw.SSIM
				}
				if step == compute.StepVisualHash {
					d := CompareHashes(swHashes, hwHashes)
					sb.HashDrift = &d
				}
			}
		}
		rep.Steps = append(rep.Steps, sb)
	}
	rep.ElapsedMS = ms(time.Since(start))
	return rep, nil
}

func ratio(a, b int64) float64 {
	if b <= 0 {
		return 0
	}
	return float64(a) / float64(b)
}

func (s Sample) withDefaults() Sample {
	def := func(v *int, d int) {
		if *v <= 0 {
			*v = d
		}
	}
	def(&s.SpriteFrames, 10)
	def(&s.MarkerWidth, 480)
	def(&s.MarkerCRF, 32)
	def(&s.MarkerSeconds, 10)
	def(&s.PreviewWidth, 480)
	def(&s.PreviewCRF, 27)
	def(&s.PreviewSegments, 12)
	def(&s.Runs, 2)
	return s
}

type benchRun struct {
	bin, dir, input string
	info            VideoProbe
	s               Sample
}

// measure runs step Runs times with acc and keeps the fastest run.
func (b *benchRun) measure(ctx context.Context, step compute.Step, acc compute.StepAccel, tag string) (Measure, []uint64, error) {
	var best Measure
	var hashes []uint64
	for i := 0; i < b.s.Runs; i++ {
		m, h, err := b.once(ctx, step, acc, tag)
		if err != nil {
			return m, nil, err
		}
		if i == 0 || m.WallMS < best.WallMS {
			best, hashes = m, h
		}
	}
	if step == compute.StepMarkerClip && !b.s.SkipSSIM {
		// the clip of the last run is still there (same command every run)
		if v, err := b.markerSSIM(ctx, filepath.Join(b.dir, tag+"_marker.mp4")); err == nil {
			best.SSIM = v
		}
	}
	return best, hashes, nil
}

func (b *benchRun) markerWindow() (ts, length int) {
	dur := int(b.info.Duration)
	length = b.s.MarkerSeconds
	if length > dur/2 {
		length = dur / 2
	}
	if length < 1 {
		length = 1
	}
	return dur / 4, length
}

func (b *benchRun) once(ctx context.Context, step compute.Step, acc compute.StepAccel, tag string) (Measure, []uint64, error) {
	var m Measure
	switch step {
	case compute.StepSpriteFrame:
		dur := b.info.Duration
		for i := 0; i < b.s.SpriteFrames; i++ {
			ts := int((float64(i) + 0.5) * dur / float64(b.s.SpriteFrames))
			out := filepath.Join(b.dir, fmt.Sprintf("%s_frame_%03d.webp", tag, i))
			res, err := run(ctx, b.bin, compute.SpriteFrameArgs(acc, b.input, out, ts, 320, 180, 75)...)
			if err != nil {
				return m, nil, err
			}
			m.WallMS += ms(res.Wall)
			m.CPUMS += ms(res.CPU)
			m.OutputBytes += fileSize(out)
		}
		return m, nil, nil
	case compute.StepMarkerClip:
		ts, length := b.markerWindow()
		out := filepath.Join(b.dir, tag+"_marker.mp4")
		res, err := run(ctx, b.bin, compute.MarkerClipArgs(acc, b.input, out, ts, length, b.s.MarkerWidth, b.s.MarkerCRF)...)
		if err != nil {
			return m, nil, err
		}
		return Measure{WallMS: ms(res.Wall), CPUMS: ms(res.CPU), OutputBytes: fileSize(out)}, nil, nil
	case compute.StepScenePreview:
		out := filepath.Join(b.dir, tag+"_preview.mp4")
		res, err := run(ctx, b.bin, compute.ScenePreviewArgs(acc, b.input, out, int(b.info.Duration), b.s.PreviewSegments, 1.0, b.s.PreviewWidth, b.s.PreviewCRF, false)...)
		if err != nil {
			return m, nil, err
		}
		return Measure{WallMS: ms(res.Wall), CPUMS: ms(res.CPU), OutputBytes: fileSize(out)}, nil, nil
	case compute.StepVisualHash:
		res, err := run(ctx, b.bin, compute.VisualHashArgs(acc, b.input)...)
		if err != nil {
			return m, nil, err
		}
		hashes := make([]uint64, 0, len(res.Stdout)/72)
		for i := 0; i+72 <= len(res.Stdout); i += 72 {
			hashes = append(hashes, dhash.ComputeDHash(res.Stdout[i:i+72]))
		}
		if len(hashes) == 0 {
			return m, nil, errors.New("no visual frames")
		}
		return Measure{WallMS: ms(res.Wall), CPUMS: ms(res.CPU)}, hashes, nil
	}
	return m, nil, fmt.Errorf("unsupported step %q", step)
}

var ssimRe = regexp.MustCompile(`All:([0-9.]+)`)

// markerSSIM is the SSIM of a marker clip against the same window of the source scaled
// with the software scaler (no compression): the quality the encoder kept.
func (b *benchRun) markerSSIM(ctx context.Context, clip string) (float64, error) {
	ts, length := b.markerWindow()
	return SSIMAgainstSource(ctx, b.bin, b.input, clip, ts, length, b.s.MarkerWidth)
}

// SSIMAgainstSource returns the SSIM of clip (a marker clip of width px cut at ts for length
// seconds) against the source window scaled to width with the software bilinear scaler.
func SSIMAgainstSource(ctx context.Context, ffmpegPath, source, clip string, ts, length, width int) (float64, error) {
	res, err := run(ctx, ffmpegBin(ffmpegPath), "-hide_banner", "-nostdin",
		"-ss", strconv.Itoa(ts), "-i", source, "-i", clip, "-t", strconv.Itoa(length),
		"-lavfi", fmt.Sprintf("[0:v]scale=%d:-2:flags=bilinear,format=yuv420p[ref];[1:v]format=yuv420p[d];[d][ref]ssim", width),
		"-f", "null", "-")
	if err != nil {
		return 0, err
	}
	m := ssimRe.FindAllStringSubmatch(string(res.Stderr), -1)
	if len(m) == 0 {
		return 0, errors.New("no SSIM in the ffmpeg output")
	}
	return strconv.ParseFloat(m[len(m)-1][1], 64)
}

func fileSize(path string) int64 {
	st, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return st.Size()
}
