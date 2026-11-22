// Package compute is the server-independent computation of the phases a remote worker can
// run: sprites (sprite sheets + VTT), animated_thumbnails (marker clips + scene preview) and
// fingerprint (chromaprint audio fingerprint + visual dHashes).
//
// It is used both by the GoonHub server (local processing, internal/jobs) and by
// goonhub-worker: same ffmpeg commands, same output names, same quality. It reads a source
// video (local path or http(s) URL), writes the output files into an output directory and
// returns a typed result (pkg/workerproto); it never touches a database. Committing the
// result (moving files into place, DB updates, fingerprint matching) is the server's job.
//
// The source is only ever opened for reading. The package builds on Windows, macOS and Linux.
package compute

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"goonhub/pkg/ffmpeg"
	"goonhub/pkg/workerproto"
)

// Env is the execution environment of a computation.
type Env struct {
	// FFmpegPath, FFprobePath, FpcalcPath are the binaries ("" = ffmpeg, ffprobe, fpcalc
	// from PATH).
	FFmpegPath  string
	FFprobePath string
	FpcalcPath  string
	// Accel is the hardware acceleration choice (zero value = software).
	Accel Accel
	// TempDir holds intermediate files such as sprite frames ("" = the OS temp dir).
	TempDir string
}

func (e Env) ffmpeg() string {
	if e.FFmpegPath != "" {
		return e.FFmpegPath
	}
	return ffmpeg.FFMpegPath()
}

func (e Env) ffprobe() string {
	if e.FFprobePath != "" {
		return e.FFprobePath
	}
	return ffmpeg.FFprobePath()
}

// ProgressFunc receives the progress of a computation, 0-100. It may be called from several
// goroutines at once (sprite frames are extracted in parallel) and values may arrive out of
// order: keep the maximum.
type ProgressFunc func(percent int)

// Request is the common input of a phase computation.
type Request struct {
	// Input is the source video: a local path (opened read-only) or an http(s) URL. ffmpeg
	// reads both; phases that read the whole file (fingerprint) are faster on a local copy,
	// the caller decides whether to download first.
	Input string
	// Video are the probed facts the server knows about the source.
	Video workerproto.VideoInfo
	// OutDir receives the output files (created if missing). Unused by fingerprint.
	OutDir string
	// Progress is optional.
	Progress ProgressFunc
}

func (r Request) report(p int) {
	if r.Progress != nil {
		r.Progress(p)
	}
}

// IsURL reports whether input is an http(s) URL rather than a local path.
func IsURL(input string) bool {
	l := strings.ToLower(input)
	return strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://")
}

// StepError tells which step of a phase failed. Context errors (cancellation, deadline)
// are wrapped too: use errors.Is(err, context.Canceled / context.DeadlineExceeded).
type StepError struct {
	Step Step
	Err  error
}

func (e *StepError) Error() string { return e.Err.Error() }
func (e *StepError) Unwrap() error { return e.Err }

// FailedStep returns the step of a StepError in err's chain, or "".
func FailedStep(err error) Step {
	var se *StepError
	if errors.As(err, &se) {
		return se.Step
	}
	return ""
}

// Run computes one phase from its parameters (dispatch on phase).
func Run(ctx context.Context, env Env, req Request, phase workerproto.Phase, params workerproto.PhaseParams) (workerproto.PhaseResult, error) {
	if err := params.Validate(phase); err != nil {
		return workerproto.PhaseResult{}, err
	}
	switch phase {
	case workerproto.PhaseSprites:
		res, err := Sprites(ctx, env, req, *params.Sprites)
		return workerproto.PhaseResult{Sprites: res}, err
	case workerproto.PhaseAnimatedThumbnails:
		res, err := AnimatedThumbnails(ctx, env, req, *params.AnimatedThumbnails)
		return workerproto.PhaseResult{AnimatedThumbnails: res}, err
	case workerproto.PhaseFingerprint:
		res, err := Fingerprint(ctx, env, req, *params.Fingerprint)
		return workerproto.PhaseResult{Fingerprint: res}, err
	}
	return workerproto.PhaseResult{}, fmt.Errorf("unsupported phase %q", phase)
}

// runFFmpeg runs ffmpeg with the arguments build returns for the step's acceleration; if an
// accelerated run fails while ctx is alive, it retries in software (unless disabled).
// errPrefix prefixes the error of a failed run ("ffmpeg failed").
func runFFmpeg(ctx context.Context, env Env, step Step, errPrefix string, build func(StepAccel) []string) error {
	acc := env.Accel.For(step)
	err := runOnce(ctx, env, errPrefix, build(acc))
	if err == nil || ctx.Err() != nil || acc.IsSoftware() || env.Accel.NoSoftwareFallback {
		return err
	}
	return runOnce(ctx, env, errPrefix, build(StepAccel{}))
}

func runOnce(ctx context.Context, env Env, errPrefix string, args []string) error {
	cmd := exec.CommandContext(ctx, env.ffmpeg(), args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%s: %w, output: %s", errPrefix, err, string(output))
	}
	return nil
}

// checkLocalInput fails when a local input does not exist (URLs are not checked).
func checkLocalInput(input string) error {
	if input == "" {
		return fmt.Errorf("scene has no stored path")
	}
	if IsURL(input) {
		return nil
	}
	if _, err := os.Stat(input); os.IsNotExist(err) {
		return fmt.Errorf("scene file not found: %s", input)
	}
	return nil
}
