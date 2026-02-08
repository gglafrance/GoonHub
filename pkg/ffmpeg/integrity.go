package ffmpeg

import (
	"context"
	"os/exec"
)

// CheckVideoIntegrityWithContext verifies video file integrity by demuxing
// the entire file without decoding (-c copy). This reads every packet and
// catches truncation, corrupted headers, bad packet structure, and missing
// keyframes while remaining I/O-bound (fast even for large files).
// Returns (true, nil) for valid files, (false, nil) for corrupted files,
// and (false, err) for system errors.
func CheckVideoIntegrityWithContext(ctx context.Context, videoPath string) (bool, error) {
	args := GetDefaultArgs()
	args = append(args,
		"-v", "error",
		"-xerror",
		"-i", videoPath,
		"-c", "copy",
		"-f", "null",
		"-",
	)

	cmd := exec.CommandContext(ctx, FFMpegPath(), args...)
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		return false, nil
	}

	return true, nil
}
