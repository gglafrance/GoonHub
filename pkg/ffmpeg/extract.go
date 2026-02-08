package ffmpeg

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

func ExtractThumbnail(videoPath, outputPath, seekPosition string, width, height, quality int) error {
	return ExtractThumbnailWithContext(context.Background(), videoPath, outputPath, seekPosition, width, height, quality)
}

func ExtractThumbnailWithContext(ctx context.Context, videoPath, outputPath, seekPosition string, width, height, quality int) error {
	args := GetDefaultArgs()
	args = append(args, []string{
		"-ss", seekPosition,
		"-i", videoPath,
		"-vframes", "1",
		"-c:v", "libwebp",
		"-vf", fmt.Sprintf("scale=%d:%d", width, height),
		"-q:v", strconv.Itoa(quality),
		"-y",
		outputPath,
	}...)

	cmd := exec.CommandContext(ctx, FFMpegPath(), args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("ffmpeg failed: %w, output: %s", err, string(output))
	}

	return nil
}

// ExtractAnimatedThumbnailWithContext extracts a short MP4 clip from a video at the given seek position.
// The clip is encoded with libx264 at the given width (height auto-calculated to preserve aspect ratio),
// with fast encoding settings optimized for small preview thumbnails.
func ExtractAnimatedThumbnailWithContext(ctx context.Context, videoPath, outputPath, seekPosition string, duration, width, crf int) error {
	args := GetDefaultArgs()
	args = append(args,
		"-ss", seekPosition,
		"-i", videoPath,
		"-t", strconv.Itoa(duration),
		"-c:v", "libx264",
		"-vf", fmt.Sprintf("scale=%d:-2:flags=bilinear", width),
		"-pix_fmt", "yuv420p",
		"-preset", "veryfast",
		"-crf", strconv.Itoa(crf),
		"-movflags", "+faststart",
		"-map_metadata", "-1",
		"-threads", "2",
		"-an",
		"-y",
		outputPath,
	)

	cmd := exec.CommandContext(ctx, FFMpegPath(), args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("ffmpeg animated thumbnail failed: %w, output: %s", err, string(output))
	}

	return nil
}

// ExtractScenePreviewWithContext generates a scene preview video by sampling multiple segments
// throughout the video and concatenating them into a single clip. For short videos where the
// total content is less than segments * segmentDuration, it encodes the entire video at reduced resolution.
func ExtractScenePreviewWithContext(ctx context.Context, videoPath, outputPath string,
	duration int, segments int, segmentDuration float64, width, crf int) error {

	totalNeeded := float64(segments) * segmentDuration

	if float64(duration) < totalNeeded {
		// Short video mode: encode entire video at reduced resolution
		args := GetDefaultArgs()
		args = append(args,
			"-i", videoPath,
			"-c:v", "libx264",
			"-vf", fmt.Sprintf("scale=%d:-2:flags=bilinear", width),
			"-pix_fmt", "yuv420p",
			"-preset", "veryfast",
			"-crf", strconv.Itoa(crf),
			"-movflags", "+faststart",
			"-map_metadata", "-1",
			"-threads", "4",
			"-an",
			"-y",
			outputPath,
		)

		cmd := exec.CommandContext(ctx, FFMpegPath(), args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("ffmpeg scene preview (short mode) failed: %w, output: %s", err, string(output))
		}
		return nil
	}

	// Normal mode: sample N segments throughout the video
	interval := float64(duration) / float64(segments)

	args := GetDefaultArgs()

	// Build multi-input args: -ss T1 -i <video> -ss T2 -i <video> ...
	for i := 0; i < segments; i++ {
		seekPos := interval*float64(i) + interval/2
		args = append(args, "-ss", fmt.Sprintf("%.2f", seekPos), "-i", videoPath)
	}

	// Build filter_complex
	var filterParts []string
	var concatInputs []string
	for i := 0; i < segments; i++ {
		label := fmt.Sprintf("v%d", i)
		filterParts = append(filterParts,
			fmt.Sprintf("[%d:v]trim=0:%.2f,setpts=PTS-STARTPTS,scale=%d:-2:flags=bilinear,format=yuv420p[%s]",
				i, segmentDuration, width, label))
		concatInputs = append(concatInputs, fmt.Sprintf("[%s]", label))
	}
	filterParts = append(filterParts,
		fmt.Sprintf("%sconcat=n=%d:v=1:a=0[out]", strings.Join(concatInputs, ""), segments))

	filterComplex := strings.Join(filterParts, ";")

	args = append(args,
		"-filter_complex", filterComplex,
		"-map", "[out]",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", strconv.Itoa(crf),
		"-movflags", "+faststart",
		"-map_metadata", "-1",
		"-threads", "4",
		"-an",
		"-y",
		outputPath,
	)

	cmd := exec.CommandContext(ctx, FFMpegPath(), args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("ffmpeg scene preview failed: %w, output: %s", err, string(output))
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
	return ExtractSpriteSheetsWithContext(context.Background(), videoPath, outputDir, videoID, width, height, gridCols, gridRows, interval, quality, concurrency)
}

func ExtractSpriteSheetsWithContext(ctx context.Context, videoPath, outputDir string, videoID int, width, height, gridCols, gridRows, interval, quality, concurrency int) ([]string, error) {
	return ExtractSpriteSheetsWithProgress(ctx, videoPath, outputDir, videoID, width, height, gridCols, gridRows, interval, quality, concurrency, nil)
}

// ExtractSpriteSheetsWithProgress extracts sprite sheets with optional progress reporting.
// The progress callback receives progress values from 0-100.
func ExtractSpriteSheetsWithProgress(ctx context.Context, videoPath, outputDir string, videoID int, width, height, gridCols, gridRows, interval, quality, concurrency int, progressCallback func(progress int)) ([]string, error) {
	metadata, err := GetMetadataWithContext(ctx, videoPath)
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

	// Atomic counter for tracking completed frames
	var completedFrames int64

	for i := 0; i < totalFrames; i++ {
		wg.Add(1)
		go func(frameIndex int) {
			defer wg.Done()

			// Check for context cancellation before acquiring semaphore
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			case semaphore <- struct{}{}:
			}
			defer func() { <-semaphore }()

			// Check again after acquiring semaphore
			if ctx.Err() != nil {
				errChan <- ctx.Err()
				return
			}

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

			cmd := exec.CommandContext(ctx, FFMpegPath(), args...)
			if output, err := cmd.CombinedOutput(); err != nil {
				if ctx.Err() != nil {
					errChan <- ctx.Err()
					return
				}
				errChan <- fmt.Errorf("ffmpeg failed extracting frame at %ds: %w, output: %s", ts, err, string(output))
				return
			}

			// Report progress (0-80% for frame extraction phase)
			completed := atomic.AddInt64(&completedFrames, 1)
			if progressCallback != nil {
				progress := int(float64(completed) / float64(totalFrames) * 80)
				progressCallback(progress)
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// Check for context cancellation first
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if err := <-errChan; err != nil {
		return nil, err
	}

	// Phase 2: Tile extracted frames into sprite sheets (80-100% progress)
	var spriteSheets []string
	for sheetIndex := 0; sheetIndex < totalSheets; sheetIndex++ {
		// Check for context cancellation between sheets
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

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

		cmd := exec.CommandContext(ctx, FFMpegPath(), args...)
		output, cmdErr := cmd.CombinedOutput()
		os.RemoveAll(sheetDir)
		if cmdErr != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("ffmpeg failed tiling sprite sheet %d: %w, output: %s", sheetIndex+1, cmdErr, string(output))
		}

		spriteSheets = append(spriteSheets, spriteName)

		// Report progress (80-100% for tiling phase)
		if progressCallback != nil {
			progress := 80 + int(float64(sheetIndex+1)/float64(totalSheets)*20)
			progressCallback(progress)
		}
	}

	return spriteSheets, nil
}
