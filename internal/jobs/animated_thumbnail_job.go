package jobs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"goonhub/pkg/compute"
	"goonhub/pkg/workerproto"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AnimatedThumbnailsPlan is what an animated thumbnails job computes: the source and the
// marker clips + preview to make (the server-side preparation).
type AnimatedThumbnailsPlan struct {
	Input  string                // scene stored path
	Video  workerproto.VideoInfo // scene facts
	Params workerproto.AnimatedThumbnailsParams
}

// AnimatedThumbnailGenerator prepares and commits the animated thumbnails of a scene (marker
// clips + scene preview video); the computation in between is pkg/compute. Defined here to
// avoid circular imports between jobs and core packages.
type AnimatedThumbnailGenerator interface {
	// PrepareAnimatedThumbnails returns what to make for the scene, nil when nothing.
	// forceTarget "markers"/"previews"/"both" regenerates existing outputs.
	PrepareAnimatedThumbnails(sceneID uint, forceTarget string) (*AnimatedThumbnailsPlan, error)
	// CommitAnimatedThumbnails moves the outputs found in artifactDir into place and records
	// them (see CommitAnimatedThumbnails).
	CommitAnimatedThumbnails(sceneID uint, res *workerproto.AnimatedThumbnailsResult, artifactDir string) (int, bool, error)
}

type AnimatedThumbnailJob struct {
	id          string
	sceneID     uint
	forceTarget string
	generator   AnimatedThumbnailGenerator
	stagingRoot string
	env         compute.Env
	logger      *zap.Logger
	status      JobStatus
	error       error
	cancelled   atomic.Bool
	ctx         context.Context
	cancelFn    context.CancelFunc
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
	forceTarget string,
	generator AnimatedThumbnailGenerator,
	logger *zap.Logger,
) *AnimatedThumbnailJob {
	return &AnimatedThumbnailJob{
		id:          jobID,
		sceneID:     sceneID,
		forceTarget: forceTarget,
		generator:   generator,
		logger:      logger,
		status:      JobStatusPending,
	}
}

func (j *AnimatedThumbnailJob) GetID() string        { return j.id }
func (j *AnimatedThumbnailJob) GetSceneID() uint     { return j.sceneID }
func (j *AnimatedThumbnailJob) GetPhase() string     { return "animated_thumbnails" }
func (j *AnimatedThumbnailJob) GetStatus() JobStatus { return j.status }
func (j *AnimatedThumbnailJob) GetError() error      { return j.error }

// SetStagingRoot sets the staging root the job computes into before its commit (default:
// the OS temp dir; set it on the generated data file system so the commit is a rename).
func (j *AnimatedThumbnailJob) SetStagingRoot(dir string) { j.stagingRoot = dir }

// SetComputeEnv sets the compute environment (binaries, acceleration; default: software).
func (j *AnimatedThumbnailJob) SetComputeEnv(env compute.Env) { j.env = env }

func (j *AnimatedThumbnailJob) Cancel() {
	j.cancelled.Store(true)
	if j.cancelFn != nil {
		j.cancelFn()
	}
}

func (j *AnimatedThumbnailJob) Execute() error {
	return j.ExecuteWithContext(context.Background())
}

// ExecuteWithContext prepares the clips (generator), computes them into a staging directory
// (pkg/compute) and commits them (generator). Clips made before a cancellation or a timeout
// are still committed.
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

	plan, err := j.generator.PrepareAnimatedThumbnails(j.sceneID, j.forceTarget)
	if err != nil {
		j.error = err
		j.status = JobStatusFailed
		return err
	}

	generated := 0
	if plan != nil && (len(plan.Params.Markers) > 0 || plan.Params.Preview != nil) {
		var computeErr error
		generated, computeErr = j.computeAndCommit(plan)
		if computeErr != nil {
			if j.ctx.Err() == context.DeadlineExceeded {
				j.status = JobStatusTimedOut
				j.error = fmt.Errorf("animated thumbnail generation timed out")
				return j.error
			}
			if j.ctx.Err() == context.Canceled || j.cancelled.Load() {
				j.status = JobStatusCancelled
				return fmt.Errorf("job cancelled")
			}
			j.error = computeErr
			j.status = JobStatusFailed
			return computeErr
		}
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

func (j *AnimatedThumbnailJob) computeAndCommit(plan *AnimatedThumbnailsPlan) (int, error) {
	stagingRoot := j.stagingRoot
	if stagingRoot == "" {
		stagingRoot = filepath.Join(os.TempDir(), "goonhub-staging")
	}
	stage := LocalStagingDir(stagingRoot, j.id)
	if err := os.RemoveAll(stage); err != nil {
		return 0, fmt.Errorf("failed to clear staging directory: %w", err)
	}
	defer os.RemoveAll(stage)

	res, computeErr := compute.AnimatedThumbnails(j.ctx, j.env, compute.Request{
		Input:  plan.Input,
		Video:  plan.Video,
		OutDir: stage,
	}, plan.Params)

	if res != nil {
		for _, f := range res.MarkerFailures {
			j.logger.Warn("Failed to generate animated marker thumbnail",
				zap.Uint("marker_id", f.MarkerID),
				zap.String("error", f.Error))
		}
		if res.PreviewError != "" {
			j.logger.Warn("Failed to generate scene preview",
				zap.Uint("scene_id", j.sceneID),
				zap.String("error", res.PreviewError))
		}
	}

	generated, previewCommitted, commitErr := j.generator.CommitAnimatedThumbnails(j.sceneID, res, stage)
	if commitErr != nil {
		j.logger.Warn("Failed to commit some animated thumbnails",
			zap.Uint("scene_id", j.sceneID),
			zap.Error(commitErr))
	}
	if previewCommitted {
		j.logger.Info("Generated scene preview video",
			zap.Uint("scene_id", j.sceneID),
			zap.String("output", res.Preview.File))
	}
	return generated, computeErr
}
