package core

import (
	"context"
	"goonhub/internal/core/processing"
	"goonhub/internal/data"
	"goonhub/internal/jobs"
	"sync"
	"time"

	"go.uber.org/zap"
)

// JobQueueFeeder polls the database for pending jobs and feeds them to worker pools.
// It acts as a bridge between the infinite-capacity DB queue and the bounded worker pool channels.
type JobQueueFeeder struct {
	repo        data.JobHistoryRepository
	sceneRepo   data.SceneRepository
	poolManager *processing.PoolManager
	logger      *zap.Logger

	pollInterval     time.Duration
	batchSize        int
	channelThreshold int // Feed when channel has space below this threshold

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewJobQueueFeeder creates a new JobQueueFeeder
func NewJobQueueFeeder(
	repo data.JobHistoryRepository,
	sceneRepo data.SceneRepository,
	poolManager *processing.PoolManager,
	logger *zap.Logger,
) *JobQueueFeeder {
	return &JobQueueFeeder{
		repo:             repo,
		sceneRepo:        sceneRepo,
		poolManager:      poolManager,
		logger:           logger.With(zap.String("component", "job_queue_feeder")),
		pollInterval:     2 * time.Second,
		batchSize:        50,
		channelThreshold: 800, // Feed when channel has < 800 of 1000 capacity
	}
}

// Start starts the feeder goroutines for each processing phase
func (f *JobQueueFeeder) Start() {
	f.ctx, f.cancel = context.WithCancel(context.Background())

	// Recover orphaned jobs from previous server crash
	f.recoverOrphanedJobs()

	// Start a feeder goroutine for each phase
	phases := []string{"metadata", "thumbnail", "sprites"}
	for _, phase := range phases {
		f.wg.Add(1)
		go f.runFeeder(phase)
	}

	f.logger.Info("Job queue feeder started",
		zap.Duration("poll_interval", f.pollInterval),
		zap.Int("batch_size", f.batchSize),
		zap.Int("channel_threshold", f.channelThreshold),
	)
}

// Stop stops all feeder goroutines gracefully
func (f *JobQueueFeeder) Stop() {
	f.logger.Info("Stopping job queue feeder")
	f.cancel()
	f.wg.Wait()
	f.logger.Info("Job queue feeder stopped")
}

// recoverOrphanedJobs marks jobs that were running when the server crashed as failed
func (f *JobQueueFeeder) recoverOrphanedJobs() {
	// Jobs running for more than 5 minutes are likely orphaned
	orphanTimeout := 5 * time.Minute

	count, err := f.repo.MarkOrphanedRunningAsFailed(orphanTimeout)
	if err != nil {
		f.logger.Error("Failed to recover orphaned jobs", zap.Error(err))
		return
	}

	if count > 0 {
		f.logger.Info("Recovered orphaned jobs from previous run",
			zap.Int64("count", count),
			zap.Duration("timeout", orphanTimeout),
		)
	}
}

// runFeeder is the main loop for a single phase feeder
func (f *JobQueueFeeder) runFeeder(phase string) {
	defer f.wg.Done()

	ticker := time.NewTicker(f.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-f.ctx.Done():
			return
		case <-ticker.C:
			f.feedPhase(phase)
		}
	}
}

// feedPhase checks if the worker pool has capacity and claims pending jobs
func (f *JobQueueFeeder) feedPhase(phase string) {
	// Get current queue status to check if there's room
	queueStatus := f.poolManager.GetQueueStatus()
	var currentQueued int

	switch phase {
	case "metadata":
		currentQueued = queueStatus.MetadataQueued
	case "thumbnail":
		currentQueued = queueStatus.ThumbnailQueued
	case "sprites":
		currentQueued = queueStatus.SpritesQueued
	}

	// Only feed if there's room in the channel
	if currentQueued >= f.channelThreshold {
		return
	}

	// Calculate how many jobs to claim
	spaceAvailable := f.channelThreshold - currentQueued
	claimLimit := min(spaceAvailable, f.batchSize)

	// Claim pending jobs from DB
	claimedJobs, err := f.repo.ClaimPendingJobs(phase, claimLimit)
	if err != nil {
		f.logger.Error("Failed to claim pending jobs",
			zap.String("phase", phase),
			zap.Error(err),
		)
		return
	}

	if len(claimedJobs) == 0 {
		return
	}

	f.logger.Debug("Claimed pending jobs",
		zap.String("phase", phase),
		zap.Int("count", len(claimedJobs)),
	)

	// Submit claimed jobs to worker pool
	for _, jobRecord := range claimedJobs {
		if err := f.submitJobToPool(jobRecord); err != nil {
			f.logger.Error("Failed to submit claimed job to pool",
				zap.String("job_id", jobRecord.JobID),
				zap.String("phase", phase),
				zap.Uint("scene_id", jobRecord.SceneID),
				zap.Error(err),
			)
			// Mark as failed so it can be retried
			errMsg := "Failed to submit to worker pool: " + err.Error()
			now := time.Now()
			_ = f.repo.UpdateStatus(jobRecord.JobID, data.JobStatusFailed, &errMsg, &now)
		}
	}
}

// submitJobToPool creates and submits a job to the appropriate worker pool
func (f *JobQueueFeeder) submitJobToPool(jobRecord data.JobHistory) error {
	// Get scene data needed for job creation
	scene, err := f.sceneRepo.GetByID(jobRecord.SceneID)
	if err != nil {
		return err
	}

	qualityConfig := f.poolManager.GetQualityConfig()
	cfg := f.poolManager.GetConfig()

	var job jobs.Job

	switch jobRecord.Phase {
	case "metadata":
		job = jobs.NewMetadataJobWithID(
			jobRecord.JobID,
			jobRecord.SceneID,
			scene.StoredPath,
			qualityConfig.MaxFrameDimensionSm,
			qualityConfig.MaxFrameDimensionLg,
			f.sceneRepo,
			f.logger,
		)
		return f.poolManager.SubmitToMetadataPool(job)

	case "thumbnail":
		if scene.Duration == 0 {
			return nil // Skip if metadata not extracted
		}
		tileWidthLg, tileHeightLg := calculateTileDimensions(scene.Width, scene.Height, cfg.MaxFrameDimensionLarge)
		job = jobs.NewThumbnailJobWithID(
			jobRecord.JobID,
			jobRecord.SceneID,
			scene.StoredPath,
			cfg.ThumbnailDir,
			scene.ThumbnailWidth, scene.ThumbnailHeight,
			tileWidthLg, tileHeightLg,
			scene.Duration,
			qualityConfig.FrameQualitySm,
			qualityConfig.FrameQualityLg,
			f.sceneRepo,
			f.logger,
		)
		return f.poolManager.SubmitToThumbnailPool(job)

	case "sprites":
		if scene.Duration == 0 {
			return nil // Skip if metadata not extracted
		}
		job = jobs.NewSpritesJobWithID(
			jobRecord.JobID,
			jobRecord.SceneID,
			scene.StoredPath,
			cfg.SpriteDir,
			cfg.VttDir,
			scene.ThumbnailWidth,
			scene.ThumbnailHeight,
			scene.Duration,
			cfg.FrameInterval,
			qualityConfig.FrameQualitySprites,
			cfg.GridCols,
			cfg.GridRows,
			qualityConfig.SpritesConcurrency,
			f.sceneRepo,
			f.logger,
		)
		return f.poolManager.SubmitToSpritesPool(job)
	}

	return nil
}

// calculateTileDimensions calculates scaled tile dimensions (same as ffmpeg.CalculateTileDimensions)
func calculateTileDimensions(width, height, maxDimension int) (int, int) {
	if width == 0 || height == 0 {
		return maxDimension, maxDimension
	}

	var tileWidth, tileHeight int
	if width > height {
		tileWidth = maxDimension
		tileHeight = int(float64(height) * float64(maxDimension) / float64(width))
	} else {
		tileHeight = maxDimension
		tileWidth = int(float64(width) * float64(maxDimension) / float64(height))
	}

	// Ensure even dimensions for video encoding compatibility
	if tileWidth%2 != 0 {
		tileWidth++
	}
	if tileHeight%2 != 0 {
		tileHeight++
	}

	return tileWidth, tileHeight
}
