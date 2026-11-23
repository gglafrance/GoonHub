package hwaccel

import (
	"strconv"

	"goonhub/pkg/compute"
)

// Quality mapping: the server gives an x264 CRF (marker clips: processing.marker_preview_crf,
// default 32; scene previews: processing.scene_preview_crf, default 27). Each hardware
// encoder gets a value of its own scale chosen so that its output stays close to the
// libx264 -preset veryfast -crf <crf> file in quality (SSIM against the source scaled with
// the same scaler) and size.
//
// Measured on the reference machine (RTX 2080 Ti driver 621.01, AMD Radeon iGPU, ffmpeg
// N-124716): CRF sweep 23 / 27 / 32 on 10 s 480 px clips cut from three filmed 20 s videos
// (H.264 1080p60, HEVC 1080p60, H.264 2160p30), then TestGPU_EncodeOutputs (marker clip
// CRF 32 and preview CRF 27, 480 px) on four filmed and five generated samples (H.264 /
// HEVC 8 and 10-bit, 1080p and 4K):
//
//   - h264_nvenc: -rc constqp -qp round(1.1*crf)+3 (28 / 33 / 38), -preset p6 -tune hq
//     -bf 3. Marker clips: size 0.67-1.34x of x264, SSIM -0.004..+0.021 on filmed content
//     (1.00-1.03x, -0.004 on generated content; 0.96x, -0.012 on testsrc2 text); previews
//     0.79-1.21x. The VBR -cq mode needs cq = crf+11 for the same SSIM and makes files
//     1.3-1.9x larger, hence constqp (its B-frame QP offset plays the role of x264's ratios).
//   - h264_amf: -rc cqp -qp_i/-qp_p/-qp_b crf+3 (26 / 30 / 35), -quality quality. Marker
//     clips: SSIM -0.009..+0.018 on filmed content, but 1.3-3x the size (the iGPU encoder
//     is much less efficient; -0.042 SSIM on sharp testsrc2 text). Hence AMD is the last
//     method tried.
//
// Not measurable on that machine (no Intel GPU, Linux VA-API or Mac): values from the
// encoders' documentation and the measured CQP behaviour above; Detect proves that the
// options are accepted, Bench reports the sizes.
//
//   - h264_qsv: -global_quality crf+2 (ICQ, Intel's CRF-like scale), -preset medium, nv12.
//   - h264_vaapi: -rc_mode CQP -qp crf+4 (between the NVENC and AMF CQP offsets); CPU
//     frames are uploaded (format=nv12,hwupload).
//   - h264_videotoolbox: -q:v clamp(110-2*crf, 1, 100) (64 / 56 / 46; constant quality,
//     Apple Silicon only, higher is better), -allow_sw 0 (never Apple's software encoder:
//     the step falls back to libx264 instead).

// Encoder returns the H.264 encoder of method m with its CRF mapping (the zero value =
// libx264 for methods without encoder).
func Encoder(m Method) compute.EncoderAccel {
	switch m {
	case MethodNVIDIA:
		return compute.EncoderAccel{
			H264: "h264_nvenc", QualityFlag: "-qp", QualityScale: 1.1, QualityOffset: 3, QualityMin: 1, QualityMax: 51,
			ExtraArgs: []string{"-rc", "constqp", "-preset", "p6", "-tune", "hq", "-bf", "3"},
		}
	case MethodAMD:
		return compute.EncoderAccel{
			H264: "h264_amf",
			QualityArgs: func(crf int) []string {
				q := strconv.Itoa(clamp(crf+3, 0, 51))
				return []string{"-rc", "cqp", "-qp_i", q, "-qp_p", q, "-qp_b", q}
			},
			ExtraArgs: []string{"-quality", "quality"},
		}
	case MethodIntel:
		return compute.EncoderAccel{
			H264: "h264_qsv", QualityFlag: "-global_quality", QualityOffset: 2, QualityMin: 1, QualityMax: 51,
			ExtraArgs: []string{"-preset", "medium"}, PixFmt: "nv12",
		}
	case MethodVAAPI:
		return compute.EncoderAccel{
			H264: "h264_vaapi", QualityFlag: "-qp", QualityOffset: 4, QualityMin: 1, QualityMax: 51,
			ExtraArgs: []string{"-rc_mode", "CQP"}, PixFmt: "nv12", Upload: true,
		}
	case MethodVideoToolbox:
		return compute.EncoderAccel{
			H264: "h264_videotoolbox",
			QualityArgs: func(crf int) []string {
				return []string{"-q:v", strconv.Itoa(clamp(110-2*crf, 1, 100))}
			},
			ExtraArgs: []string{"-allow_sw", "0"},
		}
	}
	return compute.EncoderAccel{}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
