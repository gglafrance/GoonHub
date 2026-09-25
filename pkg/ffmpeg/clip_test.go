package ffmpeg

import (
	"runtime"
	"strings"
	"testing"
)

func TestBuildClipArgs(t *testing.T) {
	opts := ClipOptions{CRF: 18, Preset: "medium", AudioBitrate: "192k"}
	args := buildClipArgs("/in/source.mp4", "/out/clip.mp4.partial", 83.5, 22.25, opts)

	want := []string{
		"-ss", "83.500",
		"-i", "/in/source.mp4",
		"-t", "22.250",
		"-map", "0:v:0",
		"-map", "0:a:0?",
		"-c:v", "libx264",
		"-preset", "medium",
		"-crf", "18",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "192k",
		"-movflags", "+faststart",
		"-map_metadata", "-1",
		"-progress", "pipe:1",
		"-nostats",
		"-f", "mp4",
		"-y",
		"/out/clip.mp4.partial",
	}
	if runtime.GOOS == "linux" {
		want = append([]string{"-nostdin"}, want...)
	}

	if strings.Join(args, " ") != strings.Join(want, " ") {
		t.Fatalf("buildClipArgs:\n got  %v\n want %v", args, want)
	}

	// No scaling or frame rate filters: resolution and fps must be kept.
	for _, a := range args {
		if a == "-vf" || a == "-r" || a == "-s" || a == "-filter_complex" {
			t.Fatalf("buildClipArgs must not rescale or change fps, found %q", a)
		}
	}
}

func TestParseClipProgress(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		duration float64
		wantPct  float64
		wantOK   bool
	}{
		{"half way", "out_time_us=5000000", 10, 0.5, true},
		{"start", "out_time_us=0", 10, 0, true},
		{"past the end is clamped", "out_time_us=12000000", 10, 1, true},
		{"negative is clamped", "out_time_us=-23000", 10, 0, true},
		{"trailing whitespace", "out_time_us=2500000\r", 10, 0.25, true},
		{"not available yet", "out_time_us=N/A", 10, 0, false},
		{"other key", "out_time_ms=5000000", 10, 0, false},
		{"progress marker", "progress=continue", 10, 0, false},
		{"zero duration", "out_time_us=5000000", 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pct, ok := parseClipProgress(tt.line, tt.duration)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && pct != tt.wantPct {
				t.Fatalf("pct = %v, want %v", pct, tt.wantPct)
			}
		})
	}
}

func TestTailBufferKeepsEnd(t *testing.T) {
	b := &tailBuffer{max: 5}
	_, _ = b.Write([]byte("abc"))
	_, _ = b.Write([]byte("defgh"))
	if got := b.String(); got != "defgh" {
		t.Fatalf("tail = %q, want %q", got, "defgh")
	}
}
