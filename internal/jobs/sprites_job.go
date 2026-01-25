package jobs

import (
	"context"
	"fmt"
	"goonhub/internal/data"
	"goonhub/pkg/ffmpeg"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type SpritesResult struct {
	SpriteSheetPath  string
	VttPath          string
	SpriteSheetCount int
}

type SpritesJob struct {
	id            string
	videoID       uint
	videoPath     string
	spriteDir     string
	vttDir        string
	tileWidth     int
	tileHeight    int
	duration      int
	frameInterval int
	frameQuality  int
	gridCols      int
	gridRows      int
	concurrency   int
	repo          data.VideoRepository
	logger        *zap.Logger
	status        JobStatus
	error         error
	cancelled     atomic.Bool
	result        *SpritesResult
	ctx           context.Context
	cancelFn      context.CancelFunc
}

func NewSpritesJob(
	videoID uint,
	videoPath string,
	spriteDir string,
	vttDir string,
	tileWidth int,
	tileHeight int,
	duration int,
	frameInterval int,
	frameQuality int,
	gridCols int,
	gridRows int,
	concurrency int,
	repo data.VideoRepository,
	logger *zap.Logger,
) *SpritesJob {
	return &SpritesJob{
		id:            uuid.New().String(),
		videoID:       videoID,
		videoPath:     videoPath,
		spriteDir:     spriteDir,
		vttDir:        vttDir,
		tileWidth:     tileWidth,
		tileHeight:    tileHeight,
		duration:      duration,
		frameInterval: frameInterval,
		frameQuality:  frameQuality,
		gridCols:      gridCols,
		gridRows:      gridRows,
		concurrency:   concurrency,
		repo:          repo,
		logger:        logger,
		status:        JobStatusPending,
	}
}

func (j *SpritesJob) GetID() string      { return j.id }
func (j *SpritesJob) GetVideoID() uint    { return j.videoID }
func (j *SpritesJob) GetPhase() string    { return "sprites" }
func (j *SpritesJob) GetStatus() JobStatus { return j.status }
func (j *SpritesJob) GetError() error     { return j.error }
func (j *SpritesJob) GetResult() *SpritesResult { return j.result }

func (j *SpritesJob) Cancel() {
	j.cancelled.Store(true)
	if j.cancelFn != nil {
		j.cancelFn()
	}
}

func (j *SpritesJob) Execute() error {
	return j.ExecuteWithContext(context.Background())
}

func (j *SpritesJob) ExecuteWithContext(ctx context.Context) error {
	// Create a cancellable context for this execution
	j.ctx, j.cancelFn = context.WithCancel(ctx)
	defer j.cancelFn()

	startTime := time.Now()
	j.status = JobStatusRunning

	j.logger.Info("Starting sprite sheet generation job",
		zap.String("job_id", j.id),
		zap.Uint("video_id", j.videoID),
		zap.Int("tile_width", j.tileWidth),
		zap.Int("tile_height", j.tileHeight),
		zap.Int("frame_interval", j.frameInterval),
		zap.Int("grid_cols", j.gridCols),
		zap.Int("grid_rows", j.gridRows),
	)

	// Check for cancellation
	if j.cancelled.Load() || j.ctx.Err() != nil {
		j.status = JobStatusCancelled
		return fmt.Errorf("job cancelled")
	}

	if err := os.MkdirAll(j.spriteDir, 0755); err != nil {
		j.logger.Error("Failed to create sprite directory",
			zap.String("dir", j.spriteDir),
			zap.Error(err),
		)
		j.handleError(fmt.Errorf("failed to create sprite directory: %w", err))
		return err
	}

	spriteSheets, err := ffmpeg.ExtractSpriteSheetsWithContext(
		j.ctx,
		j.videoPath,
		j.spriteDir,
		int(j.videoID),
		j.tileWidth,
		j.tileHeight,
		j.gridCols,
		j.gridRows,
		j.frameInterval,
		j.frameQuality,
		j.concurrency,
	)
	if err != nil {
		if j.ctx.Err() == context.DeadlineExceeded {
			j.status = JobStatusTimedOut
			j.error = fmt.Errorf("sprite sheet generation timed out")
			j.repo.UpdateProcessingStatus(j.videoID, string(JobStatusTimedOut), "sprite sheet generation timed out")
			return j.error
		}
		if j.ctx.Err() == context.Canceled || j.cancelled.Load() {
			j.status = JobStatusCancelled
			return fmt.Errorf("job cancelled")
		}
		j.logger.Error("Failed to generate sprite sheets",
			zap.Uint("video_id", j.videoID),
			zap.Error(err),
		)
		j.handleError(fmt.Errorf("sprite sheet generation failed: %w", err))
		return err
	}

	j.logger.Info("Sprite sheets generated",
		zap.Uint("video_id", j.videoID),
		zap.Int("count", len(spriteSheets)),
	)

	if err := os.MkdirAll(j.vttDir, 0755); err != nil {
		j.logger.Error("Failed to create VTT directory",
			zap.String("dir", j.vttDir),
			zap.Error(err),
		)
		j.handleError(fmt.Errorf("failed to create VTT directory: %w", err))
		return err
	}

	vttPath := filepath.Join(j.vttDir, fmt.Sprintf("%d_thumbnails.vtt", j.videoID))
	if err := ffmpeg.GenerateVttFile(
		vttPath,
		spriteSheets,
		j.duration,
		j.frameInterval,
		j.gridCols,
		j.gridRows,
		j.tileWidth,
		j.tileHeight,
	); err != nil {
		j.logger.Error("Failed to generate VTT file",
			zap.Uint("video_id", j.videoID),
			zap.Error(err),
		)
		j.handleError(fmt.Errorf("VTT generation failed: %w", err))
		return err
	}

	spriteSheetPath := ""
	if len(spriteSheets) > 0 {
		spriteSheetPath = filepath.Join(j.spriteDir, spriteSheets[0])
	}

	if err := j.repo.UpdateSprites(j.videoID, spriteSheetPath, vttPath, len(spriteSheets)); err != nil {
		j.logger.Error("Failed to update sprites in database",
			zap.Uint("video_id", j.videoID),
			zap.Error(err),
		)
		j.handleError(fmt.Errorf("failed to update sprites: %w", err))
		return err
	}

	j.result = &SpritesResult{
		SpriteSheetPath:  spriteSheetPath,
		VttPath:          vttPath,
		SpriteSheetCount: len(spriteSheets),
	}

	j.status = JobStatusCompleted
	j.logger.Info("Sprite sheet generation completed",
		zap.String("job_id", j.id),
		zap.Uint("video_id", j.videoID),
		zap.Int("sprite_sheet_count", len(spriteSheets)),
		zap.String("vtt_path", vttPath),
		zap.Duration("elapsed", time.Since(startTime)),
	)

	return nil
}

func (j *SpritesJob) handleError(err error) {
	j.error = err
	j.status = JobStatusFailed
	j.repo.UpdateProcessingStatus(j.videoID, string(JobStatusFailed), err.Error())
}
