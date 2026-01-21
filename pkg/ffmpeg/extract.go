package ffmpeg

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func ExtractThumbnail(videoPath, outputPath, seekPosition string, width, height, quality int) error {
	args := GetDefaultArgs()
	args = append(args, []string{
		"-ss", seekPosition,
		"-i", videoPath,
		"-vframes", "1",
		"-vf", fmt.Sprintf("scale=%d:%d", width, height),
		"-q:v", strconv.Itoa(quality),
		"-y",
		outputPath,
	}...)

	cmd := exec.Command(FFMpegPath(), args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg failed: %w, output: %s", err, string(output))
	}

	return nil
}

func ExtractFrames(videoPath, outputDir string, interval, width, height, quality int) ([]string, error) {
	metadata, err := GetMetadata(videoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get video metadata: %w", err)
	}

	duration := int(metadata.Duration)
	if duration < interval {
		return []string{}, nil
	}

	var framePaths []string
	for timestamp := 0; timestamp < duration; timestamp += interval {
		frameName := fmt.Sprintf("frame_%d.webp", timestamp)
		framePath := fmt.Sprintf("%s/%s", outputDir, frameName)

		args := GetDefaultArgs()
		args = append(args, []string{
			"-ss", strconv.Itoa(timestamp),
			"-i", videoPath,
			"-vframes", "1",
			"-vf", fmt.Sprintf("scale=%d:%d", width, height),
			"-q:v", strconv.Itoa(quality),
			"-y",
			framePath,
		}...)

		cmd := exec.Command(FFMpegPath(), args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("ffmpeg failed at timestamp %d: %w, output: %s", timestamp, err, string(output))
		}

		framePaths = append(framePaths, frameName)
	}

	return framePaths, nil
}

func ExtractFramesConcurrent(videoPath, outputDir string, interval, width, height, quality int) ([]string, error) {
	metadata, err := GetMetadata(videoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get video metadata: %w", err)
	}

	duration := int(metadata.Duration)
	if duration < interval {
		return []string{}, nil
	}

	var timestamps []int
	for timestamp := 0; timestamp < duration; timestamp += interval {
		timestamps = append(timestamps, timestamp)
	}

	type result struct {
		path      string
		timestamp int
		err       error
	}

	resultChan := make(chan result, len(timestamps))
	semaphore := make(chan struct{}, 4)

	for _, timestamp := range timestamps {
		go func(ts int) {
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			frameName := fmt.Sprintf("frame_%d.webp", ts)
			framePath := fmt.Sprintf("%s/%s", outputDir, frameName)

			args := GetDefaultArgs()
			args = append(args, []string{
				"-ss", strconv.Itoa(ts),
				"-i", videoPath,
				"-vframes", "1",
				"-vf", fmt.Sprintf("scale=%d:%d", width, height),
				"-q:v", strconv.Itoa(quality),
				"-y",
				framePath,
			}...)

			cmd := exec.Command(FFMpegPath(), args...)
			output, err := cmd.CombinedOutput()
			resultChan <- result{
				path:      frameName,
				timestamp: ts,
				err:       err,
			}
			if err != nil {
				resultChan <- result{
					err: fmt.Errorf("ffmpeg failed at timestamp %d: %w, output: %s", ts, err, string(output)),
				}
			}
		}(timestamp)
	}

	var framePaths []string
	for i := 0; i < len(timestamps); i++ {
		res := <-resultChan
		if res.err != nil {
			return nil, res.err
		}
		framePaths = append(framePaths, res.path)
	}

	return framePaths, nil
}

func ParseFramePaths(framePaths []string) string {
	return strings.Join(framePaths, ",")
}
