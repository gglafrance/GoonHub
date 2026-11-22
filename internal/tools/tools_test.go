package tools

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestParseHWAccels(t *testing.T) {
	out := []byte("Hardware acceleration methods:\ncuda\nd3d11va\n\nqsv\n")
	if got := ParseHWAccels(out); !reflect.DeepEqual(got, []string{"cuda", "d3d11va", "qsv"}) {
		t.Fatalf("%v", got)
	}
}

func TestParseEncoders(t *testing.T) {
	out := []byte(`Encoders:
 V..... = Video
 ------
 V....D libx264              libx264 H.264 / AVC
 V....D h264_nvenc           NVIDIA NVENC H.264 encoder (codec h264)
 V....D libvpx-vp9           libvpx VP9
 V....D hevc_videotoolbox    VideoToolbox H.265 Encoder (codec hevc)
 A....D aac                  AAC (Advanced Audio Coding)
`)
	want := []string{"libx264", "h264_nvenc", "hevc_videotoolbox"}
	if got := ParseEncoders(out); !reflect.DeepEqual(got, want) {
		t.Fatalf("%v, want %v", got, want)
	}
}

func TestFindMissingConfiguredBinary(t *testing.T) {
	_, err := Find(context.Background(), Configured{FFmpeg: "/definitely/not/ffmpeg", FFprobe: "/definitely/not/ffprobe"})
	if !errors.Is(err, ErrMissing) {
		t.Fatalf("error %v, want ErrMissing", err)
	}
}
