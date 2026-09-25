package ffmpeg

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

// ClipOptions controls how a clip is encoded.
type ClipOptions struct {
	CRF          int
	Preset       string
	AudioBitrate string
}

// maxStderrTail is how much of ffmpeg's stderr is kept for error messages.
const maxStderrTail = 4096

// ExtractClipWithContext re-encodes the range [start, start+duration) of in into
// an MP4 at out. Resolution and frame rate are kept; the re-encode makes the cut
// frame-accurate. The output is always MP4, whatever the extension of out, so a
// temporary name such as "clip.mp4.partial" can be used. onProgress, if set,
// receives values from 0 to 1.
func ExtractClipWithContext(ctx context.Context, in, out string, start, duration float64, opts ClipOptions, onProgress func(pct float64)) error {
	if duration <= 0 {
		return fmt.Errorf("clip duration must be positive, got %f", duration)
	}

	cmd := exec.CommandContext(ctx, FFMpegPath(), buildClipArgs(in, out, start, duration, opts)...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to open ffmpeg stdout: %w", err)
	}
	stderr := &tailBuffer{max: maxStderrTail}
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		if pct, ok := parseClipProgress(scanner.Text(), duration); ok && onProgress != nil {
			onProgress(pct)
		}
	}
	// Drain anything left so ffmpeg never blocks on a full pipe.
	_, _ = io.Copy(io.Discard, stdout)

	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("ffmpeg clip failed: %w, output: %s", err, stderr.String())
	}

	return nil
}

// buildClipArgs builds the ffmpeg arguments for ExtractClipWithContext.
func buildClipArgs(in, out string, start, duration float64, opts ClipOptions) []string {
	args := GetDefaultArgs()
	return append(args,
		"-ss", formatSeconds(start),
		"-i", in,
		"-t", formatSeconds(duration),
		"-map", "0:v:0",
		"-map", "0:a:0?",
		"-c:v", "libx264",
		"-preset", opts.Preset,
		"-crf", strconv.Itoa(opts.CRF),
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", opts.AudioBitrate,
		"-movflags", "+faststart",
		"-map_metadata", "-1",
		"-progress", "pipe:1",
		"-nostats",
		"-f", "mp4",
		"-y",
		out,
	)
}

// parseClipProgress reads one line of ffmpeg's -progress output and returns the
// fraction done, clamped to [0, 1]. Only out_time_us lines carry a position.
func parseClipProgress(line string, duration float64) (float64, bool) {
	value, found := strings.CutPrefix(strings.TrimSpace(line), "out_time_us=")
	if !found || duration <= 0 {
		return 0, false
	}
	us, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, false
	}
	pct := float64(us) / 1e6 / duration
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	return pct, true
}

func formatSeconds(s float64) string {
	return strconv.FormatFloat(s, 'f', 3, 64)
}

// tailBuffer keeps the last max bytes written to it.
type tailBuffer struct {
	max int
	buf []byte
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.buf = append(b.buf, p...)
	if len(b.buf) > b.max {
		b.buf = b.buf[len(b.buf)-b.max:]
	}
	return len(p), nil
}

func (b *tailBuffer) String() string {
	return string(b.buf)
}
