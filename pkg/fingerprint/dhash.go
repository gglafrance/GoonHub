// Package fingerprint provides perceptual video hashing for duplicate detection.
// It uses dHash (difference hash) on grayscale frames extracted at 1fps.
package fingerprint

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/bits"
	"os/exec"
)

// ComputeDHash computes a 64-bit difference hash from 9x8 grayscale pixels.
// The input must be exactly 72 bytes (9 columns x 8 rows).
// Each bit represents whether the pixel to the right is brighter than the current pixel.
func ComputeDHash(grayscalePixels []byte) uint64 {
	if len(grayscalePixels) != 72 {
		return 0
	}

	var hash uint64
	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			offset := row*9 + col
			if grayscalePixels[offset] < grayscalePixels[offset+1] {
				hash |= 1 << uint(row*8+col)
			}
		}
	}
	return hash
}

// HammingDistance computes the number of differing bits between two hashes.
func HammingDistance(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}

// ExtractAndHash extracts a single frame at the given timestamp and computes its dHash.
// Uses ffmpeg to extract a 9x8 grayscale raw frame.
func ExtractAndHash(ctx context.Context, videoPath string, timestampSec int) (uint64, error) {
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-ss", fmt.Sprintf("%d", timestampSec),
		"-i", videoPath,
		"-vframes", "1",
		"-vf", "scale=9:8,format=gray",
		"-f", "rawvideo",
		"-loglevel", "error",
		"pipe:1",
	)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("ffmpeg frame extraction at %ds failed: %w: %s", timestampSec, err, stderr.String())
	}

	data := stdout.Bytes()
	if len(data) != 72 {
		return 0, fmt.Errorf("unexpected frame size: got %d bytes, expected 72", len(data))
	}

	return ComputeDHash(data), nil
}

// ProgressCallback reports fingerprint extraction progress (0-100).
type ProgressCallback func(progress int)

// ExtractAllHashes extracts dHash values from a video using a single streaming ffmpeg process.
// Frames are sampled at 1 frame every intervalSec seconds using the fps filter.
func ExtractAllHashes(ctx context.Context, videoPath string, durationSec int, intervalSec int, progressCb ProgressCallback) ([]uint64, error) {
	if durationSec <= 0 {
		return nil, fmt.Errorf("invalid duration: %d", durationSec)
	}
	if intervalSec <= 0 {
		intervalSec = 2
	}

	expectedFrames := durationSec / intervalSec
	if expectedFrames <= 0 {
		expectedFrames = 1
	}

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", videoPath,
		"-vf", fmt.Sprintf("fps=1/%d,scale=9:8,format=gray", intervalSec),
		"-f", "rawvideo",
		"-loglevel", "error",
		"pipe:1",
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	hashes := make([]uint64, 0, expectedFrames)
	buf := make([]byte, 72) // 9x8 grayscale = 72 bytes per frame

	for {
		_, readErr := io.ReadFull(stdout, buf)
		if readErr != nil {
			if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
				break
			}
			// Context cancelled
			if ctx.Err() != nil {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
				return nil, ctx.Err()
			}
			break
		}

		hash := ComputeDHash(buf)
		hashes = append(hashes, hash)

		if progressCb != nil {
			progress := len(hashes) * 100 / expectedFrames
			if progress > 100 {
				progress = 100
			}
			progressCb(progress)
		}
	}

	waitErr := cmd.Wait()

	// If context was cancelled, return context error
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Tolerate non-zero exit if we got some hashes (some videos have trailing issues)
	if waitErr != nil && len(hashes) == 0 {
		return nil, fmt.Errorf("ffmpeg failed: %w: %s", waitErr, stderr.String())
	}

	if len(hashes) == 0 {
		return nil, fmt.Errorf("no frames extracted from video")
	}

	return hashes, nil
}
