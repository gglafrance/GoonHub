package dhash

import (
	"context"
	"fmt"
	"io"
	"math/bits"
	"os/exec"
)

// ExtractDHashes extracts perceptual dHash fingerprints from video frames
func ExtractDHashes(videoPath string) ([]uint64, error) {
	return ExtractDHashesWithContext(context.Background(), videoPath)
}

// ExtractDHashesWithContext extracts dHashes with context support for cancellation/timeout.
// It uses ffmpeg to extract 1 frame every 2 seconds, scale to 9x8 grayscale,
// and compute a 64-bit difference hash per frame.
func ExtractDHashesWithContext(ctx context.Context, videoPath string) ([]uint64, error) {
	args := []string{
		"-i", videoPath,
		"-vf", "fps=1/2,scale=9:8,format=gray",
		"-f", "rawvideo",
		"pipe:1",
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start ffmpeg for dhash: %w", err)
	}

	// Each frame is 9x8 = 72 bytes of grayscale pixels
	const frameSize = 9 * 8
	buf := make([]byte, frameSize)
	var hashes []uint64

	for {
		_, err := io.ReadFull(stdout, buf)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			// Check context cancellation
			if ctx.Err() != nil {
				cmd.Process.Kill()
				cmd.Wait()
				return nil, ctx.Err()
			}
			break
		}
		hashes = append(hashes, ComputeDHash(buf))
	}

	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// ffmpeg may exit with non-zero if we stop reading early, that's OK if we have hashes
		if len(hashes) == 0 {
			return nil, fmt.Errorf("ffmpeg dhash extraction failed: %w", err)
		}
	}

	return hashes, nil
}

// ComputeDHash computes a 64-bit difference hash from a 9x8 grayscale pixel buffer.
// For each of the 8 rows, compare each pixel to its right neighbor: 8 comparisons per row = 64 bits.
func ComputeDHash(pixels []byte) uint64 {
	var hash uint64
	bit := uint(0)
	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			left := pixels[row*9+col]
			right := pixels[row*9+col+1]
			if left > right {
				hash |= 1 << bit
			}
			bit++
		}
	}
	return hash
}

// HammingDistance returns the number of differing bits between two hashes
func HammingDistance(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}
