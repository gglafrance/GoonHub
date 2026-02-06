package core

import (
	"context"
	"fmt"
	"goonhub/internal/core/processing"
	"goonhub/internal/data"
	"goonhub/internal/jobs"
	"goonhub/pkg/ffmpeg"
	"sync"
	"time"

	"go.uber.org/zap"
)

// JobQueueFeeder polls the database for pending jobs and feeds them to worker pools.
// It acts as a bridge between the infinite-capacity DB queue and the bounded worker pool channels.
type JobQueueFeeder struct {
	repo              data.JobHistoryRepository
	sceneRepo         data.SceneRepository
	markerThumbGen    jobs.MarkerThumbnailGenerator
	animatedThumbGen  jobs.AnimatedThumbnailGenerator
	poolManager       *processing.PoolManager
	logger            *zap.Logger

	pollInterval     time.Duration
	batchSize        int
	bufferMultiplier int // Max buffered jobs per worker (threshold = workerCount * bufferMultiplier)

	// Configurable timeouts for orphan/stuck job recovery
	orphanTimeout    time.Duration
	stuckPendingTime time.Duration

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewJobQueueFeeder creates a new JobQueueFeeder
func NewJobQueueFeeder(
	repo data.JobHistoryRepository,
	sceneRepo data.SceneRepository,
	markerThumbGen jobs.MarkerThumbnailGenerator,
	animatedThumbGen jobs.AnimatedThumbnailGenerator,
	poolManager *processing.PoolManager,
	logger *zap.Logger,
) *JobQueueFeeder {
	return &JobQueueFeeder{
		repo:             repo,
		sceneRepo:        sceneRepo,
		markerThumbGen:   markerThumbGen,
		animatedThumbGen: animatedThumbGen,
		poolManager:      poolManager,
		logger:           logger.With(zap.String("component", "job_queue_feeder")),
		pollInterval:     2 * time.Second,
		batchSize:        50,
		bufferMultiplier: 10, // Keep up to workerCount*10 jobs buffered per phase
		orphanTimeout:    30 * time.Second,
		stuckPendingTime: 10 * time.Minute,
	}
}

// SetOrphanTimeout sets the timeout for detecting orphaned running jobs
func (f *JobQueueFeeder) SetOrphanTimeout(d time.Duration) {
	f.orphanTimeout = d
}

// SetStuckPendingTime sets the threshold for detecting stuck pending jobs
func (f *JobQueueFeeder) SetStuckPendingTime(d time.Duration) {
	f.stuckPendingTime = d
}

// Start starts the feeder goroutines for each processing phase
func (f *JobQueueFeeder) Start() {
	f.ctx, f.cancel = context.WithCancel(context.Background())

	// Recover orphaned jobs from previous server crash
	f.recoverOrphanedJobs()

	// Start a feeder goroutine for each phase
	phases := []string{"metadata", "thumbnail", "sprites", "animated_thumbnails"}
	for _, phase := range phases {
		f.wg.Add(1)
		go f.runFeeder(phase)
	}

	f.logger.Info("Job queue feeder started",
		zap.Duration("poll_interval", f.pollInterval),
		zap.Int("batch_size", f.batchSize),
		zap.Int("buffer_multiplier", f.bufferMultiplier),
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
	// Recover orphaned running jobs (using configurable timeout, default 30s)
	count, err := f.repo.MarkOrphanedRunningAsFailed(f.orphanTimeout)
	if err != nil {
		f.logger.Error("Failed to recover orphaned running jobs", zap.Error(err))
	} else if count > 0 {
		f.logger.Info("Recovered orphaned running jobs from previous run",
			zap.Int64("count", count),
			zap.Duration("timeout", f.orphanTimeout),
		)
	}

	// Recover stuck pending jobs (jobs stuck in pending state for too long)
	stuckCount, err := f.repo.MarkStuckPendingJobsAsFailed(f.stuckPendingTime)
	if err != nil {
		f.logger.Error("Failed to recover stuck pending jobs", zap.Error(err))
	} else if stuckCount > 0 {
		f.logger.Info("Recovered stuck pending jobs from previous run",
			zap.Int64("count", stuckCount),
			zap.Duration("threshold", f.stuckPendingTime),
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
	// Get current queue status and pool config to determine capacity
	queueStatus := f.poolManager.GetQueueStatus()
	poolConfig := f.poolManager.GetPoolConfig()
	var currentQueued int
	var workerCount int

	switch phase {
	case "metadata":
		currentQueued = queueStatus.MetadataQueued
		workerCount = poolConfig.MetadataWorkers
	case "thumbnail":
		currentQueued = queueStatus.ThumbnailQueued
		workerCount = poolConfig.ThumbnailWorkers
	case "sprites":
		currentQueued = queueStatus.SpritesQueued
		workerCount = poolConfig.SpritesWorkers
	case "animated_thumbnails":
		currentQueued = queueStatus.AnimatedThumbnailsQueued
		workerCount = poolConfig.AnimatedThumbnailsWorkers
	}

	// Dynamic threshold: only buffer a small multiple of the worker count.
	// This prevents claiming hundreds of jobs as "running" in the DB when only
	// a few workers are actually executing them.
	threshold := workerCount * f.bufferMultiplier
	if threshold < 1 {
		threshold = 1
	}

	// Only feed if there's room below the threshold
	if currentQueued >= threshold {
		return
	}

	// Calculate how many jobs to claim
	spaceAvailable := threshold - currentQueued
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

	// Batch-fetch all scenes upfront in a single query to avoid N+1 DB lookups
	sceneIDs := make([]uint, 0, len(claimedJobs))
	for _, j := range claimedJobs {
		sceneIDs = append(sceneIDs, j.SceneID)
	}

	scenes, err := f.sceneRepo.GetByIDs(sceneIDs)
	if err != nil {
		f.logger.Error("Failed to batch-fetch scenes for claimed jobs",
			zap.String("phase", phase),
			zap.Error(err),
		)
		// Mark all claimed jobs as failed
		for _, j := range claimedJobs {
			errMsg := "Failed to fetch scene data: " + err.Error()
			now := time.Now()
			if updateErr := f.repo.UpdateStatus(j.JobID, data.JobStatusFailed, &errMsg, &now); updateErr != nil {
				f.logger.Error("Failed to update job status, job may be stuck",
					zap.String("job_id", j.JobID), zap.Error(updateErr))
			}
		}
		return
	}

	sceneMap := make(map[uint]*data.Scene, len(scenes))
	for i := range scenes {
		sceneMap[scenes[i].ID] = &scenes[i]
	}

	// Submit claimed jobs to worker pool
	for _, jobRecord := range claimedJobs {
		scene, ok := sceneMap[jobRecord.SceneID]
		if !ok {
			f.logger.Error("Scene not found for claimed job",
				zap.String("job_id", jobRecord.JobID),
				zap.Uint("scene_id", jobRecord.SceneID),
			)
			errMsg := "Scene not found"
			now := time.Now()
			if updateErr := f.repo.UpdateStatus(jobRecord.JobID, data.JobStatusFailed, &errMsg, &now); updateErr != nil {
				f.logger.Error("Failed to update job status, job may be stuck",
					zap.String("job_id", jobRecord.JobID), zap.Error(updateErr))
			}
			continue
		}

		if err := f.submitJobToPool(jobRecord, scene); err != nil {
			f.logger.Error("Failed to submit claimed job to pool",
				zap.String("job_id", jobRecord.JobID),
				zap.String("phase", phase),
				zap.Uint("scene_id", jobRecord.SceneID),
				zap.Error(err),
			)
			// Mark as failed so it can be retried
			errMsg := "Failed to submit to worker pool: " + err.Error()
			now := time.Now()
			if updateErr := f.repo.UpdateStatus(jobRecord.JobID, data.JobStatusFailed, &errMsg, &now); updateErr != nil {
				f.logger.Error("Failed to update job status, job may be stuck",
					zap.String("job_id", jobRecord.JobID), zap.Error(updateErr))
			}
		}
	}
}

// submitJobToPool creates and submits a job to the appropriate worker pool
func (f *JobQueueFeeder) submitJobToPool(jobRecord data.JobHistory, scene *data.Scene) error {
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
			return fmt.Errorf("scene duration is 0: metadata not yet extracted")
		}
		tileWidthSm, tileHeightSm := scene.ThumbnailWidth, scene.ThumbnailHeight
		if tileWidthSm == 0 || tileHeightSm == 0 {
			tileWidthSm, tileHeightSm = ffmpeg.CalculateTileDimensions(scene.Width, scene.Height, qualityConfig.MaxFrameDimensionSm)
		}
		tileWidthLg, tileHeightLg := ffmpeg.CalculateTileDimensions(scene.Width, scene.Height, cfg.MaxFrameDimensionLarge)
		job = jobs.NewThumbnailJobWithID(
			jobRecord.JobID,
			jobRecord.SceneID,
			scene.StoredPath,
			cfg.ThumbnailDir,
			tileWidthSm, tileHeightSm,
			tileWidthLg, tileHeightLg,
			scene.Duration,
			qualityConfig.FrameQualitySm,
			qualityConfig.FrameQualityLg,
			f.sceneRepo,
			f.logger,
			f.markerThumbGen,
		)
		return f.poolManager.SubmitToThumbnailPool(job)

	case "sprites":
		if scene.Duration == 0 {
			return fmt.Errorf("scene duration is 0: metadata not yet extracted")
		}
		tileW, tileH := scene.ThumbnailWidth, scene.ThumbnailHeight
		if tileW == 0 || tileH == 0 {
			tileW, tileH = ffmpeg.CalculateTileDimensions(scene.Width, scene.Height, qualityConfig.MaxFrameDimensionSm)
		}
		spritesJob := jobs.NewSpritesJobWithID(
			jobRecord.JobID,
			jobRecord.SceneID,
			scene.StoredPath,
			cfg.SpriteDir,
			cfg.VttDir,
			tileW,
			tileH,
			scene.Duration,
			cfg.FrameInterval,
			qualityConfig.FrameQualitySprites,
			cfg.GridCols,
			cfg.GridRows,
			qualityConfig.SpritesConcurrency,
			f.sceneRepo,
			f.logger,
		)
		spritesJob.SetProgressCallback(func(jobID string, progress int) {
			if err := f.repo.UpdateProgress(jobID, progress); err != nil {
				f.logger.Warn("Failed to update sprite job progress",
					zap.String("job_id", jobID), zap.Int("progress", progress), zap.Error(err))
			}
		})
		return f.poolManager.SubmitToSpritesPool(spritesJob)

	case "animated_thumbnails":
		if scene.Duration == 0 {
			return fmt.Errorf("scene duration is 0: metadata not yet extracted")
		}
		job = jobs.NewAnimatedThumbnailJobWithID(
			jobRecord.JobID,
			jobRecord.SceneID,
			jobRecord.ForceTarget,
			f.animatedThumbGen,
			f.logger,
		)
		return f.poolManager.SubmitToAnimatedThumbnailsPool(job)
	}

	return nil
}