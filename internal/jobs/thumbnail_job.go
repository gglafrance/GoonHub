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
	ThumbnailPath   string
	ThumbnailWidth  int
	ThumbnailHeight int
}

type ThumbnailJob struct {
	id           string
	videoID      uint
	videoPath    string
	thumbnailDir string
	tileWidth    int
	tileHeight   int
	duration     int
	frameQuality int
	repo         data.VideoRepository
	logger       *zap.Logger
	status       JobStatus
	error        error
	cancelled    atomic.Bool
	result       *ThumbnailResult
}

func NewThumbnailJob(
	videoID uint,
	videoPath string,
	thumbnailDir string,
	tileWidth int,
	tileHeight int,
	duration int,
	frameQuality int,
	repo data.VideoRepository,
	logger *zap.Logger,
) *ThumbnailJob {
	return &ThumbnailJob{
		id:           uuid.New().String(),
		videoID:      videoID,
		videoPath:    videoPath,
		thumbnailDir: thumbnailDir,
		tileWidth:    tileWidth,
		tileHeight:   tileHeight,
		duration:     duration,
		frameQuality: frameQuality,
		repo:         repo,
		logger:       logger,
		status:       JobStatusPending,
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

	thumbnailPath := filepath.Join(j.thumbnailDir, fmt.Sprintf("%d_thumb.webp", j.videoID))
	thumbnailSeek := fmt.Sprintf("%d", j.duration/2)

	if err := ffmpeg.ExtractThumbnail(j.videoPath, thumbnailPath, thumbnailSeek, j.tileWidth, j.tileHeight, j.frameQuality); err != nil {
		j.logger.Error("Failed to extract thumbnail",
			zap.Uint("video_id", j.videoID),
			zap.Error(err),
		)
		j.handleError(fmt.Errorf("thumbnail extraction failed: %w", err))
		return err
	}

	if err := j.repo.UpdateThumbnail(j.videoID, thumbnailPath, j.tileWidth, j.tileHeight); err != nil {
		j.logger.Error("Failed to update thumbnail in database",
			zap.Uint("video_id", j.videoID),
			zap.Error(err),
		)
		j.handleError(fmt.Errorf("failed to update thumbnail: %w", err))
		return err
	}

	j.result = &ThumbnailResult{
		ThumbnailPath:   thumbnailPath,
		ThumbnailWidth:  j.tileWidth,
		ThumbnailHeight: j.tileHeight,
	}

	j.status = JobStatusCompleted
	j.logger.Info("Thumbnail extraction completed",
		zap.String("job_id", j.id),
		zap.Uint("video_id", j.videoID),
		zap.String("thumbnail_path", thumbnailPath),
		zap.Duration("elapsed", time.Since(startTime)),
	)

	return nil
}

func (j *ThumbnailJob) handleError(err error) {
	j.error = err
	j.status = JobStatusFailed
	j.repo.UpdateProcessingStatus(j.videoID, string(JobStatusFailed), err.Error())
}
