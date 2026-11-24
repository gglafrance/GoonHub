package hwaccel

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"goonhub/pkg/compute"
	"goonhub/pkg/dhash"
	"goonhub/pkg/workerproto"
)

// Report is what Detect found. Only what a real run proved counts: a method compiled into
// ffmpeg but without a working device has failed probes.
type Report struct {
	FFmpegPath    string `json:"ffmpeg_path"`
	FFmpegVersion string `json:"ffmpeg_version"` // "N-124716-g054dffd133-20260531", "7.1.1"...
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	// HWAccels, Encoders, Filters: what ffmpeg was built with (-hwaccels, the H.264
	// encoders of -encoders, the GPU filters of -filters), proven or not.
	HWAccels []string `json:"hwaccels"`
	Encoders []string `json:"encoders"`
	Filters  []string `json:"filters"`
	// Devices are GPU names found on a best effort basis (nvidia-smi, sysctl).
	Devices     []string       `json:"devices,omitempty"`
	VAAPIDevice string         `json:"vaapi_device,omitempty"`
	Methods     []MethodReport `json:"methods"`
	StartedAt   time.Time      `json:"started_at"`
	ElapsedMS   int64          `json:"elapsed_ms"`
}

// MethodReport is the result of one method.
type MethodReport struct {
	Method Method `json:"method"`
	// Skipped tells why the method was not probed (other OS, not compiled in, no device).
	Skipped string `json:"skipped,omitempty"`
	// Probes are the capability runs: decode_h264, decode_hevc, decode_hevc10 (strict:
	// frames must come out of the GPU), encode_h264, scale.
	Probes []Probe `json:"probes,omitempty"`
	// Candidates are the step pipelines run end to end through pkg/compute (without
	// software fallback) and checked.
	Candidates []Candidate `json:"candidates,omitempty"`
}

// Probe names.
const (
	ProbeDecodeH264   = "decode_h264"
	ProbeDecodeHEVC   = "decode_hevc"
	ProbeDecodeHEVC10 = "decode_hevc10"
	ProbeEncodeH264   = "encode_h264"
	ProbeScale        = "scale"
)

// Probe is one capability run.
type Probe struct {
	Name      string `json:"name"`
	Target    string `json:"target"` // hwaccel, encoder or filter name
	OK        bool   `json:"ok"`
	ElapsedMS int64  `json:"elapsed_ms"`
	Error     string `json:"error,omitempty"`
}

// Candidate is one step pipeline proven (or not) on the probe clip.
type Candidate struct {
	Step      compute.Step `json:"step"`
	Variant   Variant      `json:"variant"`
	Accel     string       `json:"accel"` // StepAccel.String()
	OK        bool         `json:"ok"`
	ElapsedMS int64        `json:"elapsed_ms"`
	Error     string       `json:"error,omitempty"`
}

// Method returns the report of m (nil if absent).
func (r *Report) Method(m Method) *MethodReport {
	for i := range r.Methods {
		if r.Methods[i].Method == m {
			return &r.Methods[i]
		}
	}
	return nil
}

// ProbeOK reports whether probe name of method m succeeded.
func (r *Report) ProbeOK(m Method, name string) bool {
	mr := r.Method(m)
	if mr == nil {
		return false
	}
	for _, p := range mr.Probes {
		if p.Name == name {
			return p.OK
		}
	}
	return false
}

// Proven reports whether the pipeline (m, step, v) ran successfully.
func (r *Report) Proven(m Method, step compute.Step, v Variant) bool {
	mr := r.Method(m)
	if mr == nil {
		return false
	}
	for _, c := range mr.Candidates {
		if c.Step == step && c.Variant == v {
			return c.OK
		}
	}
	return false
}

// Working returns the methods with at least one proven pipeline, in preference order.
func (r *Report) Working() []Method {
	var out []Method
	for _, m := range AllMethods {
		if mr := r.Method(m); mr != nil {
			for _, c := range mr.Candidates {
				if c.OK {
					out = append(out, m)
					break
				}
			}
		}
	}
	return out
}

// ProvenHWAccels returns the -hwaccel values whose strict H.264 decode probe succeeded
// (workerproto.Capabilities.HWAccels).
func (r *Report) ProvenHWAccels() []string {
	var out []string
	for _, m := range AllMethods {
		if r.ProbeOK(m, ProbeDecodeH264) {
			out = append(out, m.HWAccel())
		}
	}
	return out
}

// ProvenEncoders returns the H.264 encoders that encoded the probe, plus libx264 when ffmpeg
// has it (workerproto.Capabilities.Encoders).
func (r *Report) ProvenEncoders() []string {
	var out []string
	for _, m := range AllMethods {
		if m.Encoder() != "" && r.ProbeOK(m, ProbeEncodeH264) {
			out = append(out, m.Encoder())
		}
	}
	if contains(r.Encoders, "libx264") {
		out = append(out, "libx264")
	}
	return out
}

// pipelineOptions are the PipelineOptions of the machine the report describes.
func (r *Report) pipelineOptions() PipelineOptions {
	return PipelineOptions{Filters: r.Filters, VAAPIDevice: r.VAAPIDevice, GOOS: r.OS}
}

// Summary is a one-line description for logs.
func (r *Report) Summary() string {
	var parts []string
	for _, mr := range r.Methods {
		if mr.Skipped != "" {
			continue
		}
		var ok []string
		for _, c := range mr.Candidates {
			if c.OK {
				ok = append(ok, string(c.Step)+"/"+string(c.Variant))
			}
		}
		if len(ok) == 0 {
			parts = append(parts, string(mr.Method)+": none")
		} else {
			parts = append(parts, string(mr.Method)+": "+strings.Join(ok, ","))
		}
	}
	return fmt.Sprintf("ffmpeg %s; %s", r.FFmpegVersion, strings.Join(parts, "; "))
}

// DetectOptions tune Detect.
type DetectOptions struct {
	// Methods limits the methods probed (nil = every method of this OS).
	Methods []Method
	// VAAPIDevice is the DRM render node ("" = DefaultVAAPIDevice).
	VAAPIDevice string
	// TempDir holds the probe clips and outputs ("" = the OS temp dir).
	TempDir string
	// ProbeTimeout bounds each run (0 = 60 s; the first CUDA initialisation can take
	// seconds).
	ProbeTimeout time.Duration
	// Sequential probes the methods one after the other (default: in parallel, faster;
	// sequential gives cleaner timings).
	Sequential bool
}

// Detect finds the hardware acceleration ffmpegPath ("" = ffmpeg from PATH) can really use:
// it parses -hwaccels / -encoders / -filters, then proves every candidate with real runs
// on a small generated clip. The error is only for an unusable ffmpeg; a machine without
// any GPU gives a report where nothing is proven.
func Detect(ctx context.Context, ffmpegPath string) (*Report, error) {
	return DetectWith(ctx, ffmpegPath, DetectOptions{})
}

// DetectWith is Detect with options.
func DetectWith(ctx context.Context, ffmpegPath string, opts DetectOptions) (*Report, error) {
	start := time.Now()
	bin := ffmpegBin(ffmpegPath)
	if opts.ProbeTimeout <= 0 {
		opts.ProbeTimeout = 60 * time.Second
	}
	r := &Report{FFmpegPath: bin, OS: runtime.GOOS, Arch: runtime.GOARCH, StartedAt: start.UTC()}
	if runtime.GOOS == "linux" {
		r.VAAPIDevice = opts.VAAPIDevice
		if r.VAAPIDevice == "" {
			r.VAAPIDevice = DefaultVAAPIDevice
		}
	}

	ver, err := run(ctx, bin, "-hide_banner", "-version")
	if err != nil {
		return nil, fmt.Errorf("ffmpeg is not usable: %w", err)
	}
	r.FFmpegVersion = ParseVersion(string(ver.Stdout))
	if out, err := run(ctx, bin, "-hide_banner", "-hwaccels"); err == nil {
		r.HWAccels = ParseHWAccels(string(out.Stdout))
	}
	encoders := map[string]bool{}
	if out, err := run(ctx, bin, "-hide_banner", "-encoders"); err == nil {
		for _, e := range ParseCodecList(string(out.Stdout)) {
			encoders[e] = true
		}
	}
	for _, e := range []string{"libx264", "libopenh264", "libx265", "h264_nvenc", "h264_qsv", "h264_amf", "h264_vaapi", "h264_videotoolbox"} {
		if encoders[e] {
			r.Encoders = append(r.Encoders, e)
		}
	}
	if out, err := run(ctx, bin, "-hide_banner", "-filters"); err == nil {
		all := map[string]bool{}
		for _, f := range ParseFilters(string(out.Stdout)) {
			all[f] = true
		}
		for _, f := range []string{"hwdownload", "hwupload", "scale_cuda", "scale_npp", "scale_qsv", "vpp_qsv", "scale_vaapi", "scale_vt", "scale_d3d11"} {
			if all[f] {
				r.Filters = append(r.Filters, f)
			}
		}
	}
	r.Devices = gpuNames(ctx)

	dir, err := os.MkdirTemp(opts.TempDir, "goonhub-hwaccel-detect-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create probe directory: %w", err)
	}
	defer os.RemoveAll(dir)
	clips, clipErr := makeProbeClips(ctx, bin, dir, encoders)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	wanted := opts.Methods
	if wanted == nil {
		wanted = AllMethods
	}
	r.Methods = make([]MethodReport, len(wanted))
	var wg sync.WaitGroup
	for i, m := range wanted {
		r.Methods[i] = MethodReport{Method: m}
		mr := &r.Methods[i]
		if reason := skipReason(m, r, encoders); reason != "" {
			mr.Skipped = reason
			continue
		}
		if clipErr != nil {
			mr.Skipped = clipErr.Error()
			continue
		}
		p := &prober{bin: bin, probe: ffprobePath(bin), dir: filepath.Join(dir, string(m)), clips: clips, report: r, timeout: opts.ProbeTimeout}
		if opts.Sequential {
			p.method(ctx, mr)
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.method(ctx, mr)
		}()
	}
	wg.Wait()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	r.ElapsedMS = ms(time.Since(start))
	return r, nil
}

// skipReason tells why m cannot be probed here ("" = probe it).
func skipReason(m Method, r *Report, encoders map[string]bool) string {
	if !m.Valid() || m == MethodNone {
		return "unknown method"
	}
	if !m.onOS(r.OS) {
		return "not available on " + r.OS
	}
	info := m.info()
	hasDecode := info.hwaccel != "" && contains(r.HWAccels, info.hwaccel)
	hasEncode := info.encoder != "" && encoders[info.encoder]
	if !hasDecode && !hasEncode {
		return "ffmpeg is built without " + strings.Trim(info.hwaccel+" / "+info.encoder, " /")
	}
	if m == MethodVAAPI {
		if _, err := os.Stat(r.VAAPIDevice); err != nil {
			return "no VA-API device " + r.VAAPIDevice
		}
	}
	return ""
}

// probeClips are the generated inputs of the probes.
type probeClips struct {
	h264, hevc, hevc10 string
	duration           int
	h264Hashes         []uint64 // software visual hashes of h264
}

const probeClipSeconds = 4

// makeProbeClips renders a 4 s 640x360 test clip in H.264 (required) and HEVC 8/10-bit
// (when libx265 is there).
func makeProbeClips(ctx context.Context, bin, dir string, encoders map[string]bool) (probeClips, error) {
	c := probeClips{duration: probeClipSeconds}
	src := fmt.Sprintf("testsrc2=size=640x360:rate=30:duration=%d", probeClipSeconds)
	var h264Enc []string
	switch {
	case encoders["libx264"]:
		h264Enc = []string{"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p"}
	case encoders["libopenh264"]:
		h264Enc = []string{"-c:v", "libopenh264", "-pix_fmt", "yuv420p"}
	default:
		return c, errors.New("ffmpeg has no software H.264 encoder to make the probe clip")
	}
	c.h264 = filepath.Join(dir, "probe_h264.mp4")
	args := append([]string{"-hide_banner", "-v", "error", "-nostdin", "-f", "lavfi", "-i", src}, h264Enc...)
	if _, err := run(ctx, bin, append(args, "-y", c.h264)...); err != nil {
		return c, fmt.Errorf("failed to make the H.264 probe clip: %w", err)
	}
	if encoders["libx265"] {
		for _, v := range []struct {
			path *string
			name string
			pix  string
		}{{&c.hevc, "probe_hevc.mp4", "yuv420p"}, {&c.hevc10, "probe_hevc10.mp4", "yuv420p10le"}} {
			p := filepath.Join(dir, v.name)
			_, err := run(ctx, bin, "-hide_banner", "-v", "error", "-nostdin", "-f", "lavfi", "-i", src,
				"-c:v", "libx265", "-preset", "ultrafast", "-x265-params", "log-level=none", "-pix_fmt", v.pix, "-y", p)
			if err == nil {
				*v.path = p
			}
		}
	}
	hashes, err := dhash.RunFFmpeg(ctx, bin, dhash.DefaultArgs(c.h264))
	if err != nil {
		return c, fmt.Errorf("failed to hash the probe clip: %w", err)
	}
	c.h264Hashes = hashes
	return c, nil
}

// prober runs the probes of one method.
type prober struct {
	bin, probe string
	dir        string
	clips      probeClips
	report     *Report
	timeout    time.Duration
}

func (p *prober) method(ctx context.Context, mr *MethodReport) {
	m := mr.Method
	info := m.info()
	if err := os.MkdirAll(p.dir, 0o755); err != nil {
		mr.Skipped = "failed to create probe directory: " + err.Error()
		return
	}
	opts := p.report.pipelineOptions()

	if info.hwaccel != "" && contains(p.report.HWAccels, info.hwaccel) {
		mr.Probes = append(mr.Probes, p.decodeProbe(ctx, m, ProbeDecodeH264, p.clips.h264, "nv12"))
		if p.clips.hevc != "" {
			mr.Probes = append(mr.Probes, p.decodeProbe(ctx, m, ProbeDecodeHEVC, p.clips.hevc, "nv12"))
		}
		if p.clips.hevc10 != "" {
			mr.Probes = append(mr.Probes, p.decodeProbe(ctx, m, ProbeDecodeHEVC10, p.clips.hevc10, "p010le"))
		}
	}
	if info.encoder != "" && contains(p.report.Encoders, info.encoder) {
		mr.Probes = append(mr.Probes, p.encodeProbe(ctx, m, opts))
	}
	if speed, ok := Pipeline(m, compute.StepMarkerClip, VariantSpeed, opts); ok && speed.Scale.Filter != "" && contains(p.report.Filters, speed.Scale.Filter) {
		mr.Probes = append(mr.Probes, p.scaleProbe(ctx, speed))
	}

	for _, step := range []compute.Step{compute.StepVisualHash, compute.StepSpriteFrame, compute.StepMarkerClip, compute.StepScenePreview} {
		for _, v := range candidates(m, step) {
			acc, _ := Pipeline(m, step, v, opts)
			c := Candidate{Step: step, Variant: v, Accel: acc.String()}
			if reason := p.missing(mr, acc); reason != "" {
				c.Error = reason
			} else {
				start := time.Now()
				err := p.candidate(ctx, m, step, acc)
				c.ElapsedMS = ms(time.Since(start))
				c.OK = err == nil
				if err != nil {
					c.Error = tail(err.Error(), 400)
				}
			}
			mr.Candidates = append(mr.Candidates, c)
		}
	}
}

// missing returns why acc cannot work given the failed probes ("" = run it).
func (p *prober) missing(mr *MethodReport, acc compute.StepAccel) string {
	probeOK := func(name string) bool {
		for _, pr := range mr.Probes {
			if pr.Name == name {
				return pr.OK
			}
		}
		return false
	}
	if acc.Decode.HWAccel != "" && !probeOK(ProbeDecodeH264) {
		return "hardware decode not proven"
	}
	if acc.Encoder.H264 != "" && !probeOK(ProbeEncodeH264) {
		return "encoder " + acc.Encoder.H264 + " not proven"
	}
	if acc.Scale.Filter != "" && !probeOK(ProbeScale) {
		return "scale filter " + acc.Scale.Filter + " not proven"
	}
	return ""
}

func (p *prober) timed(ctx context.Context, name, target string, args []string) Probe {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	res, err := run(ctx, p.bin, args...)
	pr := Probe{Name: name, Target: target, ElapsedMS: ms(res.Wall), OK: err == nil}
	if err != nil {
		pr.Error = tail(err.Error(), 400)
	}
	return pr
}

// strictDecode are the input options of a decode whose frames stay in GPU memory: the
// hwdownload after it fails when ffmpeg silently fell back to software decoding.
func strictDecode(m Method, o PipelineOptions) []string {
	info := m.info()
	args := []string{"-hwaccel", info.hwaccel, "-hwaccel_output_format", info.outputFormat}
	if m == MethodVAAPI {
		args = append(args, "-hwaccel_device", o.vaapiDevice())
	}
	return args
}

func (p *prober) decodeProbe(ctx context.Context, m Method, name, clip, downloadFormat string) Probe {
	args := []string{"-hide_banner", "-v", "error", "-nostdin"}
	args = append(args, strictDecode(m, p.report.pipelineOptions())...)
	args = append(args, "-i", clip, "-vf", "hwdownload,format="+downloadFormat, "-f", "null", "-")
	return p.timed(ctx, name, m.HWAccel(), args)
}

// encodeProbe encodes generated frames with the encoder and its real quality options.
func (p *prober) encodeProbe(ctx context.Context, m Method, o PipelineOptions) Probe {
	enc := Encoder(m)
	acc := compute.StepAccel{Encoder: enc}
	args := []string{"-hide_banner", "-v", "error", "-nostdin"}
	if m == MethodVAAPI {
		args = append(args, "-init_hw_device", "vaapi="+sharedDevice+":"+o.vaapiDevice(), "-filter_hw_device", sharedDevice)
	}
	args = append(args, "-f", "lavfi", "-i", "testsrc2=size=640x360:rate=30:duration=1")
	if m == MethodVAAPI {
		args = append(args, "-vf", "format=nv12,hwupload")
	} else {
		pix := enc.PixFmt
		if pix == "" {
			pix = "yuv420p"
		}
		args = append(args, "-pix_fmt", pix)
	}
	args = append(args, "-c:v", enc.H264)
	args = append(args, acc.QualityArgs(27)...)
	args = append(args, "-f", "null", "-")
	return p.timed(ctx, ProbeEncodeH264, enc.H264, args)
}

// scaleProbe runs the GPU scale filter of the speed pipeline on decoded GPU frames.
func (p *prober) scaleProbe(ctx context.Context, acc compute.StepAccel) Probe {
	f := acc.Scale.Filter + "=320:-2"
	if acc.Scale.Options != "" {
		f += ":" + acc.Scale.Options
	}
	args := []string{"-hide_banner", "-v", "error", "-nostdin"}
	args = append(args, acc.Decode.GlobalArgs...)
	args = append(args, "-hwaccel", acc.Decode.HWAccel, "-hwaccel_output_format", acc.Decode.OutputFormat)
	if acc.Decode.Device != "" {
		args = append(args, "-hwaccel_device", acc.Decode.Device)
	}
	args = append(args, "-i", p.clips.h264, "-vf", f+",hwdownload,format=nv12", "-f", "null", "-")
	return p.timed(ctx, ProbeScale, acc.Scale.Filter, args)
}

// candidate runs step through pkg/compute with acc only (no software fallback) and checks
// the output.
func (p *prober) candidate(ctx context.Context, m Method, step compute.Step, acc compute.StepAccel) error {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	out, err := os.MkdirTemp(p.dir, string(step)+"-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(out)
	env := compute.Env{
		FFmpegPath: p.bin, FFprobePath: p.probe, TempDir: out,
		Accel: compute.Accel{Name: string(m), Steps: map[compute.Step]compute.StepAccel{step: acc}, NoSoftwareFallback: true},
	}
	req := compute.Request{Input: p.clips.h264, Video: workerproto.VideoInfo{DurationSec: p.clips.duration, Width: 640, Height: 360, VideoCodec: "h264"}, OutDir: out}

	switch step {
	case compute.StepVisualHash:
		res, err := compute.Fingerprint(ctx, env, req, workerproto.FingerprintParams{Mode: workerproto.FingerprintModeAudioOnly})
		if err != nil {
			return err
		}
		d := CompareHashes(p.clips.h264Hashes, res.VisualFingerprint)
		if d.Frames != d.ReferenceFrames {
			return fmt.Errorf("%d hashes, software gives %d", d.Frames, d.ReferenceFrames)
		}
		// a broken GPU path gives garbage frames: random hashes differ by ~32 bits
		if d.Mean > 8 {
			return fmt.Errorf("hashes differ from software by %.1f bits on average", d.Mean)
		}
		return nil
	case compute.StepSpriteFrame:
		res, err := compute.Sprites(ctx, env, req, workerproto.SpritesParams{
			SceneID: 1, TileWidth: 160, TileHeight: 90, FrameInterval: 2, FrameQuality: 75, GridCols: 2, GridRows: 1, Concurrency: 1,
		})
		if err != nil {
			return err
		}
		if res.SheetCount != 1 {
			return fmt.Errorf("%d sprite sheets, want 1", res.SheetCount)
		}
		return p.checkImage(ctx, filepath.Join(out, res.SpriteSheets[0]), 320, 90)
	case compute.StepMarkerClip:
		clip := filepath.Join(out, workerproto.MarkerClipName(1))
		if err := compute.MarkerClip(ctx, env, p.clips.h264, clip, 1, 2, 320, 27, p.timeout); err != nil {
			return err
		}
		return p.checkVideo(ctx, clip, 320, 180, 2)
	case compute.StepScenePreview:
		params := workerproto.AnimatedThumbnailsParams{
			SceneID: 1, MarkerWidth: 320, MarkerCRF: 27,
			Preview: &workerproto.ScenePreviewSpec{Output: workerproto.ScenePreviewName(1), Segments: 2, SegmentDuration: 1, Width: 320, CRF: 27},
		}
		res, err := compute.AnimatedThumbnails(ctx, env, req, params)
		if err != nil {
			return err
		}
		if res.Preview == nil {
			return errors.New(res.PreviewError)
		}
		return p.checkVideo(ctx, filepath.Join(out, res.Preview.File), 320, 180, 2)
	}
	return fmt.Errorf("unknown step %q", step)
}

// checkVideo verifies an output: it decodes fully, has the dimensions and about the
// duration expected.
func (p *prober) checkVideo(ctx context.Context, path string, width, height int, seconds float64) error {
	info, err := ProbeVideo(ctx, p.probe, path)
	if err != nil {
		return err
	}
	if info.Width != width || info.Height != height {
		return fmt.Errorf("output is %dx%d, want %dx%d", info.Width, info.Height, width, height)
	}
	if info.Duration < seconds-0.3 || info.Duration > seconds+0.3 {
		return fmt.Errorf("output lasts %.2fs, want %.2fs", info.Duration, seconds)
	}
	if _, err := run(ctx, p.bin, "-hide_banner", "-v", "error", "-nostdin", "-i", path, "-f", "null", "-"); err != nil {
		return fmt.Errorf("output does not decode: %w", err)
	}
	return nil
}

func (p *prober) checkImage(ctx context.Context, path string, width, height int) error {
	info, err := ProbeVideo(ctx, p.probe, path)
	if err != nil {
		return err
	}
	if info.Width != width || info.Height != height {
		return fmt.Errorf("image is %dx%d, want %dx%d", info.Width, info.Height, width, height)
	}
	return nil
}

// VideoProbe is what ProbeVideo reads.
type VideoProbe struct {
	Codec    string  `json:"codec"`
	PixFmt   string  `json:"pix_fmt"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	Duration float64 `json:"duration_sec"`
	Size     int64   `json:"size"`
}

// ProbeVideo reads the first video stream of path with ffprobe.
func ProbeVideo(ctx context.Context, ffprobe, path string) (VideoProbe, error) {
	res, err := run(ctx, ffprobe, "-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=codec_name,pix_fmt,width,height:format=duration,size", "-of", "default=noprint_wrappers=1", path)
	if err != nil {
		return VideoProbe{}, fmt.Errorf("ffprobe failed on %s: %w", filepath.Base(path), err)
	}
	var v VideoProbe
	for _, line := range strings.Split(string(res.Stdout), "\n") {
		k, val, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch k {
		case "codec_name":
			v.Codec = val
		case "pix_fmt":
			v.PixFmt = val
		case "width":
			v.Width, _ = strconv.Atoi(val)
		case "height":
			v.Height, _ = strconv.Atoi(val)
		case "duration":
			v.Duration, _ = strconv.ParseFloat(val, 64)
		case "size":
			v.Size, _ = strconv.ParseInt(val, 10, 64)
		}
	}
	return v, nil
}

// ParseVersion returns the version of "ffmpeg -version" ("N-124716-g054dffd133-20260531").
func ParseVersion(out string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(out), "\n")
	f := strings.Fields(line)
	if len(f) >= 3 && f[0] == "ffmpeg" && f[1] == "version" {
		return f[2]
	}
	return strings.TrimSpace(line)
}

// ParseHWAccels parses "ffmpeg -hwaccels".
func ParseHWAccels(out string) []string {
	var res []string
	started := false
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "Hardware acceleration methods") {
			started = true
			continue
		}
		if started && line != "" {
			res = append(res, line)
		}
	}
	return res
}

// ParseCodecList parses "ffmpeg -encoders" (or -decoders): the names after the "------"
// line (" V....D h264_nvenc  NVIDIA NVENC H.264 encoder").
func ParseCodecList(out string) []string {
	var res []string
	started := false
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 0 {
			continue
		}
		if !started {
			started = strings.HasPrefix(f[0], "------")
			continue
		}
		if len(f) >= 2 && f[0] != "=" {
			res = append(res, f[1])
		}
	}
	return res
}

// ParseFilters parses "ffmpeg -filters" (" .. scale_cuda  V->V  GPU accelerated video
// resizer"): the second field of every line with a "->" type.
func ParseFilters(out string) []string {
	var res []string
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) >= 3 && strings.Contains(f[2], "->") {
			res = append(res, f[1])
		}
	}
	return res
}

// gpuNames returns GPU names, best effort: it gives up after 5 s even when the tool cannot
// be killed (nvidia-smi blocks in the driver while a stalled CUDA process holds it).
func gpuNames(ctx context.Context) []string {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	done := make(chan []string, 1)
	go func() { done <- queryGPUNames(ctx) }()
	select {
	case names := <-done:
		return names
	case <-ctx.Done():
		return nil
	}
}

func queryGPUNames(ctx context.Context) []string {
	var out []string
	if path, err := exec.LookPath("nvidia-smi"); err == nil {
		if res, err := run(ctx, path, "--query-gpu=name,driver_version", "--format=csv,noheader"); err == nil {
			for _, line := range strings.Split(strings.TrimSpace(string(res.Stdout)), "\n") {
				name, driver, _ := strings.Cut(line, ",")
				if name = strings.TrimSpace(name); name != "" {
					out = append(out, fmt.Sprintf("%s (driver %s)", name, strings.TrimSpace(driver)))
				}
			}
		}
	}
	if runtime.GOOS == "darwin" {
		if res, err := run(ctx, "sysctl", "-n", "machdep.cpu.brand_string"); err == nil {
			if name := strings.TrimSpace(string(res.Stdout)); name != "" {
				out = append(out, name)
			}
		}
	}
	sort.Strings(out)
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
