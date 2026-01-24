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

type ThumbnailResult struct {
	ThumbnailPath        string
	ThumbnailWidth       int
	ThumbnailHeight      int
	ThumbnailPathLarge   string
	ThumbnailWidthLarge  int
	ThumbnailHeightLarge int
}

type ThumbnailJob struct {
	id             string
	videoID        uint
	videoPath      string
	thumbnailDir   string
	tileWidth      int
	tileHeight     int
	tileWidthLarge  int
	tileHeightLarge int
	duration       int
	frameQuality   int
	repo           data.VideoRepository
	logger         *zap.Logger
	status         JobStatus
	error          error
	cancelled      atomic.Bool
	result         *ThumbnailResult
}

func NewThumbnailJob(
	videoID uint,
	videoPath string,
	thumbnailDir string,
	tileWidth int,
	tileHeight int,
	tileWidthLarge int,
	tileHeightLarge int,
	duration int,
	frameQuality int,
	repo data.VideoRepository,
	logger *zap.Logger,
) *ThumbnailJob {
	return &ThumbnailJob{
		id:              uuid.New().String(),
		videoID:         videoID,
		videoPath:       videoPath,
		thumbnailDir:    thumbnailDir,
		tileWidth:       tileWidth,
		tileHeight:      tileHeight,
		tileWidthLarge:  tileWidthLarge,
		tileHeightLarge: tileHeightLarge,
		duration:        duration,
		frameQuality:    frameQuality,
		repo:            repo,
		logger:          logger,
		status:          JobStatusPending,
	}
}

func (j *ThumbnailJob) GetID() string      { return j.id }
func (j *ThumbnailJob) GetVideoID() uint    { return j.videoID }
func (j *ThumbnailJob) GetPhase() string    { return "thumbnail" }
func (j *ThumbnailJob) GetStatus() JobStatus { return j.status }
func (j *ThumbnailJob) GetError() error     { return j.error }
func (j *ThumbnailJob) GetResult() *ThumbnailResult { return j.result }

func (j *ThumbnailJob) Cancel() {
	j.cancelled.Store(true)
}

func (j *ThumbnailJob) Execute() error {
	startTime := time.Now()
	j.status = JobStatusRunning

	j.logger.Info("Starting thumbnail extraction job",
		zap.String("job_id", j.id),
		zap.Uint("video_id", j.videoID),
		zap.Int("tile_width", j.tileWidth),
		zap.Int("tile_height", j.tileHeight),
	)

	if j.cancelled.Load() {
		j.status = JobStatusCancelled
		return fmt.Errorf("job cancelled")
	}

	if err := os.MkdirAll(j.thumbnailDir, 0755); err != nil {
		j.logger.Error("Failed to create thumbnail directory",
			zap.String("dir", j.thumbnailDir),
			zap.Error(err),
		)
		j.handleError(fmt.Errorf("failed to create thumbnail directory: %w", err))
		return err
	}

	thumbnailPathSmall := filepath.Join(j.thumbnailDir, fmt.Sprintf("%d_thumb_sm.webp", j.videoID))
	thumbnailPathLarge := filepath.Join(j.thumbnailDir, fmt.Sprintf("%d_thumb_lg.webp", j.videoID))
	thumbnailSeek := fmt.Sprintf("%d", j.duration/2)

	// Extract small thumbnail
	if err := ffmpeg.ExtractThumbnail(j.videoPath, thumbnailPathSmall, thumbnailSeek, j.tileWidth, j.tileHeight, j.frameQuality); err != nil {
		j.logger.Error("Failed to extract small thumbnail",
			zap.Uint("video_id", j.videoID),
			zap.Error(err),
		)
		j.handleError(fmt.Errorf("small thumbnail extraction failed: %w", err))
		return err
	}

	// Extract large thumbnail
	if err := ffmpeg.ExtractThumbnail(j.videoPath, thumbnailPathLarge, thumbnailSeek, j.tileWidthLarge, j.tileHeightLarge, j.frameQuality); err != nil {
		j.logger.Error("Failed to extract large thumbnail",
			zap.Uint("video_id", j.videoID),
			zap.Error(err),
		)
		j.handleError(fmt.Errorf("large thumbnail extraction failed: %w", err))
		return err
	}

	if err := j.repo.UpdateThumbnail(j.videoID, thumbnailPathSmall, j.tileWidth, j.tileHeight); err != nil {
		j.logger.Error("Failed to update thumbnail in database",
			zap.Uint("video_id", j.videoID),
			zap.Error(err),
		)
		j.handleError(fmt.Errorf("failed to update thumbnail: %w", err))
		return err
	}

	j.result = &ThumbnailResult{
		ThumbnailPath:        thumbnailPathSmall,
		ThumbnailWidth:       j.tileWidth,
		ThumbnailHeight:      j.tileHeight,
		ThumbnailPathLarge:   thumbnailPathLarge,
		ThumbnailWidthLarge:  j.tileWidthLarge,
		ThumbnailHeightLarge: j.tileHeightLarge,
	}

	j.status = JobStatusCompleted
	j.logger.Info("Thumbnail extraction completed",
		zap.String("job_id", j.id),
		zap.Uint("video_id", j.videoID),
		zap.String("thumbnail_path_small", thumbnailPathSmall),
		zap.String("thumbnail_path_large", thumbnailPathLarge),
		zap.Duration("elapsed", time.Since(startTime)),
	)

	return nil
}

func (j *ThumbnailJob) handleError(err error) {
	j.error = err
	j.status = JobStatusFailed
	j.repo.UpdateProcessingStatus(j.videoID, string(JobStatusFailed), err.Error())
}
