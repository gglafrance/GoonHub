package jobs

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AnimatedThumbnailGenerator generates animated preview clips for scene markers
// and scene preview videos. Defined here to avoid circular imports between jobs and core packages.
type AnimatedThumbnailGenerator interface {
	GenerateMissingAnimatedForScene(ctx context.Context, sceneID uint) (int, error)
	GenerateScenePreview(ctx context.Context, sceneID uint) error
}

type AnimatedThumbnailJob struct {
	id        string
	sceneID   uint
	generator AnimatedThumbnailGenerator
	logger    *zap.Logger
	status    JobStatus
	error     error
	cancelled atomic.Bool
	ctx       context.Context
	cancelFn  context.CancelFunc
}

func NewAnimatedThumbnailJob(
	sceneID uint,
	generator AnimatedThumbnailGenerator,
	logger *zap.Logger,
) *AnimatedThumbnailJob {
	return &AnimatedThumbnailJob{
		id:        uuid.New().String(),
		sceneID:   sceneID,
		generator: generator,
		logger:    logger,
		status:    JobStatusPending,
	}
}

// NewAnimatedThumbnailJobWithID creates an AnimatedThumbnailJob with a pre-assigned job ID.
// Used by JobQueueFeeder when creating jobs from pending DB records.
func NewAnimatedThumbnailJobWithID(
	jobID string,
	sceneID uint,
	generator AnimatedThumbnailGenerator,
	logger *zap.Logger,
) *AnimatedThumbnailJob {
	return &AnimatedThumbnailJob{
		id:        jobID,
		sceneID:   sceneID,
		generator: generator,
		logger:    logger,
		status:    JobStatusPending,
	}
}

func (j *AnimatedThumbnailJob) GetID() string        { return j.id }
func (j *AnimatedThumbnailJob) GetSceneID() uint      { return j.sceneID }
func (j *AnimatedThumbnailJob) GetPhase() string      { return "animated_thumbnails" }
func (j *AnimatedThumbnailJob) GetStatus() JobStatus   { return j.status }
func (j *AnimatedThumbnailJob) GetError() error       { return j.error }

func (j *AnimatedThumbnailJob) Cancel() {
	j.cancelled.Store(true)
	if j.cancelFn != nil {
		j.cancelFn()
	}
}

func (j *AnimatedThumbnailJob) Execute() error {
	return j.ExecuteWithContext(context.Background())
}

func (j *AnimatedThumbnailJob) ExecuteWithContext(ctx context.Context) error {
	j.ctx, j.cancelFn = context.WithCancel(ctx)
	defer j.cancelFn()

	startTime := time.Now()
	j.status = JobStatusRunning

	j.logger.Info("Starting animated thumbnail job",
		zap.String("job_id", j.id),
		zap.Uint("scene_id", j.sceneID),
	)

	if j.cancelled.Load() || j.ctx.Err() != nil {
		j.status = JobStatusCancelled
		return fmt.Errorf("job cancelled")
	}

	generated, err := j.generator.GenerateMissingAnimatedForScene(j.ctx, j.sceneID)
	if err != nil {
		if j.ctx.Err() == context.DeadlineExceeded {
			j.status = JobStatusTimedOut
			j.error = fmt.Errorf("animated thumbnail generation timed out")
			return j.error
		}
		if j.ctx.Err() == context.Canceled || j.cancelled.Load() {
			j.status = JobStatusCancelled
			return fmt.Errorf("job cancelled")
		}
		j.error = err
		j.status = JobStatusFailed
		return err
	}

	// Generate scene preview video (best-effort, does not fail the job)
	if previewErr := j.generator.GenerateScenePreview(j.ctx, j.sceneID); previewErr != nil {
		if j.ctx.Err() != nil {
			// Propagate cancellation/timeout
			if j.ctx.Err() == context.DeadlineExceeded {
				j.status = JobStatusTimedOut
				j.error = fmt.Errorf("scene preview generation timed out")
				return j.error
			}
			j.status = JobStatusCancelled
			return fmt.Errorf("job cancelled")
		}
		j.logger.Warn("Failed to generate scene preview",
			zap.Uint("scene_id", j.sceneID),
			zap.Error(previewErr))
	}

	j.status = JobStatusCompleted
	j.logger.Info("Animated thumbnail job completed",
		zap.String("job_id", j.id),
		zap.Uint("scene_id", j.sceneID),
		zap.Int("generated", generated),
		zap.Duration("elapsed", time.Since(startTime)),
	)

	return nil
}
