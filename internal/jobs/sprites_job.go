package jobs

import (
	"context"
	"fmt"
	"goonhub/internal/data"
	"goonhub/pkg/compute"
	"goonhub/pkg/workerproto"
	"os"
	"sync"
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
	id               string
	sceneID          uint
	scenePath        string
	spriteDir        string
	vttDir           string
	tileWidth        int
	tileHeight       int
	duration         int
	frameInterval    int
	frameQuality     int
	gridCols         int
	gridRows         int
	concurrency      int
	stagingRoot      string
	env              compute.Env
	repo             data.SceneRepository
	logger           *zap.Logger
	status           JobStatus
	error            error
	cancelled        atomic.Bool
	result           *SpritesResult
	ctx              context.Context
	cancelFn         context.CancelFunc
	progressCallback ProgressCallback
	progressMu       sync.Mutex
}

func NewSpritesJob(
	sceneID uint,
	scenePath string,
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
	repo data.SceneRepository,
	logger *zap.Logger,
) *SpritesJob {
	return &SpritesJob{
		id:            uuid.New().String(),
		sceneID:       sceneID,
		scenePath:     scenePath,
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

// NewSpritesJobWithID creates a SpritesJob with a pre-assigned job ID.
// Used by JobQueueFeeder when creating jobs from pending DB records.
func NewSpritesJobWithID(
	jobID string,
	sceneID uint,
	scenePath string,
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
	repo data.SceneRepository,
	logger *zap.Logger,
) *SpritesJob {
	return &SpritesJob{
		id:            jobID,
		sceneID:       sceneID,
		scenePath:     scenePath,
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

func (j *SpritesJob) GetID() string             { return j.id }
func (j *SpritesJob) GetSceneID() uint          { return j.sceneID }
func (j *SpritesJob) GetPhase() string          { return "sprites" }
func (j *SpritesJob) GetStatus() JobStatus      { return j.status }
func (j *SpritesJob) GetError() error           { return j.error }
func (j *SpritesJob) GetResult() *SpritesResult { return j.result }

func (j *SpritesJob) Cancel() {
	j.cancelled.Store(true)
	if j.cancelFn != nil {
		j.cancelFn()
	}
}

// SetStagingRoot sets the staging root the job computes into before its commit (default:
// StagingRoot("", spriteDir)).
func (j *SpritesJob) SetStagingRoot(dir string) { j.stagingRoot = dir }

// SetComputeEnv sets the compute environment (binaries, acceleration; default: software).
func (j *SpritesJob) SetComputeEnv(env compute.Env) { j.env = env }

// Params returns the computation parameters of the job.
func (j *SpritesJob) Params() workerproto.SpritesParams {
	return workerproto.SpritesParams{
		SceneID:       j.sceneID,
		TileWidth:     j.tileWidth,
		TileHeight:    j.tileHeight,
		FrameInterval: j.frameInterval,
		FrameQuality:  j.frameQuality,
		GridCols:      j.gridCols,
		GridRows:      j.gridRows,
		Concurrency:   j.concurrency,
	}
}

// SetProgressCallback sets the progress callback for this job.
func (j *SpritesJob) SetProgressCallback(callback ProgressCallback) {
	j.progressMu.Lock()
	defer j.progressMu.Unlock()
	j.progressCallback = callback
}

// reportProgress reports progress to the callback if set.
func (j *SpritesJob) reportProgress(progress int) {
	j.progressMu.Lock()
	callback := j.progressCallback
	j.progressMu.Unlock()

	if callback != nil {
		callback(j.id, progress)
	}
}

func (j *SpritesJob) Execute() error {
	return j.ExecuteWithContext(context.Background())
}

// effectiveSpriteInterval: see workerproto.EffectiveSpriteInterval.
func effectiveSpriteInterval(duration, interval, minFrames int) int {
	return workerproto.EffectiveSpriteInterval(duration, interval, minFrames)
}

// ExecuteWithContext prepares the parameters, computes the sprites into a staging directory
// (pkg/compute, the same computation a remote worker runs) and commits them (CommitSprites).
func (j *SpritesJob) ExecuteWithContext(ctx context.Context) error {
	// Create a cancellable context for this execution
	j.ctx, j.cancelFn = context.WithCancel(ctx)
	defer j.cancelFn()

	startTime := time.Now()
	j.status = JobStatusRunning
	params := j.Params()

	j.logger.Info("Starting sprite sheet generation job",
		zap.String("job_id", j.id),
		zap.Uint("scene_id", j.sceneID),
		zap.Int("tile_width", j.tileWidth),
		zap.Int("tile_height", j.tileHeight),
		zap.Int("frame_interval", effectiveSpriteInterval(j.duration, j.frameInterval, j.gridCols)),
		zap.Int("grid_cols", j.gridCols),
		zap.Int("grid_rows", j.gridRows),
	)

	// Check for cancellation
	if j.cancelled.Load() || j.ctx.Err() != nil {
		j.status = JobStatusCancelled
		return fmt.Errorf("job cancelled")
	}

	stagingRoot := j.stagingRoot
	if stagingRoot == "" {
		stagingRoot = StagingRoot("", j.spriteDir)
	}
	stage := LocalStagingDir(stagingRoot, j.id)
	if err := os.RemoveAll(stage); err != nil {
		j.handleError(fmt.Errorf("failed to clear staging directory: %w", err))
		return j.error
	}
	defer os.RemoveAll(stage)

	computed, err := compute.Sprites(j.ctx, j.env, compute.Request{
		Input:    j.scenePath,
		Video:    workerproto.VideoInfo{DurationSec: j.duration},
		OutDir:   stage,
		Progress: j.reportProgress,
	}, params)
	if err != nil {
		if j.ctx.Err() == context.DeadlineExceeded {
			j.status = JobStatusTimedOut
			j.error = fmt.Errorf("sprite sheet generation timed out")
			j.repo.UpdateProcessingStatus(j.sceneID, string(JobStatusTimedOut), "sprite sheet generation timed out")
			return j.error
		}
		if j.ctx.Err() == context.Canceled || j.cancelled.Load() {
			j.status = JobStatusCancelled
			return fmt.Errorf("job cancelled")
		}
		j.logger.Error("Failed to generate sprite sheets",
			zap.Uint("scene_id", j.sceneID),
			zap.Error(err),
		)
		j.handleError(err)
		return err
	}

	j.logger.Info("Sprite sheets generated",
		zap.Uint("scene_id", j.sceneID),
		zap.Int("count", computed.SheetCount),
	)

	result, err := CommitSprites(j.repo, j.spriteDir, j.vttDir, j.sceneID, computed, stage)
	if err != nil {
		j.logger.Error("Failed to commit sprites",
			zap.Uint("scene_id", j.sceneID),
			zap.Error(err),
		)
		j.handleError(err)
		return err
	}
	j.result = result

	j.status = JobStatusCompleted
	j.logger.Info("Sprite sheet generation completed",
		zap.String("job_id", j.id),
		zap.Uint("scene_id", j.sceneID),
		zap.Int("sprite_sheet_count", result.SpriteSheetCount),
		zap.String("vtt_path", result.VttPath),
		zap.Duration("elapsed", time.Since(startTime)),
	)

	return nil
}

func (j *SpritesJob) handleError(err error) {
	j.error = err
	j.status = JobStatusFailed
	j.repo.UpdateProcessingStatus(j.sceneID, string(JobStatusFailed), err.Error())
}
