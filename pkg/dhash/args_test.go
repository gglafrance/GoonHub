package dhash

import (
	"context"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestDefaultArgs_DecoderOptionsBeforeInput(t *testing.T) {
	args := DefaultArgs("/videos/a.mp4")
	in := slices.Index(args, "-i")
	if in < 0 || args[in+1] != "/videos/a.mp4" {
		t.Fatalf("input missing: %v", args)
	}
	// decoder options only apply before -i
	for _, opt := range [][2]string{{"-threads", "1"}, {"-export_side_data", "film_grain"}, {"-skip_frame", "noref"}, {"-skip_loop_filter", "all"}} {
		i := slices.Index(args, opt[0])
		if i < 0 || i > in || args[i+1] != opt[1] {
			t.Fatalf("%s %s must be set before -i: %v", opt[0], opt[1], args)
		}
	}
}

// The single-threaded decoder must give exactly the hashes of the previous (multi-threaded)
// command, so the hashes already indexed stay comparable. Needs ffmpeg with libx264.
func TestDefaultArgs_SameHashesAsMultiThreadedDecode(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	clip := filepath.Join(t.TempDir(), "clip.mp4")
	gen := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "lavfi",
		"-i", "testsrc2=duration=30:size=640x360:rate=30,noise=alls=20:allf=t",
		"-c:v", "libx264", "-preset", "veryfast", "-bf", "3", "-g", "90", "-pix_fmt", "yuv420p", "-y", clip)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Skipf("cannot encode a test clip (libx264?): %v: %s", err, out)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	previous := []string{
		"-nostdin", "-skip_loop_filter", "all", "-skip_frame", "noref",
		"-i", clip, "-an", "-sn", "-dn", "-vf", "fps=1/2,scale=9:8,format=gray", "-f", "rawvideo", "pipe:1",
	}
	want := runHashes(t, ctx, previous)
	got, err := ExtractDHashesWithContext(ctx, clip)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(want) == 0 || !slices.Equal(got, want) {
		t.Fatalf("hashes differ from the multi-threaded decode:\n got %x\nwant %x", got, want)
	}
}

func runHashes(t *testing.T, ctx context.Context, args []string) []uint64 {
	t.Helper()
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("ffmpeg: %v", err)
	}
	var hashes []uint64
	for i := 0; i+72 <= len(out); i += 72 {
		hashes = append(hashes, ComputeDHash(out[i:i+72]))
	}
	return hashes
}
