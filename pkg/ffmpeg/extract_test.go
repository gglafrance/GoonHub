package ffmpeg

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// legacyScenePreviewArgs is the argument list ExtractScenePreviewWithContext built before
// audio support was added. Silent previews must keep producing exactly these arguments.
func legacyScenePreviewArgs(videoPath, outputPath string, duration, segments int, segmentDuration float64, width, crf int) []string {
	if float64(duration) < float64(segments)*segmentDuration {
		args := GetDefaultArgs()
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
	args := GetDefaultArgs()
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

func TestBuildScenePreviewArgs_NoAudioMatchesLegacy(t *testing.T) {
	tests := []struct {
		name     string
		duration int
	}{
		{"normal mode", 600},
		{"short mode", 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildScenePreviewArgs("in.mp4", "out.mp4", tt.duration, 12, 1.0, 360, 27, false)
			want := legacyScenePreviewArgs("in.mp4", "out.mp4", tt.duration, 12, 1.0, 360, 27)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("args changed for silent preview\n got: %v\nwant: %v", got, want)
			}
		})
	}
}

func TestBuildScenePreviewArgs_WithAudio(t *testing.T) {
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
			args := buildScenePreviewArgs("in.mp4", "out.mp4", tt.duration, 12, 1.0, 360, 27, true)
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

// makeTestClip renders a synthetic video (optionally with a sine tone) of the given length.
func makeTestClip(t *testing.T, path string, seconds int, withAudio bool) {
	t.Helper()
	args := []string{"-f", "lavfi", "-i", fmt.Sprintf("testsrc=duration=%d:size=320x240:rate=25", seconds)}
	if withAudio {
		args = append(args, "-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=440:duration=%d", seconds), "-c:a", "aac")
	}
	args = append(args, "-c:v", "libx264", "-pix_fmt", "yuv420p", "-y", path)
	if out, err := exec.Command(FFMpegPath(), args...).CombinedOutput(); err != nil {
		t.Fatalf("failed to create test clip: %v, output: %s", err, out)
	}
}

// countStreams returns how many streams of the given type ("a" or "v") the file has.
func countStreams(t *testing.T, path, kind string) int {
	t.Helper()
	out, err := exec.Command(FFprobePath(), "-v", "error", "-select_streams", kind,
		"-show_entries", "stream=codec_type", "-of", "csv=p=0", path).Output()
	if err != nil {
		t.Fatalf("ffprobe failed on %s: %v", path, err)
	}
	return len(strings.Fields(string(out)))
}

func TestExtractScenePreviewWithContext_Integration(t *testing.T) {
	if err := CheckInstallation(); err != nil {
		t.Skip("ffmpeg/ffprobe not on PATH")
	}

	dir := t.TempDir()
	withAudio := filepath.Join(dir, "with_audio.mp4")
	silent := filepath.Join(dir, "silent.mp4")
	makeTestClip(t, withAudio, 30, true)
	makeTestClip(t, silent, 30, false)

	tests := []struct {
		name       string
		input      string
		segments   int
		hasAudio   bool
		wantAudio  int
	}{
		{"normal mode with audio", withAudio, 6, true, 1},
		{"short mode with audio", withAudio, 60, true, 1},
		{"normal mode silent source", silent, 6, false, 0},
		{"short mode silent source", silent, 60, false, 0},
		{"audio source, audio off", withAudio, 6, false, 0},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := filepath.Join(dir, fmt.Sprintf("preview_%d.mp4", i))
			if err := ExtractScenePreviewWithContext(context.Background(), tt.input, out,
				30, tt.segments, 1.0, 160, 30, tt.hasAudio); err != nil {
				t.Fatalf("ExtractScenePreviewWithContext failed: %v", err)
			}
			if got := countStreams(t, out, "v"); got != 1 {
				t.Fatalf("video streams = %d, want 1", got)
			}
			if got := countStreams(t, out, "a"); got != tt.wantAudio {
				t.Fatalf("audio streams = %d, want %d", got, tt.wantAudio)
			}
		})
	}

	// A source without audio asked for audio fails, which is what triggers the
	// caller's retry without audio.
	t.Run("silent source with audio requested fails", func(t *testing.T) {
		out := filepath.Join(dir, "preview_fail.mp4")
		if err := ExtractScenePreviewWithContext(context.Background(), silent, out,
			30, 6, 1.0, 160, 30, true); err == nil {
			t.Fatal("expected an error when mapping a missing audio stream")
		}
	})
}
