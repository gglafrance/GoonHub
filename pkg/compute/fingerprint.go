package compute

import (
	"context"
	"errors"
	"fmt"

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
		hashes, err := visualHashes(ctx, env, req.Input)
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

// visualHashes decodes the video to 9x8 grayscale frames and hashes them.
func visualHashes(ctx context.Context, env Env, input string) ([]uint64, error) {
	acc := env.Accel.For(StepVisualHash)
	hashes, err := dhash.RunFFmpeg(ctx, env.ffmpeg(), visualHashArgs(acc, input))
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

// visualHashArgs are dhash.DefaultArgs with the step acceleration: the decoder skips the
// deblocking filter and the non-reference frames (frames end up 9x8, the hashes stay
// comparable with the indexed ones).
func visualHashArgs(acc StepAccel, input string) []string {
	if acc.IsSoftware() {
		return dhash.DefaultArgs(input)
	}
	args := []string{"-nostdin"}
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
