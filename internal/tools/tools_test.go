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

func TestHasWebPEncoder(t *testing.T) {
	with := []byte(" V....D libx264              libx264 H.264 / AVC\n V....D libwebp_anim         libwebp WebP image (codec webp)\n V....D libwebp              libwebp WebP image (codec webp)\n")
	without := []byte(" V....D libx264              libx264 H.264 / AVC\n V....D png                  PNG (Portable Network Graphics) image\n")
	if !HasWebPEncoder(with) {
		t.Fatal("libwebp not detected")
	}
	if HasWebPEncoder(without) {
		t.Fatal("WebP detected in an ffmpeg without libwebp")
	}
	tl := &Tools{Encoders: []string{"libx264"}}
	if tl.MissingForPhase("sprites") == "" {
		t.Fatal("sprites allowed without WebP")
	}
	if tl.MissingForPhase("animated_thumbnails") != "" {
		t.Fatal("animated thumbnails refused with libx264")
	}
}
