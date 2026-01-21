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
	id             string
	videoID        uint
	videoPath      string
	frameOutputDir string
	thumbnailDir   string
	frameInterval  int
	frameWidth     int
	frameHeight    int
	frameQuality   int
	thumbnailSeek  string
	repo           data.VideoRepository
	logger         *zap.Logger
	status         JobStatus
	error          error
	cancelled      atomic.Bool
}

func NewProcessVideoJob(
	videoID uint,
	videoPath string,
	frameOutputDir string,
	thumbnailDir string,
	frameInterval int,
	frameWidth int,
	frameHeight int,
	frameQuality int,
	thumbnailSeek string,
	repo data.VideoRepository,
	logger *zap.Logger,
) *ProcessVideoJob {
	return &ProcessVideoJob{
		id:             uuid.New().String(),
		videoID:        videoID,
		videoPath:      videoPath,
		frameOutputDir: frameOutputDir,
		thumbnailDir:   thumbnailDir,
		frameInterval:  frameInterval,
		frameWidth:     frameWidth,
		frameHeight:    frameHeight,
		frameQuality:   frameQuality,
		thumbnailSeek:  thumbnailSeek,
		repo:           repo,
		logger:         logger,
		status:         JobStatusPending,
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
		zap.Int("frame_dimensions", j.frameWidth*j.frameHeight),
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

	j.logger.Info("Video metadata extracted",
		zap.Uint("video_id", j.videoID),
		zap.Int("duration_seconds", int(metadata.Duration)),
		zap.Int("width", metadata.Width),
		zap.Int("height", metadata.Height),
		zap.Float64("aspect_ratio", float64(metadata.Width)/float64(metadata.Height)),
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

	if err := ffmpeg.ExtractThumbnail(j.videoPath, thumbnailPath, thumbnailSeek, j.frameWidth, j.frameHeight, j.frameQuality); err != nil {
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
	videoFrameDir := filepath.Join(j.frameOutputDir, fmt.Sprintf("%d", j.videoID))
	if err := os.MkdirAll(videoFrameDir, 0755); err != nil {
		j.logger.Error("Failed to create frame directory",
			zap.String("dir", videoFrameDir),
			zap.Error(err),
		)
		j.handleError(err)
		return err
	}

	expectedFrameCount := int(metadata.Duration) / j.frameInterval
	if int(metadata.Duration)%j.frameInterval != 0 {
		expectedFrameCount++
	}

	j.logger.Info("Starting frame extraction",
		zap.Uint("video_id", j.videoID),
		zap.String("output_dir", videoFrameDir),
		zap.Int("expected_frame_count", expectedFrameCount),
		zap.Int("interval_seconds", j.frameInterval),
	)

	framePaths, err := ffmpeg.ExtractFrames(
		j.videoPath,
		videoFrameDir,
		j.frameInterval,
		j.frameWidth,
		j.frameHeight,
		j.frameQuality,
	)
	if err != nil {
		j.logger.Error("Failed to extract frames",
			zap.Uint("video_id", j.videoID),
			zap.Duration("step_duration", time.Since(stepStart)),
			zap.Error(err),
		)
		j.handleError(fmt.Errorf("frame extraction failed: %w", err))
		return err
	}

	j.logger.Info("Frames extracted successfully",
		zap.Uint("video_id", j.videoID),
		zap.Int("actual_frame_count", len(framePaths)),
		zap.Float64("frames_per_second", float64(len(framePaths))/metadata.Duration),
		zap.Duration("step_duration", time.Since(stepStart)),
	)

	framePathsStr := ffmpeg.ParseFramePaths(framePaths)
	duration := int(metadata.Duration)

	stepStart = time.Now()
	if err := j.repo.UpdateMetadata(
		j.videoID,
		duration,
		metadata.Width,
		metadata.Height,
		thumbnailPath,
		framePathsStr,
		len(framePaths),
		j.frameInterval,
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
		zap.Int("frame_count", len(framePaths)),
		zap.Int("duration", duration),
		zap.Duration("total_duration", totalDuration),
		zap.Float64("processing_rate_secs_per_sec", processingRate),
		zap.String("estimated_storage", fmt.Sprintf("%.2f MB", float64(len(framePaths)*j.frameWidth*j.frameHeight*3)/1024/1024)),
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
