package compute

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"goonhub/pkg/dhash"
	"goonhub/pkg/ffmpeg"
)

// The legacy* functions are the argument lists the server built before pkg/compute (in
// pkg/ffmpeg). Software runs must keep producing exactly these arguments.

func legacyScenePreviewArgs(videoPath, outputPath string, duration, segments int, segmentDuration float64, width, crf int) []string {
	if float64(duration) < float64(segments)*segmentDuration {
		args := ffmpeg.GetDefaultArgs()
		return append(args,
			"-i", videoPath,
			"-c:v", "libx264",
			"-vf", fmt.Sprintf("scale=%d:-2:flags=bilinear", width),
			"-pix_fmt", "yuv420p",
			"-preset", "veryfast",
			"-crf", strconv.Itoa(crf),
			"-movflags", "+faststart",
			"-map_metadata", "-1",
			"-threads", "4",
			"-an",
			"-y",
			outputPath,
		)
	}

	interval := float64(duration) / float64(segments)
	args := ffmpeg.GetDefaultArgs()
	for i := 0; i < segments; i++ {
		seekPos := interval*float64(i) + interval/2
		args = append(args, "-ss", fmt.Sprintf("%.2f", seekPos), "-i", videoPath)
	}
	var filterParts []string
	var concatInputs []string
	for i := 0; i < segments; i++ {
		label := fmt.Sprintf("v%d", i)
		filterParts = append(filterParts,
			fmt.Sprintf("[%d:v]trim=0:%.2f,setpts=PTS-STARTPTS,scale=%d:-2:flags=bilinear,format=yuv420p[%s]",
				i, segmentDuration, width, label))
		concatInputs = append(concatInputs, fmt.Sprintf("[%s]", label))
	}
	filterParts = append(filterParts,
		fmt.Sprintf("%sconcat=n=%d:v=1:a=0[out]", strings.Join(concatInputs, ""), segments))
	return append(args,
		"-filter_complex", strings.Join(filterParts, ";"),
		"-map", "[out]",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", strconv.Itoa(crf),
		"-movflags", "+faststart",
		"-map_metadata", "-1",
		"-threads", "4",
		"-an",
		"-y",
		outputPath,
	)
}

func legacyMarkerClipArgs(videoPath, outputPath, seekPosition string, duration, width, crf int) []string {
	args := ffmpeg.GetDefaultArgs()
	return append(args,
		"-ss", seekPosition,
		"-i", videoPath,
		"-t", strconv.Itoa(duration),
		"-c:v", "libx264",
		"-vf", fmt.Sprintf("scale=%d:-2:flags=bilinear", width),
		"-pix_fmt", "yuv420p",
		"-preset", "veryfast",
		"-crf", strconv.Itoa(crf),
		"-movflags", "+faststart",
		"-map_metadata", "-1",
		"-threads", "2",
		"-an",
		"-y",
		outputPath,
	)
}

func legacySpriteFrameArgs(videoPath, framePath string, ts, width, height, quality int) []string {
	args := ffmpeg.GetDefaultArgs()
	return append(args,
		"-ss", strconv.Itoa(ts),
		"-i", videoPath,
		"-threads", "1",
		"-vframes", "1",
		"-vf", fmt.Sprintf("scale=%d:%d", width, height),
		"-q:v", strconv.Itoa(quality),
		"-y",
		framePath,
	)
}

func TestScenePreviewArgs_SoftwareMatchesLegacy(t *testing.T) {
	tests := []struct {
		name     string
		duration int
	}{
		{"normal mode", 600},
		{"short mode", 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scenePreviewArgs(StepAccel{}, "in.mp4", "out.mp4", tt.duration, 12, 1.0, 360, 27, false)
			want := legacyScenePreviewArgs("in.mp4", "out.mp4", tt.duration, 12, 1.0, 360, 27)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("args changed for silent preview\n got: %v\nwant: %v", got, want)
			}
		})
	}
}

func TestScenePreviewArgs_WithAudio(t *testing.T) {
	tests := []struct {
		name     string
		duration int
		contains []string
	}{
		{"normal mode", 600, []string{"a=1", "[aout]", "aac", "[0:a:0]atrim=0:1.00", "afade=t=out:st=0.95:d=0.05", "[v11][a11]concat"}},
		{"short mode", 5, []string{"0:v:0", "0:a:0", "aac"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := scenePreviewArgs(StepAccel{}, "in.mp4", "out.mp4", tt.duration, 12, 1.0, 360, 27, true)
			joined := strings.Join(args, " ")
			for _, want := range tt.contains {
				if !strings.Contains(joined, want) {
					t.Fatalf("args missing %q: %s", want, joined)
				}
			}
			for _, a := range args {
				if a == "-an" {
					t.Fatalf("args with audio must not contain -an: %s", joined)
				}
			}
		})
	}
}

func TestMarkerClipArgs_SoftwareMatchesLegacy(t *testing.T) {
	got := markerClipArgs(StepAccel{}, "in.mp4", "marker_7.mp4", "42", 10, 480, 32)
	want := legacyMarkerClipArgs("in.mp4", "marker_7.mp4", "42", 10, 480, 32)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("marker clip args changed\n got: %v\nwant: %v", got, want)
	}
}

func TestSpriteFrameArgs_SoftwareMatchesLegacy(t *testing.T) {
	got := spriteFrameArgs(StepAccel{}, "in.mp4", "frame_0003.webp", 30, 320, 180, 75)
	want := legacySpriteFrameArgs("in.mp4", "frame_0003.webp", 30, 320, 180, 75)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sprite frame args changed\n got: %v\nwant: %v", got, want)
	}
}

func TestVisualHashArgs_SoftwareMatchesDHash(t *testing.T) {
	if got, want := visualHashArgs(StepAccel{}, "in.mp4"), dhash.DefaultArgs("in.mp4"); !reflect.DeepEqual(got, want) {
		t.Fatalf("visual hash args changed\n got: %v\nwant: %v", got, want)
	}
}

func TestAccelArgs_HardwarePaths(t *testing.T) {
	nvidia := StepAccel{
		Decode:  DecodeAccel{HWAccel: "cuda", OutputFormat: "cuda"},
		Scale:   ScaleAccel{Filter: "scale_cuda"},
		Encoder: EncoderAccel{H264: "h264_nvenc", QualityFlag: "-cq", QualityOffset: 2, ExtraArgs: []string{"-preset", "p4"}},
	}
	clip := strings.Join(markerClipArgs(nvidia, "in.mp4", "out.mp4", "5", 10, 480, 30), " ")
	for _, want := range []string{"-hwaccel cuda -hwaccel_output_format cuda -ss 5 -i in.mp4", "-c:v h264_nvenc", "-vf scale_cuda=480:-2", "-cq 32 -preset p4"} {
		if !strings.Contains(clip, want) {
			t.Fatalf("marker clip args missing %q: %s", want, clip)
		}
	}
	for _, unwanted := range []string{"-pix_fmt", "libx264", "-crf", "flags=bilinear"} {
		if strings.Contains(clip, unwanted) {
			t.Fatalf("GPU marker clip args must not contain %q: %s", unwanted, clip)
		}
	}

	// GPU decode + scale, frames downloaded for a CPU filter after
	hashAcc := StepAccel{
		Decode: DecodeAccel{HWAccel: "cuda", OutputFormat: "cuda"},
		Scale:  ScaleAccel{Filter: "scale_cuda", DownloadFormat: "nv12"},
	}
	hash := strings.Join(visualHashArgs(hashAcc, "in.mp4"), " ")
	if !strings.Contains(hash, "fps=1/2,scale_cuda=9:8,hwdownload,format=nv12,format=gray") {
		t.Fatalf("visual hash filter wrong: %s", hash)
	}

	preview := strings.Join(scenePreviewArgs(nvidia, "in.mp4", "out.mp4", 600, 3, 1.0, 360, 27, false), " ")
	if strings.Count(preview, "-hwaccel cuda") != 3 {
		t.Fatalf("each preview input needs the decoder options: %s", preview)
	}
	if strings.Contains(preview, "format=yuv420p") {
		t.Fatalf("frames kept on the GPU must not get a CPU pixel format: %s", preview)
	}
}

func TestQualityArgs(t *testing.T) {
	tests := []struct {
		name string
		enc  EncoderAccel
		crf  int
		want []string
	}{
		{"software", EncoderAccel{}, 27, []string{"-preset", "veryfast", "-crf", "27"}},
		{"offset", EncoderAccel{H264: "h264_nvenc", QualityFlag: "-cq", QualityOffset: 3}, 27, []string{"-cq", "30"}},
		{"scale and clamp", EncoderAccel{H264: "h264_videotoolbox", QualityFlag: "-q:v", QualityScale: 3, QualityMin: 1, QualityMax: 70}, 27, []string{"-q:v", "70"}},
		{"custom", EncoderAccel{H264: "h264_amf", QualityArgs: func(crf int) []string { return []string{"-qp_i", strconv.Itoa(crf)} }, ExtraArgs: []string{"-quality", "speed"}}, 25, []string{"-qp_i", "25", "-quality", "speed"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StepAccel{Encoder: tt.enc}.qualityArgs(tt.crf)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("qualityArgs = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsURL(t *testing.T) {
	tests := map[string]bool{
		"/data/a.mp4":                 false,
		`C:\videos\a.mp4`:             false,
		"http://host:8080/x?sig=1":    true,
		"HTTPS://host/api/v1/workers": true,
		"httpfile.mp4":                false,
	}
	for in, want := range tests {
		if got := IsURL(in); got != want {
			t.Fatalf("IsURL(%q) = %v, want %v", in, got, want)
		}
	}
}
