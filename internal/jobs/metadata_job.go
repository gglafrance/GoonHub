package jobs

import (
	"context"
	"fmt"
	"goonhub/internal/data"
	"goonhub/pkg/ffmpeg"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type MetadataResult struct {
	Duration        int
	Width           int
	Height          int
	TileWidth       int
	TileHeight      int
	TileWidthLarge  int
	TileHeightLarge int
	FrameRate       float64
	BitRate         int64
	VideoCodec      string
	AudioCodec      string
}

type MetadataJob struct {
	id                     string
	sceneID                uint
	scenePath              string
	maxFrameDimension      int
	maxFrameDimensionLarge int
	repo                   data.SceneRepository
	logger                 *zap.Logger
	status                 JobStatus
	error                  error
	cancelled              atomic.Bool
	result                 *MetadataResult
	ctx                    context.Context
	cancelFn               context.CancelFunc
}

func NewMetadataJob(
	sceneID uint,
	scenePath string,
	maxFrameDimension int,
	maxFrameDimensionLarge int,
	repo data.SceneRepository,
	logger *zap.Logger,
) *MetadataJob {
	return &MetadataJob{
		id:                     uuid.New().String(),
		sceneID:                sceneID,
		scenePath:              scenePath,
		maxFrameDimension:      maxFrameDimension,
		maxFrameDimensionLarge: maxFrameDimensionLarge,
		repo:                   repo,
		logger:                 logger,
		status:                 JobStatusPending,
	}
}

func (j *MetadataJob) GetID() string             { return j.id }
func (j *MetadataJob) GetSceneID() uint           { return j.sceneID }
func (j *MetadataJob) GetPhase() string           { return "metadata" }
func (j *MetadataJob) GetStatus() JobStatus       { return j.status }
func (j *MetadataJob) GetError() error            { return j.error }
func (j *MetadataJob) GetResult() *MetadataResult { return j.result }
func (j *MetadataJob) GetScenePath() string       { return j.scenePath }

func (j *MetadataJob) Cancel() {
	j.cancelled.Store(true)
	if j.cancelFn != nil {
		j.cancelFn()
	}
}

func (j *MetadataJob) Execute() error {
	return j.ExecuteWithContext(context.Background())
}

func (j *MetadataJob) ExecuteWithContext(ctx context.Context) error {
	// Create a cancellable context for this execution
	j.ctx, j.cancelFn = context.WithCancel(ctx)
	defer j.cancelFn()

	startTime := time.Now()
	j.status = JobStatusRunning

	j.logger.Info("Starting metadata extraction job",
		zap.String("job_id", j.id),
		zap.Uint("scene_id", j.sceneID),
		zap.String("scene_path", j.scenePath),
	)

	if err := j.repo.UpdateProcessingStatus(j.sceneID, "processing", ""); err != nil {
		j.logger.Error("Failed to update processing status",
			zap.Uint("scene_id", j.sceneID),
			zap.Error(err),
		)
		j.error = err
		j.status = JobStatusFailed
		return err
	}

	// Check for cancellation
	if j.cancelled.Load() || j.ctx.Err() != nil {
		j.status = JobStatusCancelled
		j.repo.UpdateProcessingStatus(j.sceneID, string(JobStatusCancelled), "job was cancelled")
		return fmt.Errorf("job cancelled")
	}

	metadata, err := ffmpeg.GetMetadataWithContext(j.ctx, j.scenePath)
	if err != nil {
		// Check if this was a timeout or cancellation
		if j.ctx.Err() == context.DeadlineExceeded {
			j.status = JobStatusTimedOut
			j.error = fmt.Errorf("metadata extraction timed out")
			j.repo.UpdateProcessingStatus(j.sceneID, string(JobStatusTimedOut), "metadata extraction timed out")
			return j.error
		}
		if j.ctx.Err() == context.Canceled || j.cancelled.Load() {
			j.status = JobStatusCancelled
			j.repo.UpdateProcessingStatus(j.sceneID, string(JobStatusCancelled), "job was cancelled")
			return fmt.Errorf("job cancelled")
		}
		j.logger.Error("Failed to get scene metadata",
			zap.Uint("scene_id", j.sceneID),
			zap.Error(err),
		)
		j.handleError(fmt.Errorf("metadata extraction failed: %w", err))
		return err
	}

	tileWidth, tileHeight := ffmpeg.CalculateTileDimensions(metadata.Width, metadata.Height, j.maxFrameDimension)
	tileWidthLarge, tileHeightLarge := ffmpeg.CalculateTileDimensions(metadata.Width, metadata.Height, j.maxFrameDimensionLarge)

	duration := int(metadata.Duration)
	if err := j.repo.UpdateBasicMetadata(j.sceneID, duration, metadata.Width, metadata.Height, metadata.FrameRate, metadata.BitRate, metadata.VideoCodec, metadata.AudioCodec); err != nil {
		j.logger.Error("Failed to update basic metadata",
			zap.Uint("scene_id", j.sceneID),
			zap.Error(err),
		)
		j.handleError(fmt.Errorf("failed to update metadata: %w", err))
		return err
	}

	j.result = &MetadataResult{
		Duration:        duration,
		Width:           metadata.Width,
		Height:          metadata.Height,
		TileWidth:       tileWidth,
		TileHeight:      tileHeight,
		TileWidthLarge:  tileWidthLarge,
		TileHeightLarge: tileHeightLarge,
		FrameRate:       metadata.FrameRate,
		BitRate:         metadata.BitRate,
		VideoCodec:      metadata.VideoCodec,
		AudioCodec:      metadata.AudioCodec,
	}

	j.status = JobStatusCompleted
	j.logger.Info("Metadata extraction completed",
		zap.String("job_id", j.id),
		zap.Uint("scene_id", j.sceneID),
		zap.Int("duration", duration),
		zap.Int("width", metadata.Width),
		zap.Int("height", metadata.Height),
		zap.Int("tile_width", tileWidth),
		zap.Int("tile_height", tileHeight),
		zap.Duration("elapsed", time.Since(startTime)),
	)

	return nil
}

func (j *MetadataJob) handleError(err error) {
	j.error = err
	j.status = JobStatusFailed
	j.repo.UpdateProcessingStatus(j.sceneID, string(JobStatusFailed), err.Error())
}
