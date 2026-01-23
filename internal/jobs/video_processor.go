package jobs

import (
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

type ProcessVideoJob struct {
	id                string
	videoID           uint
	videoPath         string
	spriteDir         string
	vttDir            string
	thumbnailDir      string
	frameInterval     int
	maxFrameDimension int
	frameQuality      int
	gridCols          int
	gridRows          int
	thumbnailSeek     string
	repo              data.VideoRepository
	logger            *zap.Logger
	status            JobStatus
	error             error
	cancelled         atomic.Bool
}

func NewProcessVideoJob(
	videoID uint,
	videoPath string,
	spriteDir string,
	vttDir string,
	thumbnailDir string,
	frameInterval int,
	maxFrameDimension int,
	frameQuality int,
	gridCols int,
	gridRows int,
	thumbnailSeek string,
	repo data.VideoRepository,
	logger *zap.Logger,
) *ProcessVideoJob {
	return &ProcessVideoJob{
		id:                uuid.New().String(),
		videoID:           videoID,
		videoPath:         videoPath,
		spriteDir:         spriteDir,
		vttDir:            vttDir,
		thumbnailDir:      thumbnailDir,
		frameInterval:     frameInterval,
		maxFrameDimension: maxFrameDimension,
		frameQuality:      frameQuality,
		gridCols:          gridCols,
		gridRows:          gridRows,
		thumbnailSeek:     thumbnailSeek,
		repo:              repo,
		logger:            logger,
		status:            JobStatusPending,
	}
}

func (j *ProcessVideoJob) GetID() string {
	return j.id
}

func (j *ProcessVideoJob) GetStatus() JobStatus {
	return j.status
}

func (j *ProcessVideoJob) GetError() error {
	return j.error
}

func (j *ProcessVideoJob) Cancel() {
	j.cancelled.Store(true)
}

func (j *ProcessVideoJob) Execute() error {
	startTime := time.Now()
	j.status = JobStatusRunning

	j.logger.Info("Starting video processing job",
		zap.String("job_id", j.id),
		zap.Uint("video_id", j.videoID),
		zap.String("video_path", j.videoPath),
		zap.Int("frame_interval", j.frameInterval),
		zap.Int("max_frame_dimension", j.maxFrameDimension),
		zap.Int("grid_cols", j.gridCols),
		zap.Int("grid_rows", j.gridRows),
	)

	if err := j.repo.UpdateProcessingStatus(j.videoID, string(JobStatusRunning), ""); err != nil {
		j.logger.Error("Failed to update processing status to running",
			zap.Uint("video_id", j.videoID),
			zap.Error(err),
		)
		j.error = err
		j.status = JobStatusFailed
		return err
	}

	if err := j.checkCancelled(); err != nil {
		return err
	}

	stepStart := time.Now()
	metadata, err := ffmpeg.GetMetadata(j.videoPath)
	if err != nil {
		j.logger.Error("Failed to get video metadata",
			zap.Uint("video_id", j.videoID),
			zap.String("path", j.videoPath),
			zap.Duration("step_duration", time.Since(stepStart)),
			zap.Error(err),
		)
		j.handleError(fmt.Errorf("metadata extraction failed: %w", err))
		return err
	}

	tileWidth, tileHeight := ffmpeg.CalculateTileDimensions(metadata.Width, metadata.Height, j.maxFrameDimension)

	j.logger.Info("Video metadata extracted",
		zap.Uint("video_id", j.videoID),
		zap.Int("duration_seconds", int(metadata.Duration)),
		zap.Int("width", metadata.Width),
		zap.Int("height", metadata.Height),
		zap.Float64("aspect_ratio", float64(metadata.Width)/float64(metadata.Height)),
		zap.Int("tile_width", tileWidth),
		zap.Int("tile_height", tileHeight),
		zap.Duration("step_duration", time.Since(stepStart)),
	)

	if err := j.checkCancelled(); err != nil {
		return err
	}

	stepStart = time.Now()
	if err := os.MkdirAll(j.thumbnailDir, 0755); err != nil {
		j.logger.Error("Failed to create thumbnail directory",
			zap.String("dir", j.thumbnailDir),
			zap.Error(err),
		)
		j.handleError(err)
		return err
	}

	thumbnailPath := filepath.Join(j.thumbnailDir, fmt.Sprintf("%d_thumb.webp", j.videoID))
	thumbnailSeek := fmt.Sprintf("%d", int(metadata.Duration/2))

	j.logger.Info("Extracting thumbnail at middle timecode",
		zap.Uint("video_id", j.videoID),
		zap.String("seek_position", thumbnailSeek),
		zap.String("output_path", thumbnailPath),
	)

	if err := ffmpeg.ExtractThumbnail(j.videoPath, thumbnailPath, thumbnailSeek, tileWidth, tileHeight, j.frameQuality); err != nil {
		j.logger.Error("Failed to extract thumbnail",
			zap.Uint("video_id", j.videoID),
			zap.Duration("step_duration", time.Since(stepStart)),
			zap.Error(err),
		)
		j.handleError(fmt.Errorf("thumbnail extraction failed: %w", err))
		return err
	}

	j.logger.Info("Thumbnail extracted successfully",
		zap.Uint("video_id", j.videoID),
		zap.Duration("step_duration", time.Since(stepStart)),
	)

	if err := j.checkCancelled(); err != nil {
		return err
	}

	stepStart = time.Now()
	if err := os.MkdirAll(j.spriteDir, 0755); err != nil {
		j.logger.Error("Failed to create sprite directory",
			zap.String("dir", j.spriteDir),
			zap.Error(err),
		)
		j.handleError(err)
		return err
	}

	expectedFrameCount := int(metadata.Duration) / j.frameInterval
	if int(metadata.Duration)%j.frameInterval != 0 {
		expectedFrameCount++
	}

	j.logger.Info("Starting sprite sheet generation",
		zap.Uint("video_id", j.videoID),
		zap.String("output_dir", j.spriteDir),
		zap.Int("expected_frame_count", expectedFrameCount),
		zap.Int("interval_seconds", j.frameInterval),
		zap.Int("grid_cols", j.gridCols),
		zap.Int("grid_rows", j.gridRows),
	)

	spriteSheets, err := ffmpeg.ExtractSpriteSheets(
		j.videoPath,
		j.spriteDir,
		int(j.videoID),
		tileWidth,
		tileHeight,
		j.gridCols,
		j.gridRows,
		j.frameInterval,
		j.frameQuality,
	)
	if err != nil {
		j.logger.Error("Failed to generate sprite sheets",
			zap.Uint("video_id", j.videoID),
			zap.Duration("step_duration", time.Since(stepStart)),
			zap.Error(err),
		)
		j.handleError(fmt.Errorf("sprite sheet generation failed: %w", err))
		return err
	}

	j.logger.Info("Sprite sheets generated successfully",
		zap.Uint("video_id", j.videoID),
		zap.Int("sprite_sheet_count", len(spriteSheets)),
		zap.Duration("step_duration", time.Since(stepStart)),
	)

	stepStart = time.Now()
	if err := os.MkdirAll(j.vttDir, 0755); err != nil {
		j.logger.Error("Failed to create VTT directory",
			zap.String("dir", j.vttDir),
			zap.Error(err),
		)
		j.handleError(err)
		return err
	}

	vttPath := filepath.Join(j.vttDir, fmt.Sprintf("%d_thumbnails.vtt", j.videoID))
	j.logger.Info("Generating VTT file",
		zap.Uint("video_id", j.videoID),
		zap.String("output_path", vttPath),
	)

	duration := int(metadata.Duration)
	if err := ffmpeg.GenerateVttFile(
		vttPath,
		spriteSheets,
		duration,
		j.frameInterval,
		j.gridCols,
		j.gridRows,
		tileWidth,
		tileHeight,
	); err != nil {
		j.logger.Error("Failed to generate VTT file",
			zap.Uint("video_id", j.videoID),
			zap.Duration("step_duration", time.Since(stepStart)),
			zap.Error(err),
		)
		j.handleError(fmt.Errorf("VTT generation failed: %w", err))
		return err
	}

	j.logger.Info("VTT file generated successfully",
		zap.Uint("video_id", j.videoID),
		zap.Duration("step_duration", time.Since(stepStart)),
	)

	stepStart = time.Now()
	spriteSheetPath := ""
	if len(spriteSheets) > 0 {
		spriteSheetPath = filepath.Join(j.spriteDir, spriteSheets[0])
	}

	if err := j.repo.UpdateMetadata(
		j.videoID,
		duration,
		metadata.Width,
		metadata.Height,
		thumbnailPath,
		spriteSheetPath,
		vttPath,
		len(spriteSheets),
		tileWidth,
		tileHeight,
	); err != nil {
		j.logger.Error("Failed to update video metadata",
			zap.Uint("video_id", j.videoID),
			zap.Duration("step_duration", time.Since(stepStart)),
			zap.Error(err),
		)
		j.handleError(err)
		return err
	}

	totalDuration := time.Since(startTime)
	processingRate := float64(metadata.Duration) / totalDuration.Seconds()

	j.status = JobStatusCompleted
	j.logger.Info("Video processing completed successfully",
		zap.String("job_id", j.id),
		zap.Uint("video_id", j.videoID),
		zap.Int("sprite_sheet_count", len(spriteSheets)),
		zap.Int("duration", duration),
		zap.Duration("total_duration", totalDuration),
		zap.Float64("processing_rate_secs_per_sec", processingRate),
	)

	return nil
}

func (j *ProcessVideoJob) checkCancelled() error {
	if j.cancelled.Load() {
		j.status = JobStatusCancelled
		j.repo.UpdateProcessingStatus(j.videoID, string(JobStatusCancelled), "job was cancelled")
		j.logger.Warn("Video processing job cancelled",
			zap.String("job_id", j.id),
			zap.Uint("video_id", j.videoID),
		)
		return fmt.Errorf("job cancelled")
	}
	return nil
}

func (j *ProcessVideoJob) handleError(err error) {
	j.error = err
	j.status = JobStatusFailed
	j.repo.UpdateProcessingStatus(j.videoID, string(JobStatusFailed), err.Error())
	j.logger.Error("Video processing job failed",
		zap.String("job_id", j.id),
		zap.Uint("video_id", j.videoID),
		zap.Error(err),
	)
}
