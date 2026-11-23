package compute

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"

	"goonhub/pkg/ffmpeg"
	"goonhub/pkg/workerproto"
)

// Sprites extracts the sprite sheets (<scene>_sheet_NNN.webp) and writes the VTT
// (<scene>_thumbnails.vtt) into req.OutDir. Progress: 0-80 while extracting the frames,
// 80-100 while tiling. The frame interval is shortened for short scenes
// (workerproto.EffectiveSpriteInterval); the result reports the value used.
func Sprites(ctx context.Context, env Env, req Request, p workerproto.SpritesParams) (*workerproto.SpritesResult, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	interval := workerproto.EffectiveSpriteInterval(req.Video.DurationSec, p.FrameInterval, p.GridCols)

	if err := os.MkdirAll(req.OutDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create sprite directory: %w", err)
	}

	sheets, err := extractSpriteSheets(ctx, env, req, p, interval)
	if err != nil {
		return nil, fmt.Errorf("sprite sheet generation failed: %w", err)
	}

	vtt := workerproto.VTTName(p.SceneID)
	if err := ffmpeg.GenerateVttFile(filepath.Join(req.OutDir, vtt), sheets, req.Video.DurationSec, interval,
		p.GridCols, p.GridRows, p.TileWidth, p.TileHeight); err != nil {
		return nil, fmt.Errorf("VTT generation failed: %w", err)
	}

	if sheets == nil {
		sheets = []string{}
	}
	return &workerproto.SpritesResult{
		SpriteSheets:  sheets,
		VTT:           vtt,
		SheetCount:    len(sheets),
		FrameInterval: interval,
	}, nil
}

// extractSpriteSheets extracts one frame per interval in parallel (input seeking: -ss before
// -i jumps to the nearest keyframe and decodes a few frames, much faster than an fps filter
// decoding everything), then tiles them into sheets.
func extractSpriteSheets(ctx context.Context, env Env, req Request, p workerproto.SpritesParams, interval int) ([]string, error) {
	metadata, err := ffmpeg.GetMetadataWithBinary(ctx, env.ffprobe(), req.Input)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("failed to get video metadata: %w", err)
	}

	duration := int(metadata.Duration)
	if duration < interval {
		return []string{}, nil
	}

	totalFrames := duration / interval
	if duration%interval != 0 {
		totalFrames++
	}
	framesPerSheet := p.GridCols * p.GridRows
	totalSheets := (totalFrames + framesPerSheet - 1) / framesPerSheet

	tmpDir, err := os.MkdirTemp(env.TempDir, "goonhub-sprites-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	concurrency := p.Concurrency
	if concurrency <= 0 {
		concurrency = runtime.NumCPU()
		if concurrency < 4 {
			concurrency = 4
		}
	}

	if acc := env.Accel.For(StepSpriteFrame); !acc.IsSoftware() && acc.MaxParallel > 0 && concurrency > acc.MaxParallel {
		concurrency = acc.MaxParallel
	}

	// Phase 1: extract every frame (0-80%)
	semaphore := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	errChan := make(chan error, totalFrames)
	var completedFrames int64

	for i := 0; i < totalFrames; i++ {
		wg.Add(1)
		go func(frameIndex int) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			case semaphore <- struct{}{}:
			}
			defer func() { <-semaphore }()

			if ctx.Err() != nil {
				errChan <- ctx.Err()
				return
			}

			ts := frameIndex * interval
			framePath := filepath.Join(tmpDir, fmt.Sprintf("frame_%04d.webp", frameIndex))
			err := runFFmpeg(ctx, env, StepSpriteFrame, 1, fmt.Sprintf("ffmpeg failed extracting frame at %ds", ts),
				func(acc StepAccel) []string {
					return spriteFrameArgs(acc, req.Input, framePath, ts, p.TileWidth, p.TileHeight, p.FrameQuality)
				})
			if err != nil {
				errChan <- err
				return
			}

			completed := atomic.AddInt64(&completedFrames, 1)
			req.report(int(float64(completed) / float64(totalFrames) * 80))
		}(i)
	}

	wg.Wait()
	close(errChan)

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err := <-errChan; err != nil {
		return nil, err
	}

	// Phase 2: tile the frames into sheets (80-100%)
	var sheets []string
	for sheetIndex := 0; sheetIndex < totalSheets; sheetIndex++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		spriteName := workerproto.SpriteSheetName(p.SceneID, sheetIndex+1)
		spritePath := filepath.Join(req.OutDir, spriteName)

		startFrame := sheetIndex * framesPerSheet
		endFrame := startFrame + framesPerSheet
		if endFrame > totalFrames {
			endFrame = totalFrames
		}

		// A directory of sequentially numbered links to this sheet's frames
		sheetDir, err := os.MkdirTemp(env.TempDir, "goonhub-sheet-*")
		if err != nil {
			return nil, fmt.Errorf("failed to create sheet temp directory: %w", err)
		}
		for i := startFrame; i < endFrame; i++ {
			src := filepath.Join(tmpDir, fmt.Sprintf("frame_%04d.webp", i))
			dst := filepath.Join(sheetDir, fmt.Sprintf("%04d.webp", i-startFrame))
			if err := linkFile(src, dst); err != nil {
				os.RemoveAll(sheetDir)
				return nil, fmt.Errorf("failed to create symlink: %w", err)
			}
		}

		err = runOnce(ctx, env, fmt.Sprintf("ffmpeg failed tiling sprite sheet %d", sheetIndex+1),
			spriteTileArgs(filepath.Join(sheetDir, "%04d.webp"), spritePath, p.GridCols, p.GridRows, p.FrameQuality))
		os.RemoveAll(sheetDir)
		if err != nil {
			return nil, err
		}

		sheets = append(sheets, spriteName)
		req.report(80 + int(float64(sheetIndex+1)/float64(totalSheets)*20))
	}

	return sheets, nil
}

// SpriteFrameArgs returns the ffmpeg arguments extracting one sprite frame at ts seconds
// with the step acceleration acc (the zero StepAccel gives the server's software command).
// Exported for benchmarks and hardware probes (pkg/compute/hwaccel).
func SpriteFrameArgs(acc StepAccel, input, output string, ts, width, height, quality int) []string {
	return spriteFrameArgs(acc, input, output, ts, width, height, quality)
}

// spriteFrameArgs extracts one scaled frame at ts seconds.
func spriteFrameArgs(acc StepAccel, input, output string, ts, width, height, quality int) []string {
	args := ffmpeg.GetDefaultArgs()
	args = append(args, acc.globalArgs()...)
	args = append(args, acc.inputArgs()...)
	return append(args,
		"-ss", strconv.Itoa(ts),
		"-i", input,
		"-threads", "1",
		"-vframes", "1",
		"-vf", acc.scale(fmt.Sprintf("%d:%d", width, height), ""),
		"-q:v", strconv.Itoa(quality),
		"-y",
		output,
	)
}

// spriteTileArgs tiles numbered frames into one sheet.
func spriteTileArgs(pattern, output string, cols, rows, quality int) []string {
	args := ffmpeg.GetDefaultArgs()
	return append(args,
		"-framerate", "1",
		"-i", pattern,
		"-vf", fmt.Sprintf("tile=%dx%d", cols, rows),
		"-q:v", strconv.Itoa(quality),
		"-frames:v", "1",
		"-y",
		output,
	)
}

// linkFile makes dst point to src: a symlink (what the server always did), else a hard link
// (Windows without the symlink privilege), else a copy.
func linkFile(src, dst string) error {
	if err := os.Symlink(src, dst); err == nil {
		return nil
	}
	if err := os.Link(src, dst); err == nil {
		return nil
	}
	return copyFile(src, dst)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
