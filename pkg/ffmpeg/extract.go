package ffmpeg

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
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

func ResizeImageToWebp(inputPath, outputPath string, width, height, quality int) error {
	args := GetDefaultArgs()
	args = append(args,
		"-i", inputPath,
		"-vf", fmt.Sprintf("scale=%d:%d", width, height),
		"-q:v", strconv.Itoa(quality),
		"-y",
		outputPath,
	)

	cmd := exec.Command(FFMpegPath(), args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg failed: %w, output: %s", err, string(output))
	}
	return nil
}

func ExtractSpriteSheets(videoPath, outputDir string, videoID int, width, height, gridCols, gridRows, interval, quality, concurrency int) ([]string, error) {
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

	// Create temp directory for individual frame extraction
	tmpDir, err := os.MkdirTemp("", "goonhub-sprites-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Phase 1: Extract all frames in parallel using input seeking.
	// Input seeking (-ss before -i) jumps to the nearest keyframe and only decodes
	// a few frames, which is much faster than the fps filter that decodes every frame.
	if concurrency <= 0 {
		concurrency = runtime.NumCPU()
		if concurrency < 4 {
			concurrency = 4
		}
	}

	semaphore := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	errChan := make(chan error, totalFrames)

	for i := 0; i < totalFrames; i++ {
		wg.Add(1)
		go func(frameIndex int) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			ts := frameIndex * interval
			framePath := filepath.Join(tmpDir, fmt.Sprintf("frame_%04d.webp", frameIndex))

			args := GetDefaultArgs()
			args = append(args,
				"-ss", strconv.Itoa(ts),
				"-i", videoPath,
				"-threads", "1",
				"-vframes", "1",
				"-vf", fmt.Sprintf("scale=%d:%d", width, height),
				"-q:v", strconv.Itoa(quality),
				"-y",
				framePath,
			)

			cmd := exec.Command(FFMpegPath(), args...)
			if output, err := cmd.CombinedOutput(); err != nil {
				errChan <- fmt.Errorf("ffmpeg failed extracting frame at %ds: %w, output: %s", ts, err, string(output))
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	if err := <-errChan; err != nil {
		return nil, err
	}

	// Phase 2: Tile extracted frames into sprite sheets
	var spriteSheets []string
	for sheetIndex := 0; sheetIndex < totalSheets; sheetIndex++ {
		spriteName := fmt.Sprintf("%d_sheet_%03d.webp", videoID, sheetIndex+1)
		spritePath := filepath.Join(outputDir, spriteName)

		startFrame := sheetIndex * framesPerSheet
		endFrame := startFrame + framesPerSheet
		if endFrame > totalFrames {
			endFrame = totalFrames
		}

		// Create a temporary directory with sequential symlinks for this sheet
		sheetDir, err := os.MkdirTemp("", "goonhub-sheet-*")
		if err != nil {
			return nil, fmt.Errorf("failed to create sheet temp directory: %w", err)
		}

		for i := startFrame; i < endFrame; i++ {
			src := filepath.Join(tmpDir, fmt.Sprintf("frame_%04d.webp", i))
			dst := filepath.Join(sheetDir, fmt.Sprintf("%04d.webp", i-startFrame))
			if err := os.Symlink(src, dst); err != nil {
				os.RemoveAll(sheetDir)
				return nil, fmt.Errorf("failed to create symlink: %w", err)
			}
		}

		args := GetDefaultArgs()
		args = append(args,
			"-framerate", "1",
			"-i", filepath.Join(sheetDir, "%04d.webp"),
			"-vf", fmt.Sprintf("tile=%dx%d", gridCols, gridRows),
			"-q:v", strconv.Itoa(quality),
			"-frames:v", "1",
			"-y",
			spritePath,
		)

		cmd := exec.Command(FFMpegPath(), args...)
		output, cmdErr := cmd.CombinedOutput()
		os.RemoveAll(sheetDir)
		if cmdErr != nil {
			return nil, fmt.Errorf("ffmpeg failed tiling sprite sheet %d: %w, output: %s", sheetIndex+1, cmdErr, string(output))
		}

		spriteSheets = append(spriteSheets, spriteName)
	}

	return spriteSheets, nil
}
