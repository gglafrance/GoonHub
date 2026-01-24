package jobs

import (
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
	videoID                uint
	videoPath              string
	maxFrameDimension      int
	maxFrameDimensionLarge int
	repo                   data.VideoRepository
	logger                 *zap.Logger
	status                 JobStatus
	error                  error
	cancelled              atomic.Bool
	result                 *MetadataResult
}

func NewMetadataJob(
	videoID uint,
	videoPath string,
	maxFrameDimension int,
	maxFrameDimensionLarge int,
	repo data.VideoRepository,
	logger *zap.Logger,
) *MetadataJob {
	return &MetadataJob{
		id:                     uuid.New().String(),
		videoID:                videoID,
		videoPath:              videoPath,
		maxFrameDimension:      maxFrameDimension,
		maxFrameDimensionLarge: maxFrameDimensionLarge,
		repo:                   repo,
		logger:                 logger,
		status:                 JobStatusPending,
	}
}

func (j *MetadataJob) GetID() string             { return j.id }
func (j *MetadataJob) GetVideoID() uint           { return j.videoID }
func (j *MetadataJob) GetPhase() string           { return "metadata" }
func (j *MetadataJob) GetStatus() JobStatus       { return j.status }
func (j *MetadataJob) GetError() error            { return j.error }
func (j *MetadataJob) GetResult() *MetadataResult { return j.result }
func (j *MetadataJob) GetVideoPath() string       { return j.videoPath }

func (j *MetadataJob) Cancel() {
	j.cancelled.Store(true)
}

func (j *MetadataJob) Execute() error {
	startTime := time.Now()
	j.status = JobStatusRunning

	j.logger.Info("Starting metadata extraction job",
		zap.String("job_id", j.id),
		zap.Uint("video_id", j.videoID),
		zap.String("video_path", j.videoPath),
	)

	if err := j.repo.UpdateProcessingStatus(j.videoID, "processing", ""); err != nil {
		j.logger.Error("Failed to update processing status",
			zap.Uint("video_id", j.videoID),
			zap.Error(err),
		)
		j.error = err
		j.status = JobStatusFailed
		return err
	}

	if j.cancelled.Load() {
		j.status = JobStatusCancelled
		j.repo.UpdateProcessingStatus(j.videoID, string(JobStatusCancelled), "job was cancelled")
		return fmt.Errorf("job cancelled")
	}

	metadata, err := ffmpeg.GetMetadata(j.videoPath)
	if err != nil {
		j.logger.Error("Failed to get video metadata",
			zap.Uint("video_id", j.videoID),
			zap.Error(err),
		)
		j.handleError(fmt.Errorf("metadata extraction failed: %w", err))
		return err
	}

	tileWidth, tileHeight := ffmpeg.CalculateTileDimensions(metadata.Width, metadata.Height, j.maxFrameDimension)
	tileWidthLarge, tileHeightLarge := ffmpeg.CalculateTileDimensions(metadata.Width, metadata.Height, j.maxFrameDimensionLarge)

	duration := int(metadata.Duration)
	if err := j.repo.UpdateBasicMetadata(j.videoID, duration, metadata.Width, metadata.Height, metadata.FrameRate, metadata.BitRate, metadata.VideoCodec, metadata.AudioCodec); err != nil {
		j.logger.Error("Failed to update basic metadata",
			zap.Uint("video_id", j.videoID),
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
		zap.Uint("video_id", j.videoID),
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
	j.repo.UpdateProcessingStatus(j.videoID, string(JobStatusFailed), err.Error())
}
