package chromaprint

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// FingerprintResult holds the extracted audio fingerprint data
type FingerprintResult struct {
	Duration    float64 `json:"duration"`
	Fingerprint []int32 `json:"fingerprint"`
}

// fpcalcOutput matches the JSON output of fpcalc -json -raw -signed
type fpcalcOutput struct {
	Duration    float64 `json:"duration"`
	Fingerprint []int32 `json:"fingerprint"`
}

// ExtractFingerprint extracts an audio fingerprint from a video file
func ExtractFingerprint(videoPath string) (*FingerprintResult, error) {
	return ExtractFingerprintWithContext(context.Background(), videoPath)
}

// ExtractFingerprintWithContext extracts an audio fingerprint with context support
func ExtractFingerprintWithContext(ctx context.Context, videoPath string) (*FingerprintResult, error) {
	args := []string{"-raw", "-signed", "-length", "0", "-json", videoPath}

	cmd := exec.CommandContext(ctx, FpcalcPath(), args...)
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// fpcalc may exit non-zero (e.g. status 3 on last-frame decode error)
		// but still produce valid JSON output. Try to parse it before giving up.
		if len(output) == 0 {
			if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
				return nil, fmt.Errorf("fpcalc failed: %w, stderr: %s", err, string(exitErr.Stderr))
			}
			return nil, fmt.Errorf("fpcalc failed: %w", err)
		}
	}

	var result fpcalcOutput
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse fpcalc output: %w", err)
	}

	if len(result.Fingerprint) == 0 {
		return nil, fmt.Errorf("fpcalc returned empty fingerprint")
	}

	return &FingerprintResult{
		Duration:    result.Duration,
		Fingerprint: result.Fingerprint,
	}, nil
}
