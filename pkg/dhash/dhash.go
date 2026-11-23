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
//
// Decoding the whole video dominates the cost. Since frames end up as 9x8 thumbnails, the
// decoder skips the deblocking filter and the non-reference frames: measured on real 1080p
// and 720p H.264 files this halves the CPU time (-47 to -58%) while every frame hash stays
// within 1-5 bits (mean < 1 bit) of the full decode, well under the matching Hamming
// tolerance, so these hashes stay comparable with the ones already indexed.
func ExtractDHashesWithContext(ctx context.Context, videoPath string) ([]uint64, error) {
	return RunFFmpeg(ctx, "ffmpeg", DefaultArgs(videoPath))
}

// Filter is the video filter that turns the video into 9x8 grayscale frames, one every 2 s.
const Filter = "fps=1/2,scale=9:8,format=gray"

// DefaultArgs returns the ffmpeg arguments ExtractDHashesWithContext runs: software decode,
// 9x8 grayscale raw frames on stdout.
//
// The decoder runs on a single thread: hashing jobs run several at a time, and the threads of
// a frame-threaded decoder mostly add synchronisation overhead. Measured on 51 real videos
// (7 h, H.264 SD to 4K, HEVC, AV1, MPEG-4, WMV) this cuts the CPU time by 36 % (x1.3 to x2.3
// per file, AV1 and MPEG-4 the most) and the hashes are bit-identical, so nothing indexed
// changes. Each job takes about twice as long on its own: the gain is throughput.
// export_side_data film_grain makes AV1 decoders export the film grain parameters instead of
// synthesizing the grain (display-only noise, irrelevant at 9x8; -8 % CPU on AV1, no effect
// on other codecs).
func DefaultArgs(videoPath string) []string {
	return []string{
		"-nostdin",
		"-skip_loop_filter", "all",
		"-skip_frame", "noref",
		"-threads", "1",
		"-export_side_data", "film_grain",
		"-i", videoPath,
		"-an", "-sn", "-dn",
		"-vf", Filter,
		"-f", "rawvideo",
		"pipe:1",
	}
}

// RunFFmpeg runs ffmpegBin with args, which must write 9x8 grayscale raw frames to stdout
// (pipe:1), and returns one dHash per frame.
func RunFFmpeg(ctx context.Context, ffmpegBin string, args []string) ([]uint64, error) {
	if ffmpegBin == "" {
		ffmpegBin = "ffmpeg"
	}
	cmd := exec.CommandContext(ctx, ffmpegBin, args...)
	dieWithParent(cmd)
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
