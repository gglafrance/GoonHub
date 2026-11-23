package compute

import (
	"fmt"
	"strconv"
	"time"
)

// Step is one ffmpeg step of a phase. Hardware acceleration is chosen per step: a GPU decode
// helps the long decodes (visual hash, scene preview), rarely the single-frame seeks.
type Step string

const (
	// StepSpriteFrame extracts one scaled WebP frame per sprite tile (decode + scale).
	StepSpriteFrame Step = "sprite_frame"
	// StepMarkerClip encodes a marker animated thumbnail (decode + scale + H.264 encode).
	StepMarkerClip Step = "marker_clip"
	// StepScenePreview encodes the scene preview video (decode + scale + H.264 encode).
	StepScenePreview Step = "scene_preview"
	// StepVisualHash decodes the whole video to 9x8 grayscale frames (decode + scale).
	StepVisualHash Step = "visual_hash"
	// StepAudioFingerprint runs fpcalc (no acceleration, listed for StepError).
	StepAudioFingerprint Step = "audio_fingerprint"
)

// Accel is the hardware acceleration choice of a worker. The zero value is software
// everywhere: the exact ffmpeg commands the server runs today.
//
// Steps maps a step to its acceleration; a step without entry runs in software. When an
// accelerated run fails (and the context is still alive), the step is retried in software
// unless NoSoftwareFallback is set.
type Accel struct {
	// Name is the profile label reported to the server ("none", "nvidia", "amd", "intel",
	// "videotoolbox", "vaapi"...). "" means "none".
	Name               string
	Steps              map[Step]StepAccel
	NoSoftwareFallback bool
}

// For returns the acceleration of a step (the zero StepAccel = software).
func (a Accel) For(step Step) StepAccel {
	if a.Steps == nil {
		return StepAccel{}
	}
	return a.Steps[step]
}

// Label returns Name, or "none".
func (a Accel) Label() string {
	if a.Name == "" {
		return "none"
	}
	return a.Name
}

// StepAccel is the acceleration of one step: how frames are decoded, scaled and encoded.
type StepAccel struct {
	Decode  DecodeAccel
	Scale   ScaleAccel
	Encoder EncoderAccel
	// MaxParallel caps the concurrent ffmpeg runs of the step while it is accelerated
	// (sprite frames run in parallel, every run opens its own GPU context and decoder, and
	// too many of them exhaust the video memory). 0 = no cap.
	MaxParallel int
	// Timeout + TimeoutPerSecond x the source seconds the run covers bounds an accelerated
	// run: a run still going after that is killed and the step runs again in software (a
	// stalled GPU run would otherwise hold the job while its lease keeps being renewed).
	// 0 = no bound.
	Timeout          time.Duration
	TimeoutPerSecond time.Duration
}

// deadline returns the time bound of an accelerated run over seconds of source (0 = none).
func (s StepAccel) deadline(seconds int) time.Duration {
	if s.Timeout <= 0 && s.TimeoutPerSecond <= 0 {
		return 0
	}
	if seconds < 0 {
		seconds = 0
	}
	return s.Timeout + time.Duration(seconds)*s.TimeoutPerSecond
}

// DecodeAccel selects a hardware decoder.
type DecodeAccel struct {
	// HWAccel is the -hwaccel value ("cuda", "qsv", "d3d11va", "videotoolbox", "vaapi"...);
	// "" decodes in software.
	HWAccel string
	// OutputFormat is the -hwaccel_output_format value ("cuda", "qsv", "vaapi"...): frames
	// stay in GPU memory, required for a GPU scale filter. "" downloads decoded frames.
	OutputFormat string
	// Device is the -hwaccel_device value ("0", "/dev/dri/renderD128", or the name of a
	// device created by GlobalArgs).
	Device string
	// ExtraInputArgs are put before every -i (e.g. "-extra_hw_frames", "8").
	ExtraInputArgs []string
	// GlobalArgs are put once, before the first input: options that must not repeat per
	// input, such as "-init_hw_device", "vaapi=gh:/dev/dri/renderD128" (every input then
	// shares that device through Device "gh", which a multi-input filter graph needs).
	GlobalArgs []string
}

// ScaleAccel selects a GPU scale filter.
type ScaleAccel struct {
	// Filter is the GPU scale filter ("scale_cuda", "scale_npp", "scale_qsv", "scale_vaapi",
	// "scale_vt"); "" scales on the CPU with "scale".
	Filter string
	// Options are extra options of the GPU filter, after the dimensions
	// ("interp_algo=bilinear:format=nv12" gives "scale_cuda=480:-2:interp_algo=bilinear:format=nv12").
	Options string
	// DownloadFormat brings frames back to the CPU (",hwdownload,format=<DownloadFormat>"),
	// needed when a CPU filter or a software encoder follows. "" keeps them on the GPU
	// (hardware encoder next).
	//
	// With Filter "" it means the decoder keeps its frames in GPU memory
	// (DecodeAccel.OutputFormat) and they are downloaded before the CPU scale:
	// "[<Convert>,]hwdownload,format=<DownloadFormat>,scale=...". Filters placed before the
	// scale (fps=1/2 of the visual hash) then run on the GPU frames, so only the frames kept
	// cross the bus, and the CPU scale keeps the software results.
	DownloadFormat string
	// Convert is a GPU filter run before that download when the scaling is on the CPU (Filter
	// ""), typically a format conversion such as "scale_cuda=format=nv12" so that 10-bit
	// frames can be downloaded as nv12.
	Convert string
}

// EncoderAccel selects the H.264 encoder and maps the x264 CRF to its quality scale.
type EncoderAccel struct {
	// H264 is the encoder name ("h264_nvenc", "h264_amf", "h264_qsv", "h264_videotoolbox",
	// "h264_vaapi"); "" encodes with libx264.
	H264 string
	// QualityFlag is the quality option of the encoder ("-cq", "-qp_i", "-global_quality",
	// "-q:v", "-qp"...). Its value is CRF*QualityScale+QualityOffset (QualityScale 0 = 1),
	// clamped to [QualityMin, QualityMax] when QualityMax > 0.
	QualityFlag   string
	QualityScale  float64
	QualityOffset int
	QualityMin    int
	QualityMax    int
	// QualityArgs, when set, replaces the mapping above.
	QualityArgs func(crf int) []string
	// ExtraArgs are put after the quality options (e.g. "-preset", "p4", "-rc", "vbr").
	ExtraArgs []string
	// PixFmt is the pixel format given to the encoder when frames are in CPU memory ("" =
	// yuv420p, what libx264 gets). h264_qsv needs "nv12".
	PixFmt string
	// Upload uploads CPU frames to the GPU before an encoder that only takes GPU frames
	// (h264_vaapi): "format=<PixFmt>,hwupload" ends the filter chain instead of -pix_fmt.
	// The device comes from DecodeAccel.GlobalArgs ("-filter_hw_device").
	Upload bool
}

// IsSoftware reports whether the step runs fully in software.
func (s StepAccel) IsSoftware() bool {
	return s.Decode.HWAccel == "" && len(s.Decode.ExtraInputArgs) == 0 && len(s.Decode.GlobalArgs) == 0 &&
		s.Scale.Filter == "" && s.Scale.DownloadFormat == "" && s.Encoder.H264 == ""
}

// globalArgs are the options put once before the first input.
func (s StepAccel) globalArgs() []string {
	return s.Decode.GlobalArgs
}

// inputArgs are the options put before each -i.
func (s StepAccel) inputArgs() []string {
	var args []string
	if s.Decode.HWAccel != "" {
		args = append(args, "-hwaccel", s.Decode.HWAccel)
		if s.Decode.OutputFormat != "" {
			args = append(args, "-hwaccel_output_format", s.Decode.OutputFormat)
		}
		if s.Decode.Device != "" {
			args = append(args, "-hwaccel_device", s.Decode.Device)
		}
	}
	return append(args, s.Decode.ExtraInputArgs...)
}

// scale returns the scale filter for dims ("w:h"): "scale=<dims>[:<cpuOpts>]" on the CPU
// (after "[<convert>,]hwdownload,format=<fmt>," when the decoded frames are in GPU memory),
// "<gpu filter>=<dims>[:<options>][,hwdownload,format=<fmt>]" on the GPU.
func (s StepAccel) scale(dims, cpuOpts string) string {
	if s.Scale.Filter == "" {
		cpu := "scale=" + dims
		if cpuOpts != "" {
			cpu += ":" + cpuOpts
		}
		if s.Scale.DownloadFormat == "" {
			return cpu
		}
		f := "hwdownload,format=" + s.Scale.DownloadFormat + "," + cpu
		if s.Scale.Convert != "" {
			f = s.Scale.Convert + "," + f
		}
		return f
	}
	f := s.Scale.Filter + "=" + dims
	if s.Scale.Options != "" {
		f += ":" + s.Scale.Options
	}
	if s.Scale.DownloadFormat != "" {
		f += ",hwdownload,format=" + s.Scale.DownloadFormat
	}
	return f
}

// framesOnCPU reports whether the frames reach the encoder in CPU memory (so CPU pixel
// format options apply).
func (s StepAccel) framesOnCPU() bool {
	return s.Scale.Filter == "" || s.Scale.DownloadFormat != ""
}

// cpuTail is what ends a filter chain whose frames are in CPU memory: the encoder's pixel
// format, then the upload for an encoder that takes GPU frames only.
func (s StepAccel) cpuTail() string {
	t := "format=" + s.pixFmt()
	if s.Encoder.Upload {
		t += ",hwupload"
	}
	return t
}

// pixFmt is the CPU pixel format of the frames given to the encoder.
func (s StepAccel) pixFmt() string {
	if s.Encoder.PixFmt == "" {
		return "yuv420p"
	}
	return s.Encoder.PixFmt
}

// h264Encoder returns the H.264 encoder name.
func (s StepAccel) h264Encoder() string {
	if s.Encoder.H264 == "" {
		return "libx264"
	}
	return s.Encoder.H264
}

// qualityArgs returns the rate control options for a CRF: "-preset veryfast -crf <crf>" for
// libx264 (today's settings), the mapped quality otherwise.
func (s StepAccel) qualityArgs(crf int) []string {
	e := s.Encoder
	if e.H264 == "" {
		return []string{"-preset", "veryfast", "-crf", strconv.Itoa(crf)}
	}
	var args []string
	switch {
	case e.QualityArgs != nil:
		args = e.QualityArgs(crf)
	case e.QualityFlag != "":
		scale := e.QualityScale
		if scale == 0 {
			scale = 1
		}
		q := int(float64(crf)*scale+0.5) + e.QualityOffset
		if e.QualityMax > 0 {
			if q < e.QualityMin {
				q = e.QualityMin
			}
			if q > e.QualityMax {
				q = e.QualityMax
			}
		}
		args = []string{e.QualityFlag, strconv.Itoa(q)}
	}
	return append(args, e.ExtraArgs...)
}

// QualityArgs returns the rate control options the step's encoder gets for an x264 CRF
// ("-preset veryfast -crf <crf>" in software).
func (s StepAccel) QualityArgs(crf int) []string {
	return s.qualityArgs(crf)
}

// String describes the step acceleration for logs.
func (s StepAccel) String() string {
	if s.IsSoftware() {
		return "software"
	}
	scale := orSW(s.Scale.Filter)
	if s.Scale.Filter == "" && s.Scale.DownloadFormat != "" {
		scale = "sw(gpu frames)"
	}
	return fmt.Sprintf("decode=%s scale=%s encode=%s", orSW(s.Decode.HWAccel), scale, orSW(s.Encoder.H264))
}

func orSW(v string) string {
	if v == "" {
		return "sw"
	}
	return v
}
