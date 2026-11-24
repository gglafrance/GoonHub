package hwaccel

import (
	"runtime"
	"time"

	"goonhub/pkg/compute"
)

// Method is a hardware acceleration family.
type Method string

const (
	// MethodNone is software everywhere (the server's commands).
	MethodNone Method = "none"
	// MethodNVIDIA: NVDEC decode (-hwaccel cuda), scale_npp / scale_cuda, h264_nvenc.
	// Windows and Linux.
	MethodNVIDIA Method = "nvidia"
	// MethodIntel: Quick Sync (-hwaccel qsv, scale_qsv, h264_qsv). Windows and Linux.
	MethodIntel Method = "intel"
	// MethodAMD: AMF encoder (h264_amf), software decode (an AMF encoder cannot use the
	// frames of a D3D11 decoder running on another adapter, measured on a PC with an NVIDIA
	// card and an AMD iGPU). Windows (Linux with the AMF runtime).
	MethodAMD Method = "amd"
	// MethodVAAPI: VA-API (-hwaccel vaapi on a DRM render node, scale_vaapi, h264_vaapi).
	// Linux.
	MethodVAAPI Method = "vaapi"
	// MethodVideoToolbox: Apple VideoToolbox (-hwaccel videotoolbox, scale_vt,
	// h264_videotoolbox). macOS.
	MethodVideoToolbox Method = "videotoolbox"
	// MethodD3D11VA: Direct3D 11 video decode on the default adapter, any GPU vendor. Decode
	// only (the encode steps keep libx264). Windows.
	MethodD3D11VA Method = "d3d11va"
)

// DefaultVAAPIDevice is the DRM render node used for VA-API.
const DefaultVAAPIDevice = "/dev/dri/renderD128"

// AllMethods lists the hardware methods in preference order (the first proven method of a
// step wins). AMD AMF comes last: it only encodes (software decode), and on the measured
// Radeon iGPU it needs 1.2-3x the size of x264 for the same SSIM, while a D3D11VA decode +
// libx264 offloads the expensive part (decode) and gives the software file bit for bit.
var AllMethods = []Method{MethodNVIDIA, MethodVideoToolbox, MethodIntel, MethodVAAPI, MethodD3D11VA, MethodAMD}

// Valid reports whether m is a known method (MethodNone included).
func (m Method) Valid() bool {
	if m == MethodNone {
		return true
	}
	for _, k := range AllMethods {
		if k == m {
			return true
		}
	}
	return false
}

// methodInfo is what ffmpeg must offer for a method.
type methodInfo struct {
	hwaccel      string   // -hwaccel value ("" = no hardware decode)
	outputFormat string   // -hwaccel_output_format of the strict decode probe
	encoder      string   // H.264 encoder ("" = libx264)
	scaleFilters []string // GPU scale filters, preferred first
	goos         []string // operating systems where the method exists
}

var methods = map[Method]methodInfo{
	MethodNVIDIA:       {hwaccel: "cuda", outputFormat: "cuda", encoder: "h264_nvenc", scaleFilters: []string{"scale_npp", "scale_cuda"}, goos: []string{"windows", "linux"}},
	MethodIntel:        {hwaccel: "qsv", outputFormat: "qsv", encoder: "h264_qsv", scaleFilters: []string{"scale_qsv"}, goos: []string{"windows", "linux"}},
	MethodAMD:          {encoder: "h264_amf", goos: []string{"windows", "linux"}},
	MethodVAAPI:        {hwaccel: "vaapi", outputFormat: "vaapi", encoder: "h264_vaapi", scaleFilters: []string{"scale_vaapi"}, goos: []string{"linux"}},
	MethodVideoToolbox: {hwaccel: "videotoolbox", outputFormat: "videotoolbox_vld", encoder: "h264_videotoolbox", scaleFilters: []string{"scale_vt"}, goos: []string{"darwin"}},
	MethodD3D11VA:      {hwaccel: "d3d11va", outputFormat: "d3d11", goos: []string{"windows"}},
}

func (m Method) info() methodInfo { return methods[m] }

// onOS reports whether the method exists on goos.
func (m Method) onOS(goos string) bool {
	for _, g := range m.info().goos {
		if g == goos {
			return true
		}
	}
	return false
}

// HWAccel returns the -hwaccel value of the method ("" when it has no hardware decode).
func (m Method) HWAccel() string { return m.info().hwaccel }

// Encoder returns the H.264 encoder of the method ("" when it keeps libx264).
func (m Method) Encoder() string { return m.info().encoder }

// Variant is one way of running a step with a method.
type Variant string

const (
	// VariantDecode: hardware decode only; scaling and encoding stay in software (same
	// scaler and encoder as the server). Sprite frames, visual hash, and the encode steps of
	// a decode-only method (D3D11VA + libx264).
	VariantDecode Variant = "decode"
	// VariantGPUFrames (visual hash): decoded frames stay in GPU memory, the fps filter drops
	// them there, only the kept frames are converted to nv12 and downloaded, then scaled to
	// 9x8 on the CPU. Same hashes as VariantDecode for 8-bit sources, ~1 bit for 10-bit.
	VariantGPUFrames Variant = "gpu_frames"
	// VariantQuality (encode steps): hardware decode, CPU scale (the software scaler, same
	// pixels), hardware encoder. The scene preview decodes in software (see encodePipeline).
	VariantQuality Variant = "quality"
	// VariantSpeed (encode steps): everything on the GPU: decode, GPU scale, encode.
	VariantSpeed Variant = "speed"
)

// PipelineOptions are the machine facts a pipeline depends on.
type PipelineOptions struct {
	// Filters are the filters ffmpeg has (Report.Filters); picks scale_npp over scale_cuda.
	Filters []string
	// VAAPIDevice is the DRM render node ("" = DefaultVAAPIDevice).
	VAAPIDevice string
	// GOOS is the target OS ("" = runtime.GOOS); Intel QSV device setup differs on Linux.
	GOOS string
}

func (o PipelineOptions) vaapiDevice() string {
	if o.VAAPIDevice == "" {
		return DefaultVAAPIDevice
	}
	return o.VAAPIDevice
}

func (o PipelineOptions) goos() string {
	if o.GOOS == "" {
		return runtime.GOOS
	}
	return o.GOOS
}

func (o PipelineOptions) hasFilter(name string) bool {
	for _, f := range o.Filters {
		if f == name {
			return true
		}
	}
	return false
}

// SpriteMaxParallel caps the parallel sprite frame runs when they decode on the GPU (each
// run opens its own GPU context and decoder).
const SpriteMaxParallel = 4

// sharedDevice is the name of the device created once with -init_hw_device for the
// pipelines whose inputs must share one device (multi-input scene preview).
const sharedDevice = "gh"

// Run bounds of the accelerated runs (StepAccel.Timeout + TimeoutPerSecond x source
// seconds): far above a normal GPU run, so that only a stalled one is cut (it is then run
// again in software). Normal runs measured on the RTX 2080 Ti: visual hash of 120 s of 4K
// in 3.5-8.5 s, a 10 s marker clip in under 2 s.
var timeouts = map[compute.Step][2]time.Duration{
	compute.StepVisualHash:   {2 * time.Minute, 500 * time.Millisecond}, // 30 min video: 17 min
	compute.StepSpriteFrame:  {30 * time.Second, 0},
	compute.StepMarkerClip:   {time.Minute, 3 * time.Second},
	compute.StepScenePreview: {2 * time.Minute, 100 * time.Millisecond}, // seconds = whole video
}

// Pipeline returns the acceleration of step with method m and variant v, and whether the
// method offers that combination. It does not check the machine (see Detect / Choose).
func Pipeline(m Method, step compute.Step, v Variant, o PipelineOptions) (compute.StepAccel, bool) {
	acc, ok := buildPipeline(m, step, v, o)
	if ok {
		t := timeouts[step]
		acc.Timeout, acc.TimeoutPerSecond = t[0], t[1]
	}
	return acc, ok
}

func buildPipeline(m Method, step compute.Step, v Variant, o PipelineOptions) (compute.StepAccel, bool) {
	switch step {
	case compute.StepSpriteFrame:
		if v != VariantDecode {
			return compute.StepAccel{}, false
		}
		d, ok := decodeOnly(m, o)
		return compute.StepAccel{Decode: d, MaxParallel: SpriteMaxParallel}, ok
	case compute.StepVisualHash:
		switch v {
		case VariantDecode:
			d, ok := decodeOnly(m, o)
			return compute.StepAccel{Decode: d}, ok
		case VariantGPUFrames:
			return gpuFramesHash(m, o)
		}
		return compute.StepAccel{}, false
	case compute.StepMarkerClip, compute.StepScenePreview:
		return encodePipeline(m, step, v, o)
	}
	return compute.StepAccel{}, false
}

// decodeOnly is the hardware decoder with frames downloaded to the CPU by ffmpeg (a stream
// the decoder cannot handle silently falls back to the software decoder).
func decodeOnly(m Method, o PipelineOptions) (compute.DecodeAccel, bool) {
	switch m {
	case MethodNVIDIA, MethodIntel, MethodVideoToolbox, MethodD3D11VA:
		return compute.DecodeAccel{HWAccel: m.HWAccel()}, true
	case MethodVAAPI:
		return compute.DecodeAccel{HWAccel: "vaapi", Device: o.vaapiDevice()}, true
	}
	return compute.DecodeAccel{}, false
}

// gpuFramesHash keeps the decoded frames on the GPU through fps=1/2, converts them to nv12
// there (10-bit sources), downloads them and scales to 9x8 on the CPU. The GPU scalers are
// never used for the 9x8 thumbnail itself: measured on NVIDIA, scale_cuda straight to 9x8
// moves the hashes by 12-25 bits (it samples a few pixels instead of averaging the area).
func gpuFramesHash(m Method, o PipelineOptions) (compute.StepAccel, bool) {
	var convert string
	d := compute.DecodeAccel{HWAccel: m.HWAccel(), OutputFormat: m.info().outputFormat}
	switch m {
	case MethodNVIDIA:
		convert = "scale_cuda=format=nv12"
	case MethodIntel:
		convert = "scale_qsv=format=nv12"
	case MethodVAAPI:
		convert = "scale_vaapi=format=nv12"
		d.Device = o.vaapiDevice()
	default:
		return compute.StepAccel{}, false
	}
	return compute.StepAccel{Decode: d, Scale: compute.ScaleAccel{Convert: convert, DownloadFormat: "nv12"}}, true
}

// encodePipeline is a marker clip (one input) or scene preview pipeline.
//
// The scene preview opens one input per segment (12 by default, up to 24), each with its
// own decoder running at once. Measured on the RTX 2080 Ti, 12-input preview of a 20 s 4K
// HEVC 10-bit source: software decode 10.3-10.6 s (with NVENC 10.4-10.7 s), -hwaccel cuda
// stalls (4 runs out of 4 cut after 60 s at 7.2 of 11 GB of video memory; once, the
// stalled process also blocked every other CUDA / nvidia-smi call until it was killed),
// -hwaccel d3d11va 57-60 s. So the preview never decodes on the GPU: software decode (short
// seeks of 1 s segments), software scaler, GPU encoder.
func encodePipeline(m Method, step compute.Step, v Variant, o PipelineOptions) (compute.StepAccel, bool) {
	enc := Encoder(m)
	if step == compute.StepScenePreview {
		if v != VariantQuality || enc.H264 == "" {
			return compute.StepAccel{}, false
		}
		acc := compute.StepAccel{Encoder: enc}
		if m == MethodVAAPI {
			acc.Decode.GlobalArgs = vaapiInitArgs(o)
		}
		return acc, true
	}
	switch m {
	case MethodNVIDIA:
		switch v {
		case VariantQuality:
			return compute.StepAccel{Decode: compute.DecodeAccel{HWAccel: "cuda"}, Encoder: enc}, true
		case VariantSpeed:
			scale := compute.ScaleAccel{Filter: "scale_cuda", Options: "interp_algo=bilinear:format=nv12"}
			if o.hasFilter("scale_npp") {
				// supersampling: a real area downscale, closer to swscale than bilinear taps
				scale = compute.ScaleAccel{Filter: "scale_npp", Options: "interp_algo=super:format=nv12"}
			}
			return compute.StepAccel{Decode: compute.DecodeAccel{HWAccel: "cuda", OutputFormat: "cuda"}, Scale: scale, Encoder: enc}, true
		}
	case MethodIntel:
		switch v {
		case VariantQuality:
			return compute.StepAccel{Decode: compute.DecodeAccel{HWAccel: "qsv"}, Encoder: enc}, true
		case VariantSpeed:
			return compute.StepAccel{
				Decode:  compute.DecodeAccel{HWAccel: "qsv", OutputFormat: "qsv", Device: sharedDevice, GlobalArgs: qsvInitArgs(o)},
				Scale:   compute.ScaleAccel{Filter: "scale_qsv", Options: "format=nv12"},
				Encoder: enc,
			}, true
		}
	case MethodAMD:
		if v == VariantQuality {
			return compute.StepAccel{Encoder: enc}, true
		}
	case MethodVAAPI:
		switch v {
		case VariantQuality:
			// decoded frames come back to the CPU, software scaler, uploaded for the encoder
			return compute.StepAccel{
				Decode:  compute.DecodeAccel{HWAccel: "vaapi", Device: sharedDevice, GlobalArgs: vaapiInitArgs(o)},
				Encoder: enc,
			}, true
		case VariantSpeed:
			return compute.StepAccel{
				Decode:  compute.DecodeAccel{HWAccel: "vaapi", OutputFormat: "vaapi", Device: sharedDevice, GlobalArgs: vaapiInitArgs(o)},
				Scale:   compute.ScaleAccel{Filter: "scale_vaapi", Options: "format=nv12"},
				Encoder: enc,
			}, true
		}
	case MethodVideoToolbox:
		switch v {
		case VariantQuality:
			return compute.StepAccel{Decode: compute.DecodeAccel{HWAccel: "videotoolbox"}, Encoder: enc}, true
		case VariantSpeed:
			return compute.StepAccel{
				Decode:  compute.DecodeAccel{HWAccel: "videotoolbox", OutputFormat: "videotoolbox_vld"},
				Scale:   compute.ScaleAccel{Filter: "scale_vt"},
				Encoder: enc,
			}, true
		}
	case MethodD3D11VA:
		if v == VariantDecode {
			return compute.StepAccel{Decode: compute.DecodeAccel{HWAccel: "d3d11va"}}, true
		}
	}
	return compute.StepAccel{}, false
}

// vaapiInitArgs create the VA-API device once and make it the device of hwupload.
func vaapiInitArgs(o PipelineOptions) []string {
	return []string{"-init_hw_device", "vaapi=" + sharedDevice + ":" + o.vaapiDevice(), "-filter_hw_device", sharedDevice}
}

// qsvInitArgs create one QSV device shared by every input.
func qsvInitArgs(o PipelineOptions) []string {
	if o.goos() == "linux" {
		return []string{"-init_hw_device", "vaapi=va:" + o.vaapiDevice(), "-init_hw_device", "qsv=" + sharedDevice + "@va"}
	}
	return []string{"-init_hw_device", "qsv=" + sharedDevice}
}

// candidates lists, per step, the variants of a method in the order Detect proves them.
func candidates(m Method, step compute.Step) []Variant {
	var out []Variant
	for _, v := range []Variant{VariantGPUFrames, VariantDecode, VariantQuality, VariantSpeed} {
		if _, ok := Pipeline(m, step, v, PipelineOptions{}); ok {
			out = append(out, v)
		}
	}
	return out
}
