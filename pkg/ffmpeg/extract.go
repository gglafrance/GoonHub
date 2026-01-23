package ffmpeg

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

func ExtractSpriteSheets(videoPath, outputDir string, videoID int, width, height, gridCols, gridRows, interval, quality int) ([]string, error) {
	metadata, err := GetMetadata(videoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get video metadata: %w", err)
	}

	duration := int(metadata.Duration)
	if duration < interval {
		return []string{}, nil
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create sprite directory: %w", err)
	}

	totalFrames := duration / interval
	if duration%interval != 0 {
		totalFrames++
	}

	framesPerSheet := gridCols * gridRows
	totalSheets := (totalFrames + framesPerSheet - 1) / framesPerSheet

	var spriteSheets []string
	for sheetIndex := 0; sheetIndex < totalSheets; sheetIndex++ {
		spriteName := fmt.Sprintf("%d_sheet_%03d.webp", videoID, sheetIndex+1)
		spritePath := filepath.Join(outputDir, spriteName)

		startTime := sheetIndex * framesPerSheet * interval
		vf := fmt.Sprintf("fps=1/%d,scale=%d:%d,tile=%dx%d", interval, width, height, gridCols, gridRows)

		args := GetDefaultArgs()
		args = append(args, []string{
			"-ss", strconv.Itoa(startTime),
			"-i", videoPath,
			"-vf", vf,
			"-q:v", strconv.Itoa(quality),
			"-frames:v", "1",
			"-y",
			spritePath,
		}...)

		cmd := exec.Command(FFMpegPath(), args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("ffmpeg failed for sprite sheet %d: %w, output: %s", sheetIndex+1, err, string(output))
		}

		spriteSheets = append(spriteSheets, spriteName)
	}

	return spriteSheets, nil
}
