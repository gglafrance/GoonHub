package hwaccel

import (
	"reflect"
	"testing"
)

const hwaccelsOut = `Hardware acceleration methods:
cuda
vaapi
dxva2
qsv
d3d11va

`

const encodersOut = `Encoders:
 V..... = Video
 A..... = Audio
 ------
 V....D libx264              libx264 H.264 / AVC / MPEG-4 AVC / MPEG-4 part 10 (codec h264)
 V....D h264_nvenc           NVIDIA NVENC H.264 encoder (codec h264)
 V..... h264_qsv             H.264 / AVC / MPEG-4 AVC / MPEG-4 part 10 (Intel Quick Sync Video acceleration) (codec h264)
 A....D aac                  AAC (Advanced Audio Coding)
`

const filtersOut = `Filters:
  T.. = Timeline support
  .S. = Slice threading
  A = Audio input/output
  | = Source or sink filter
 ... abench            A->A       Benchmark part of a filtergraph.
 .. hwdownload        V->V       Download a hardware frame to a normal frame
 T. overlay_cuda      VV->V      Overlay one video on top of another using CUDA
 .. scale_cuda        V->V       GPU accelerated video resizer
 .. vsrc_amf          |->V       AMD AMF screen capture
`

func TestParseVersion(t *testing.T) {
	tests := map[string]string{
		"ffmpeg version N-124716-g054dffd133-20260531 Copyright (c) 2000-2026 the FFmpeg developers\nbuilt with gcc": "N-124716-g054dffd133-20260531",
		"ffmpeg version 7.1.1 Copyright (c) 2000-2025":                                                               "7.1.1",
		"something else": "something else",
	}
	for in, want := range tests {
		if got := ParseVersion(in); got != want {
			t.Fatalf("ParseVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseHWAccels(t *testing.T) {
	want := []string{"cuda", "vaapi", "dxva2", "qsv", "d3d11va"}
	if got := ParseHWAccels(hwaccelsOut); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got := ParseHWAccels(""); got != nil {
		t.Fatalf("empty output gave %v", got)
	}
}

func TestParseCodecList(t *testing.T) {
	want := []string{"libx264", "h264_nvenc", "h264_qsv", "aac"}
	if got := ParseCodecList(encodersOut); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestParseFilters(t *testing.T) {
	want := []string{"abench", "hwdownload", "overlay_cuda", "scale_cuda", "vsrc_amf"}
	if got := ParseFilters(filtersOut); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestFFprobePath(t *testing.T) {
	tests := map[string]string{
		"":                       "ffprobe",
		"ffmpeg":                 "ffprobe",
		"/opt/ffmpeg/bin/ffmpeg": "/opt/ffmpeg/bin/ffprobe",
	}
	for in, want := range tests {
		got := ffprobePath(in)
		if in != "" && in != "ffmpeg" {
			// filepath.Join uses the OS separator
			if got != want && got != `\opt\ffmpeg\bin\ffprobe` {
				t.Fatalf("ffprobePath(%q) = %q", in, got)
			}
			continue
		}
		if got != want {
			t.Fatalf("ffprobePath(%q) = %q, want %q", in, got, want)
		}
	}
}
