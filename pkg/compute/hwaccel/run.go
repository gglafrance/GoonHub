package hwaccel

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// runResult is one ffmpeg run.
type runResult struct {
	Wall   time.Duration
	CPU    time.Duration // user + system time of the process
	Stdout []byte
	Stderr []byte
}

// run executes bin with args and measures it. The error carries the tail of stderr.
func run(ctx context.Context, bin string, args ...string) (runResult, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	start := time.Now()
	err := cmd.Run()
	res := runResult{Wall: time.Since(start), Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}
	if cmd.ProcessState != nil {
		res.CPU = cmd.ProcessState.UserTime() + cmd.ProcessState.SystemTime()
	}
	if err != nil {
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		return res, fmt.Errorf("%w: %s", err, tail(stderr.String(), 400))
	}
	return res, nil
}

// tail returns the last n bytes of s, trimmed, on one line.
func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		s = "..." + s[len(s)-n:]
	}
	return strings.Join(strings.Fields(s), " ")
}

// ffprobePath returns the ffprobe next to ffmpegPath ("ffprobe" from PATH when ffmpegPath is
// a bare name).
func ffprobePath(ffmpegPath string) string {
	if ffmpegPath == "" || !strings.ContainsAny(ffmpegPath, `/\`) {
		return "ffprobe"
	}
	dir, base := filepath.Split(ffmpegPath)
	return filepath.Join(dir, strings.Replace(base, "ffmpeg", "ffprobe", 1))
}

func ffmpegBin(p string) string {
	if p == "" {
		return "ffmpeg"
	}
	return p
}

func ms(d time.Duration) int64 { return d.Milliseconds() }
