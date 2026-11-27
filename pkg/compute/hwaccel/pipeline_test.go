package hwaccel

import (
	"reflect"
	"strings"
	"testing"

	"goonhub/pkg/compute"
)

// The VideoToolbox, VA-API and Quick Sync pipelines cannot run on the development machine:
// these tests pin their ffmpeg arguments to the documented forms. Detect proves them on the
// real machines.

func pipeline(t *testing.T, m Method, step compute.Step, v Variant, o PipelineOptions) compute.StepAccel {
	t.Helper()
	acc, ok := Pipeline(m, step, v, o)
	if !ok {
		t.Fatalf("%s has no %s/%s pipeline", m, step, v)
	}
	return acc
}

// argsOf strips the platform-dependent default arguments (-nostdin on Linux).
func argsOf(args []string) string {
	s := strings.Join(args, " ")
	return strings.TrimPrefix(s, "-nostdin ")
}

func TestPipelines_MarkerClipArgs(t *testing.T) {
	linux := PipelineOptions{GOOS: "linux"}
	tests := []struct {
		name string
		m    Method
		v    Variant
		o    PipelineOptions
		want string
	}{
		{"nvidia quality", MethodNVIDIA, VariantQuality, PipelineOptions{},
			"-hwaccel cuda -ss 5 -i in.mp4 -t 10 -c:v h264_nvenc -vf scale=480:-2:flags=bilinear -pix_fmt yuv420p -qp 38 -rc constqp -preset p6 -tune hq -bf 3 -movflags +faststart -map_metadata -1 -threads 2 -an -y out.mp4"},
		{"nvidia speed scale_cuda", MethodNVIDIA, VariantSpeed, PipelineOptions{Filters: []string{"scale_cuda"}},
			"-hwaccel cuda -hwaccel_output_format cuda -ss 5 -i in.mp4 -t 10 -c:v h264_nvenc -vf scale_cuda=480:-2:interp_algo=bilinear:format=nv12 -qp 38 -rc constqp -preset p6 -tune hq -bf 3 -movflags +faststart -map_metadata -1 -threads 2 -an -y out.mp4"},
		{"nvidia speed scale_npp", MethodNVIDIA, VariantSpeed, PipelineOptions{Filters: []string{"scale_cuda", "scale_npp"}},
			"-hwaccel cuda -hwaccel_output_format cuda -ss 5 -i in.mp4 -t 10 -c:v h264_nvenc -vf scale_npp=480:-2:interp_algo=super:format=nv12 -qp 38 -rc constqp -preset p6 -tune hq -bf 3 -movflags +faststart -map_metadata -1 -threads 2 -an -y out.mp4"},
		{"amd", MethodAMD, VariantQuality, PipelineOptions{},
			"-ss 5 -i in.mp4 -t 10 -c:v h264_amf -vf scale=480:-2:flags=bilinear -pix_fmt yuv420p -rc cqp -qp_i 35 -qp_p 35 -qp_b 35 -quality quality -movflags +faststart -map_metadata -1 -threads 2 -an -y out.mp4"},
		{"intel quality", MethodIntel, VariantQuality, PipelineOptions{},
			"-hwaccel qsv -ss 5 -i in.mp4 -t 10 -c:v h264_qsv -vf scale=480:-2:flags=bilinear -pix_fmt nv12 -global_quality 34 -preset medium -movflags +faststart -map_metadata -1 -threads 2 -an -y out.mp4"},
		{"intel speed windows", MethodIntel, VariantSpeed, PipelineOptions{GOOS: "windows"},
			"-init_hw_device qsv=gh -hwaccel qsv -hwaccel_output_format qsv -hwaccel_device gh -ss 5 -i in.mp4 -t 10 -c:v h264_qsv -vf scale_qsv=480:-2:format=nv12 -global_quality 34 -preset medium -movflags +faststart -map_metadata -1 -threads 2 -an -y out.mp4"},
		{"intel speed linux", MethodIntel, VariantSpeed, linux,
			"-init_hw_device vaapi=va:/dev/dri/renderD128 -init_hw_device qsv=gh@va -hwaccel qsv -hwaccel_output_format qsv -hwaccel_device gh -ss 5 -i in.mp4 -t 10 -c:v h264_qsv -vf scale_qsv=480:-2:format=nv12 -global_quality 34 -preset medium -movflags +faststart -map_metadata -1 -threads 2 -an -y out.mp4"},
		{"vaapi quality", MethodVAAPI, VariantQuality, linux,
			"-init_hw_device vaapi=gh:/dev/dri/renderD128 -filter_hw_device gh -hwaccel vaapi -hwaccel_device gh -ss 5 -i in.mp4 -t 10 -c:v h264_vaapi -vf scale=480:-2:flags=bilinear,format=nv12,hwupload -qp 36 -rc_mode CQP -movflags +faststart -map_metadata -1 -threads 2 -an -y out.mp4"},
		{"vaapi speed other device", MethodVAAPI, VariantSpeed, PipelineOptions{GOOS: "linux", VAAPIDevice: "/dev/dri/renderD129"},
			"-init_hw_device vaapi=gh:/dev/dri/renderD129 -filter_hw_device gh -hwaccel vaapi -hwaccel_output_format vaapi -hwaccel_device gh -ss 5 -i in.mp4 -t 10 -c:v h264_vaapi -vf scale_vaapi=480:-2:format=nv12 -qp 36 -rc_mode CQP -movflags +faststart -map_metadata -1 -threads 2 -an -y out.mp4"},
		{"videotoolbox quality", MethodVideoToolbox, VariantQuality, PipelineOptions{},
			"-hwaccel videotoolbox -ss 5 -i in.mp4 -t 10 -c:v h264_videotoolbox -vf scale=480:-2:flags=bilinear -pix_fmt yuv420p -q:v 46 -allow_sw 0 -movflags +faststart -map_metadata -1 -threads 2 -an -y out.mp4"},
		{"videotoolbox speed", MethodVideoToolbox, VariantSpeed, PipelineOptions{},
			"-hwaccel videotoolbox -hwaccel_output_format videotoolbox_vld -ss 5 -i in.mp4 -t 10 -c:v h264_videotoolbox -vf scale_vt=480:-2 -q:v 46 -allow_sw 0 -movflags +faststart -map_metadata -1 -threads 2 -an -y out.mp4"},
		{"d3d11va decode + libx264", MethodD3D11VA, VariantDecode, PipelineOptions{},
			"-hwaccel d3d11va -ss 5 -i in.mp4 -t 10 -c:v libx264 -vf scale=480:-2:flags=bilinear -pix_fmt yuv420p -preset veryfast -crf 32 -movflags +faststart -map_metadata -1 -threads 2 -an -y out.mp4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acc := pipeline(t, tt.m, compute.StepMarkerClip, tt.v, tt.o)
			if got := argsOf(compute.MarkerClipArgs(acc, "in.mp4", "out.mp4", 5, 10, 480, 32)); got != tt.want {
				t.Fatalf("args\n got: %s\nwant: %s", got, tt.want)
			}
		})
	}
}

func TestPipelines_ScenePreview(t *testing.T) {
	// one decoder per segment: the preview never decodes on the GPU, it only encodes there
	for _, m := range AllMethods {
		for _, v := range []Variant{VariantDecode, VariantGPUFrames, VariantQuality, VariantSpeed} {
			acc, ok := Pipeline(m, compute.StepScenePreview, v, PipelineOptions{GOOS: "linux"})
			if !ok {
				continue
			}
			if v != VariantQuality || acc.Decode.HWAccel != "" || acc.Scale.Filter != "" || acc.Encoder.H264 == "" {
				t.Fatalf("%s/%s preview pipeline %+v", m, v, acc)
			}
		}
	}

	acc := pipeline(t, MethodVAAPI, compute.StepScenePreview, VariantQuality, PipelineOptions{GOOS: "linux"})
	args := argsOf(compute.ScenePreviewArgs(acc, "in.mp4", "out.mp4", 600, 3, 1.0, 360, 27, true))
	if strings.Count(args, "-init_hw_device") != 1 || !strings.HasPrefix(args, "-init_hw_device vaapi=gh:/dev/dri/renderD128 -filter_hw_device gh -ss ") {
		t.Fatalf("the device must be created once, before the inputs: %s", args)
	}
	if strings.Contains(args, "-hwaccel") {
		t.Fatalf("software decode expected: %s", args)
	}
	if !strings.Contains(args, "setpts=PTS-STARTPTS,scale=360:-2:flags=bilinear,format=nv12,hwupload[v0]") {
		t.Fatalf("CPU frames must be uploaded for h264_vaapi: %s", args)
	}
	if !strings.Contains(args, "-c:v h264_vaapi -qp 31 -rc_mode CQP") || !strings.Contains(args, "-c:a aac") {
		t.Fatalf("encoder / audio args: %s", args)
	}
	short := argsOf(compute.ScenePreviewArgs(acc, "in.mp4", "out.mp4", 5, 12, 1.0, 360, 27, false))
	if !strings.Contains(short, "-vf scale=360:-2:flags=bilinear,format=nv12,hwupload -qp 31") || strings.Contains(short, "-pix_fmt") {
		t.Fatalf("short vaapi preview: %s", short)
	}

	qsv := pipeline(t, MethodIntel, compute.StepScenePreview, VariantQuality, PipelineOptions{})
	args = argsOf(compute.ScenePreviewArgs(qsv, "in.mp4", "out.mp4", 600, 2, 1.0, 360, 27, false))
	if !strings.Contains(args, "scale=360:-2:flags=bilinear,format=nv12[v0]") || !strings.Contains(args, "-c:v h264_qsv -global_quality 29") {
		t.Fatalf("qsv preview: %s", args)
	}
	short = argsOf(compute.ScenePreviewArgs(qsv, "in.mp4", "out.mp4", 5, 12, 1.0, 360, 27, false))
	if !strings.Contains(short, "-pix_fmt nv12 -global_quality 29") {
		t.Fatalf("short qsv preview: %s", short)
	}

	nv := pipeline(t, MethodNVIDIA, compute.StepScenePreview, VariantQuality, PipelineOptions{})
	args = argsOf(compute.ScenePreviewArgs(nv, "in.mp4", "out.mp4", 600, 12, 1.0, 360, 27, false))
	if strings.Contains(args, "-hwaccel") || !strings.Contains(args, "format=yuv420p[v11]") || !strings.Contains(args, "-c:v h264_nvenc -qp 33 -rc constqp") {
		t.Fatalf("nvidia preview: %s", args)
	}
}

func TestPipelines_VisualHashArgs(t *testing.T) {
	tests := []struct {
		name string
		m    Method
		v    Variant
		want string
	}{
		{"nvidia gpu frames", MethodNVIDIA, VariantGPUFrames,
			"-nostdin -hwaccel cuda -hwaccel_output_format cuda -skip_loop_filter all -skip_frame noref -i in.mp4 -an -sn -dn -vf fps=1/2,scale_cuda=format=nv12,hwdownload,format=nv12,scale=9:8,format=gray -f rawvideo pipe:1"},
		{"nvidia decode", MethodNVIDIA, VariantDecode,
			"-nostdin -hwaccel cuda -skip_loop_filter all -skip_frame noref -i in.mp4 -an -sn -dn -vf fps=1/2,scale=9:8,format=gray -f rawvideo pipe:1"},
		{"intel gpu frames", MethodIntel, VariantGPUFrames,
			"-nostdin -hwaccel qsv -hwaccel_output_format qsv -skip_loop_filter all -skip_frame noref -i in.mp4 -an -sn -dn -vf fps=1/2,scale_qsv=format=nv12,hwdownload,format=nv12,scale=9:8,format=gray -f rawvideo pipe:1"},
		{"vaapi gpu frames", MethodVAAPI, VariantGPUFrames,
			"-nostdin -hwaccel vaapi -hwaccel_output_format vaapi -hwaccel_device /dev/dri/renderD128 -skip_loop_filter all -skip_frame noref -i in.mp4 -an -sn -dn -vf fps=1/2,scale_vaapi=format=nv12,hwdownload,format=nv12,scale=9:8,format=gray -f rawvideo pipe:1"},
		{"vaapi decode", MethodVAAPI, VariantDecode,
			"-nostdin -hwaccel vaapi -hwaccel_device /dev/dri/renderD128 -skip_loop_filter all -skip_frame noref -i in.mp4 -an -sn -dn -vf fps=1/2,scale=9:8,format=gray -f rawvideo pipe:1"},
		{"videotoolbox decode", MethodVideoToolbox, VariantDecode,
			"-nostdin -hwaccel videotoolbox -skip_loop_filter all -skip_frame noref -i in.mp4 -an -sn -dn -vf fps=1/2,scale=9:8,format=gray -f rawvideo pipe:1"},
		{"d3d11va decode", MethodD3D11VA, VariantDecode,
			"-nostdin -hwaccel d3d11va -skip_loop_filter all -skip_frame noref -i in.mp4 -an -sn -dn -vf fps=1/2,scale=9:8,format=gray -f rawvideo pipe:1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acc := pipeline(t, tt.m, compute.StepVisualHash, tt.v, PipelineOptions{GOOS: "linux"})
			if got := strings.Join(compute.VisualHashArgs(acc, "in.mp4"), " "); got != tt.want {
				t.Fatalf("args\n got: %s\nwant: %s", got, tt.want)
			}
		})
	}
	// the 9x8 thumbnail is never made by a GPU scaler (it moves the hashes by 12-25 bits)
	for _, m := range AllMethods {
		for _, v := range []Variant{VariantDecode, VariantGPUFrames} {
			if acc, ok := Pipeline(m, compute.StepVisualHash, v, PipelineOptions{}); ok && acc.Scale.Filter != "" {
				t.Fatalf("%s/%s scales the hash frames on the GPU", m, v)
			}
		}
	}
	if _, ok := Pipeline(MethodVideoToolbox, compute.StepVisualHash, VariantGPUFrames, PipelineOptions{}); ok {
		t.Fatal("videotoolbox has no tested nv12 conversion: no gpu_frames variant")
	}
	if _, ok := Pipeline(MethodAMD, compute.StepVisualHash, VariantDecode, PipelineOptions{}); ok {
		t.Fatal("amd has no decode pipeline")
	}
}

func TestPipelines_SpriteFrameArgs(t *testing.T) {
	acc := pipeline(t, MethodNVIDIA, compute.StepSpriteFrame, VariantDecode, PipelineOptions{})
	want := "-hwaccel cuda -ss 30 -i in.mp4 -threads 1 -vframes 1 -vf scale=320:180 -q:v 75 -y f.webp"
	if got := argsOf(compute.SpriteFrameArgs(acc, "in.mp4", "f.webp", 30, 320, 180, 75)); got != want {
		t.Fatalf("args\n got: %s\nwant: %s", got, want)
	}
	if _, ok := Pipeline(MethodNVIDIA, compute.StepSpriteFrame, VariantSpeed, PipelineOptions{}); ok {
		t.Fatal("sprite frames are decode-only")
	}
	if acc.MaxParallel != SpriteMaxParallel {
		t.Fatalf("GPU sprite frames must cap their parallel runs: %d", acc.MaxParallel)
	}
}

// TestQualityMapping pins the documented CRF mapping of every encoder.
func TestQualityMapping(t *testing.T) {
	tests := []struct {
		m    Method
		crf  int
		want []string
	}{
		{MethodNVIDIA, 23, []string{"-qp", "28", "-rc", "constqp", "-preset", "p6", "-tune", "hq", "-bf", "3"}},
		{MethodNVIDIA, 27, []string{"-qp", "33", "-rc", "constqp", "-preset", "p6", "-tune", "hq", "-bf", "3"}},
		{MethodNVIDIA, 32, []string{"-qp", "38", "-rc", "constqp", "-preset", "p6", "-tune", "hq", "-bf", "3"}},
		{MethodNVIDIA, 50, []string{"-qp", "51", "-rc", "constqp", "-preset", "p6", "-tune", "hq", "-bf", "3"}},
		{MethodAMD, 27, []string{"-rc", "cqp", "-qp_i", "30", "-qp_p", "30", "-qp_b", "30", "-quality", "quality"}},
		{MethodIntel, 27, []string{"-global_quality", "29", "-preset", "medium"}},
		{MethodVAAPI, 27, []string{"-qp", "31", "-rc_mode", "CQP"}},
		{MethodVideoToolbox, 23, []string{"-q:v", "64", "-allow_sw", "0"}},
		{MethodVideoToolbox, 27, []string{"-q:v", "56", "-allow_sw", "0"}},
		{MethodVideoToolbox, 32, []string{"-q:v", "46", "-allow_sw", "0"}},
		{MethodD3D11VA, 27, []string{"-preset", "veryfast", "-crf", "27"}},
	}
	for _, tt := range tests {
		got := compute.StepAccel{Encoder: Encoder(tt.m)}.QualityArgs(tt.crf)
		if !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("%s crf %d: %v, want %v", tt.m, tt.crf, got, tt.want)
		}
	}
}

func TestCandidates(t *testing.T) {
	want := map[Method]map[compute.Step][]Variant{
		MethodNVIDIA: {
			compute.StepVisualHash: {VariantGPUFrames, VariantDecode}, compute.StepSpriteFrame: {VariantDecode},
			compute.StepMarkerClip: {VariantQuality, VariantSpeed}, compute.StepScenePreview: {VariantQuality},
		},
		MethodAMD: {
			compute.StepMarkerClip: {VariantQuality}, compute.StepScenePreview: {VariantQuality},
		},
		MethodD3D11VA: {
			compute.StepVisualHash: {VariantDecode}, compute.StepSpriteFrame: {VariantDecode},
			compute.StepMarkerClip: {VariantDecode},
		},
		MethodVideoToolbox: {
			compute.StepVisualHash: {VariantDecode}, compute.StepSpriteFrame: {VariantDecode},
			compute.StepMarkerClip: {VariantQuality, VariantSpeed}, compute.StepScenePreview: {VariantQuality},
		},
	}
	for m, steps := range want {
		for _, step := range Steps {
			got := candidates(m, step)
			if !reflect.DeepEqual(got, steps[step]) {
				t.Fatalf("%s %s candidates %v, want %v", m, step, got, steps[step])
			}
		}
	}
}
