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

type ThumbnailResult struct {
	ThumbnailPath        string
	ThumbnailWidth       int
	ThumbnailHeight      int
	ThumbnailPathLarge   string
	ThumbnailWidthLarge  int
	ThumbnailHeightLarge int
}

type ThumbnailJob struct {
	id              string
	sceneID         uint
	scenePath       string
	thumbnailDir    string
	tileWidth       int
	tileHeight      int
	tileWidthLarge  int
	tileHeightLarge int
	duration        int
	frameQualitySm  int
	frameQualityLg  int
	repo            data.SceneRepository
	logger          *zap.Logger
	status          JobStatus
	error           error
	cancelled       atomic.Bool
	result          *ThumbnailResult
	ctx             context.Context
	cancelFn        context.CancelFunc

	// Marker thumbnail support (optional)
	markerRepo         data.MarkerRepository
	markerThumbnailDir string
	sceneWidth         int
	sceneHeight        int
}

func NewThumbnailJob(
	sceneID uint,
	scenePath string,
	thumbnailDir string,
	tileWidth int,
	tileHeight int,
	tileWidthLarge int,
	tileHeightLarge int,
	duration int,
	frameQualitySm int,
	frameQualityLg int,
	repo data.SceneRepository,
	logger *zap.Logger,
	markerRepo data.MarkerRepository,
	markerThumbnailDir string,
	sceneWidth int,
	sceneHeight int,
) *ThumbnailJob {
	return &ThumbnailJob{
		id:                 uuid.New().String(),
		sceneID:            sceneID,
		scenePath:          scenePath,
		thumbnailDir:       thumbnailDir,
		tileWidth:          tileWidth,
		tileHeight:         tileHeight,
		tileWidthLarge:     tileWidthLarge,
		tileHeightLarge:    tileHeightLarge,
		duration:           duration,
		frameQualitySm:     frameQualitySm,
		frameQualityLg:     frameQualityLg,
		repo:               repo,
		logger:             logger,
		status:             JobStatusPending,
		markerRepo:         markerRepo,
		markerThumbnailDir: markerThumbnailDir,
		sceneWidth:         sceneWidth,
		sceneHeight:        sceneHeight,
	}
}

// NewThumbnailJobWithID creates a ThumbnailJob with a pre-assigned job ID.
// Used by JobQueueFeeder when creating jobs from pending DB records.
func NewThumbnailJobWithID(
	jobID string,
	sceneID uint,
	scenePath string,
	thumbnailDir string,
	tileWidth int,
	tileHeight int,
	tileWidthLarge int,
	tileHeightLarge int,
	duration int,
	frameQualitySm int,
	frameQualityLg int,
	repo data.SceneRepository,
	logger *zap.Logger,
	markerRepo data.MarkerRepository,
	markerThumbnailDir string,
	sceneWidth int,
	sceneHeight int,
) *ThumbnailJob {
	return &ThumbnailJob{
		id:                 jobID,
		sceneID:            sceneID,
		scenePath:          scenePath,
		thumbnailDir:       thumbnailDir,
		tileWidth:          tileWidth,
		tileHeight:         tileHeight,
		tileWidthLarge:     tileWidthLarge,
		tileHeightLarge:    tileHeightLarge,
		duration:           duration,
		frameQualitySm:     frameQualitySm,
		frameQualityLg:     frameQualityLg,
		repo:               repo,
		logger:             logger,
		status:             JobStatusPending,
		markerRepo:         markerRepo,
		markerThumbnailDir: markerThumbnailDir,
		sceneWidth:         sceneWidth,
		sceneHeight:        sceneHeight,
	}
}

func (j *ThumbnailJob) GetID() string      { return j.id }
func (j *ThumbnailJob) GetSceneID() uint    { return j.sceneID }
func (j *ThumbnailJob) GetPhase() string    { return "thumbnail" }
func (j *ThumbnailJob) GetStatus() JobStatus { return j.status }
func (j *ThumbnailJob) GetError() error     { return j.error }
func (j *ThumbnailJob) GetResult() *ThumbnailResult { return j.result }

func (j *ThumbnailJob) Cancel() {
	j.cancelled.Store(true)
	if j.cancelFn != nil {
		j.cancelFn()
	}
}

func (j *ThumbnailJob) Execute() error {
	return j.ExecuteWithContext(context.Background())
}

func (j *ThumbnailJob) ExecuteWithContext(ctx context.Context) error {
	// Create a cancellable context for this execution
	j.ctx, j.cancelFn = context.WithCancel(ctx)
	defer j.cancelFn()

	startTime := time.Now()
	j.status = JobStatusRunning

	j.logger.Info("Starting thumbnail extraction job",
		zap.String("job_id", j.id),
		zap.Uint("scene_id", j.sceneID),
		zap.String("scene_path", j.scenePath),
		zap.Int("tile_width", j.tileWidth),
		zap.Int("tile_height", j.tileHeight),
	)

	// Check for cancellation
	if j.cancelled.Load() || j.ctx.Err() != nil {
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

	thumbnailPathSmall := filepath.Join(j.thumbnailDir, fmt.Sprintf("%d_thumb_sm.webp", j.sceneID))
	thumbnailPathLarge := filepath.Join(j.thumbnailDir, fmt.Sprintf("%d_thumb_lg.webp", j.sceneID))
	thumbnailSeek := fmt.Sprintf("%d", j.duration/2)

	// Extract small thumbnail
	if err := ffmpeg.ExtractThumbnailWithContext(j.ctx, j.scenePath, thumbnailPathSmall, thumbnailSeek, j.tileWidth, j.tileHeight, j.frameQualitySm); err != nil {
		if j.ctx.Err() == context.DeadlineExceeded {
			j.status = JobStatusTimedOut
			j.error = fmt.Errorf("thumbnail extraction timed out")
			j.repo.UpdateProcessingStatus(j.sceneID, string(JobStatusTimedOut), "thumbnail extraction timed out")
			return j.error
		}
		if j.ctx.Err() == context.Canceled || j.cancelled.Load() {
			j.status = JobStatusCancelled
			return fmt.Errorf("job cancelled")
		}
		j.logger.Error("Failed to extract small thumbnail",
			zap.Uint("scene_id", j.sceneID),
			zap.Error(err),
		)
		j.handleError(fmt.Errorf("small thumbnail extraction failed: %w", err))
		return err
	}

	// Check for cancellation before large thumbnail
	if j.cancelled.Load() || j.ctx.Err() != nil {
		j.status = JobStatusCancelled
		return fmt.Errorf("job cancelled")
	}

	// Extract large thumbnail
	if err := ffmpeg.ExtractThumbnailWithContext(j.ctx, j.scenePath, thumbnailPathLarge, thumbnailSeek, j.tileWidthLarge, j.tileHeightLarge, j.frameQualityLg); err != nil {
		if j.ctx.Err() == context.DeadlineExceeded {
			j.status = JobStatusTimedOut
			j.error = fmt.Errorf("thumbnail extraction timed out")
			j.repo.UpdateProcessingStatus(j.sceneID, string(JobStatusTimedOut), "thumbnail extraction timed out")
			return j.error
		}
		if j.ctx.Err() == context.Canceled || j.cancelled.Load() {
			j.status = JobStatusCancelled
			return fmt.Errorf("job cancelled")
		}
		j.logger.Error("Failed to extract large thumbnail",
			zap.Uint("scene_id", j.sceneID),
			zap.Error(err),
		)
		j.handleError(fmt.Errorf("large thumbnail extraction failed: %w", err))
		return err
	}

	if err := j.repo.UpdateThumbnail(j.sceneID, thumbnailPathSmall, j.tileWidth, j.tileHeight); err != nil {
		j.logger.Error("Failed to update thumbnail in database",
			zap.Uint("scene_id", j.sceneID),
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

	// Generate missing marker thumbnails (best-effort)
	j.generateMissingMarkerThumbnails()

	j.status = JobStatusCompleted
	j.logger.Info("Thumbnail extraction completed",
		zap.String("job_id", j.id),
		zap.Uint("scene_id", j.sceneID),
		zap.String("thumbnail_path_small", thumbnailPathSmall),
		zap.String("thumbnail_path_large", thumbnailPathLarge),
		zap.Duration("elapsed", time.Since(startTime)),
	)

	return nil
}

func (j *ThumbnailJob) handleError(err error) {
	j.error = err
	j.status = JobStatusFailed
	j.repo.UpdateProcessingStatus(j.sceneID, string(JobStatusFailed), err.Error())
}

const (
	markerThumbnailMaxDimension = 320
	markerThumbnailQuality      = 75
)

// generateMissingMarkerThumbnails checks for scene markers without thumbnails and generates them.
// This is best-effort: failures are logged but do not fail the overall thumbnail job.
func (j *ThumbnailJob) generateMissingMarkerThumbnails() {
	if j.markerRepo == nil || j.markerThumbnailDir == "" {
		return
	}

	markers, err := j.markerRepo.GetBySceneWithoutThumbnail(j.sceneID)
	if err != nil {
		j.logger.Warn("Failed to query markers without thumbnails",
			zap.Uint("scene_id", j.sceneID),
			zap.Error(err))
		return
	}

	if len(markers) == 0 {
		return
	}

	j.logger.Info("Generating missing marker thumbnails",
		zap.Uint("scene_id", j.sceneID),
		zap.Int("count", len(markers)))

	if err := os.MkdirAll(j.markerThumbnailDir, 0755); err != nil {
		j.logger.Warn("Failed to create marker thumbnail directory",
			zap.String("dir", j.markerThumbnailDir),
			zap.Error(err))
		return
	}

	tileWidth, tileHeight := ffmpeg.CalculateTileDimensions(j.sceneWidth, j.sceneHeight, markerThumbnailMaxDimension)

	generated := 0
	for i := range markers {
		// Only stop on explicit user cancellation, not on job timeout —
		// marker thumbnails are best-effort work that should not be constrained
		// by the scene thumbnail job's timeout.
		if j.cancelled.Load() {
			j.logger.Info("Marker thumbnail generation interrupted by cancellation",
				zap.Uint("scene_id", j.sceneID),
				zap.Int("generated", generated),
				zap.Int("remaining", len(markers)-i))
			return
		}

		marker := &markers[i]
		thumbnailFilename := fmt.Sprintf("marker_%d.webp", marker.ID)
		thumbnailPath := filepath.Join(j.markerThumbnailDir, thumbnailFilename)

		seekPosition := fmt.Sprintf("%d", marker.Timestamp)

		// Use background context so marker thumbnails are not killed by the job timeout
		extractCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := ffmpeg.ExtractThumbnailWithContext(extractCtx, j.scenePath, thumbnailPath, seekPosition, tileWidth, tileHeight, markerThumbnailQuality); err != nil {
			cancel()
			j.logger.Warn("Failed to generate marker thumbnail",
				zap.Uint("marker_id", marker.ID),
				zap.Int("timestamp", marker.Timestamp),
				zap.Error(err))
			continue
		}
		cancel()

		marker.ThumbnailPath = thumbnailFilename
		if err := j.markerRepo.Update(marker); err != nil {
			os.Remove(thumbnailPath)
			j.logger.Warn("Failed to update marker with thumbnail path",
				zap.Uint("marker_id", marker.ID),
				zap.Error(err))
			continue
		}

		generated++
	}

	if generated > 0 {
		j.logger.Info("Generated missing marker thumbnails",
			zap.Uint("scene_id", j.sceneID),
			zap.Int("generated", generated),
			zap.Int("total", len(markers)))
	}
}
