package compute

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"

	"goonhub/pkg/chromaprint"
	"goonhub/pkg/dhash"
	"goonhub/pkg/workerproto"
)

// Fingerprint extracts the fingerprints of the source, as the server does:
//   - audio (chromaprint via fpcalc) whenever the video has an audio track;
//   - visual (dHash of one frame every 2 s) when it has none, or always in dual mode.
//
// In dual mode one extraction may fail while the other succeeds: the result keeps the one
// that worked and records the other's error (AudioError / VisualError). Both read the whole
// file: for a remote source prefer a local copy. Errors are *StepError (StepAudioFingerprint
// or StepVisualHash), context errors included.
func Fingerprint(ctx context.Context, env Env, req Request, p workerproto.FingerprintParams) (*workerproto.FingerprintResult, error) {
	extractAudio := req.Video.AudioCodec != ""
	extractVisual := req.Video.AudioCodec == "" || p.Mode == workerproto.FingerprintModeDual

	res := &workerproto.FingerprintResult{}

	if extractAudio {
		fp, err := chromaprint.ExtractFingerprintWithBinary(ctx, env.FpcalcPath, req.Input)
		if err != nil {
			if ctx.Err() != nil {
				return nil, &StepError{Step: StepAudioFingerprint, Err: ctx.Err()}
			}
			err = fmt.Errorf("chromaprint extraction failed: %w", err)
			// The visual fingerprint alone is still usable for matching (e.g. a silent audio
			// track gives an empty chromaprint), so only fail if there is no visual pass.
			if !extractVisual {
				return nil, &StepError{Step: StepAudioFingerprint, Err: err}
			}
			res.AudioError = err.Error()
		} else {
			res.AudioFingerprint = fp.Fingerprint
			res.AudioDurationSec = fp.Duration
		}
	}

	if extractVisual {
		hashes, err := visualHashes(ctx, env, req.Input, req.Video.DurationSec)
		if err != nil {
			if ctx.Err() != nil {
				return nil, &StepError{Step: StepVisualHash, Err: ctx.Err()}
			}
			// Dual mode: keep the audio fingerprint if it worked
			if extractAudio && len(res.AudioFingerprint) > 0 {
				res.VisualError = err.Error()
			} else {
				return nil, &StepError{Step: StepVisualHash, Err: err}
			}
		} else {
			res.VisualFingerprint = hashes
		}
	}

	return res, nil
}

// errNoVisualFrames is returned when ffmpeg produced no frame at all.
var errNoVisualFrames = errors.New("no visual frames extracted")

// visualHashes decodes the video (durationSec long, 0 = unknown) to 9x8 grayscale frames and
// hashes them.
func visualHashes(ctx context.Context, env Env, input string, durationSec int) ([]uint64, error) {
	acc := env.Accel.For(StepVisualHash)
	var hashes []uint64
	var err error
	if acc.IsSoftware() {
		hashes, err = dhash.RunFFmpeg(ctx, env.ffmpeg(), visualHashArgs(acc, input))
	} else {
		err = withDeadline(ctx, acc.deadline(durationSec), func(ctx context.Context) error {
			var e error
			hashes, e = runHashesStrict(ctx, env.ffmpeg(), visualHashArgs(acc, input))
			return e
		})
	}
	if err == nil && len(hashes) == 0 {
		err = errNoVisualFrames
	}
	if err != nil && ctx.Err() == nil && !acc.IsSoftware() && !env.Accel.NoSoftwareFallback {
		hashes, err = dhash.RunFFmpeg(ctx, env.ffmpeg(), visualHashArgs(StepAccel{}, input))
		if err == nil && len(hashes) == 0 {
			err = errNoVisualFrames
		}
	}
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if errors.Is(err, errNoVisualFrames) {
			return nil, err
		}
		return nil, fmt.Errorf("dhash extraction failed: %w", err)
	}
	return hashes, nil
}

// VisualHashArgs returns the ffmpeg arguments of the visual hash decode (9x8 grayscale raw
// frames on stdout, one every 2 s) with the step acceleration acc; the zero StepAccel gives
// dhash.DefaultArgs. Exported for benchmarks and hardware probes (pkg/compute/hwaccel).
func VisualHashArgs(acc StepAccel, input string) []string {
	return visualHashArgs(acc, input)
}

// visualHashArgs are dhash.DefaultArgs with the step acceleration: the decoder skips the
// deblocking filter and the non-reference frames (frames end up 9x8, the hashes stay
// comparable with the indexed ones). Hardware decoders ignore -skip_loop_filter (they
// always deblock: their frames equal a full software decode) but honour -skip_frame.
func visualHashArgs(acc StepAccel, input string) []string {
	if acc.IsSoftware() {
		return dhash.DefaultArgs(input)
	}
	args := []string{"-nostdin"}
	args = append(args, acc.globalArgs()...)
	args = append(args, acc.inputArgs()...)
	return append(args,
		"-skip_loop_filter", "all",
		"-skip_frame", "noref",
		"-i", input,
		"-an", "-sn", "-dn",
		"-vf", "fps=1/2,"+acc.scale("9:8", "")+",format=gray",
		"-f", "rawvideo",
		"pipe:1",
	)
}

// runHashesStrict is dhash.RunFFmpeg for accelerated runs: any ffmpeg failure is an error,
// even after some frames (dhash.RunFFmpeg keeps the frames read so far). A GPU decode that
// breaks in the middle of a file must not commit a truncated fingerprint: the error makes
// the step run again in software.
func runHashesStrict(ctx context.Context, bin string, args []string) ([]uint64, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	dieWithParent(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start ffmpeg for dhash: %w", err)
	}
	const frameSize = 9 * 8
	buf := make([]byte, frameSize)
	var hashes []uint64
	var readErr error
	for {
		if _, err := io.ReadFull(stdout, buf); err != nil {
			if err != io.EOF && err != io.ErrUnexpectedEOF {
				readErr = err
			}
			break
		}
		hashes = append(hashes, dhash.ComputeDHash(buf))
	}
	if readErr != nil {
		_ = cmd.Process.Kill()
	}
	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if readErr != nil {
		return nil, fmt.Errorf("failed to read ffmpeg output: %w", readErr)
	}
	if waitErr != nil {
		msg := stderr.String()
		if len(msg) > 500 {
			msg = msg[len(msg)-500:]
		}
		return nil, fmt.Errorf("ffmpeg dhash extraction failed: %w, output: %s", waitErr, msg)
	}
	return hashes, nil
}
