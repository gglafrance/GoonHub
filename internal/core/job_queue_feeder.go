package core

import (
	"context"
	"fmt"
	"goonhub/internal/core/processing"
	"goonhub/internal/data"
	"goonhub/internal/jobs"
	"goonhub/pkg/ffmpeg"
	"reflect"
	"sync"
	"time"

	"go.uber.org/zap"
)

// JobSchedulePlanner evaluates the job schedule (implemented by JobSchedulingService).
type JobSchedulePlanner interface {
	Plan(now time.Time) SchedulePlan
}

// JobQueueFeeder polls the database for pending jobs and feeds them to worker pools.
// It acts as a bridge between the infinite-capacity DB queue and the bounded worker pool channels.
//
// With a JobSchedulePlanner, it also applies the job schedule: a phase outside its time window
// claims nothing (its pending jobs stay pending, its queued jobs go back to pending, running
// jobs finish), pool sizes follow the quiet hours, the global cap and phase priorities are
// handed to the pools, and the jobs of new scenes are claimed first.
type JobQueueFeeder struct {
	repo               data.JobHistoryRepository
	sceneRepo          data.SceneRepository
	markerThumbGen     jobs.MarkerThumbnailGenerator
	animatedThumbGen   jobs.AnimatedThumbnailGenerator
	poolManager        *processing.PoolManager
	duplicationEnabled bool
	dupConfigRepo      data.DuplicationConfigRepository
	logger             *zap.Logger

	pollInterval     time.Duration
	batchSize        int
	bufferMultiplier int // Max buffered jobs per worker (threshold = workerCount * bufferMultiplier)

	// Configurable timeouts for orphan/stuck job recovery
	orphanTimeout    time.Duration
	stuckPendingTime time.Duration

	// Job scheduling (optional)
	planner      JobSchedulePlanner
	now          func() time.Time
	planMu       sync.RWMutex
	plan         *SchedulePlan
	lastApplied  *processing.ScheduleSettings
	pausedMu     sync.Mutex
	pausedPhases map[string]bool

	// feedLocks serializes, per phase, a feed (claim then submit) with a bulk cancel, so that no
	// job is claimed while the pending jobs of the phase are being cancelled.
	feedLocks map[string]*sync.Mutex
	eventBus  *EventBus

	// Remote workers (optional): phase routing, remote-only phases seen, remote cancel.
	router          PhaseRouter
	remoteOnly      map[string]bool
	remoteCanceller RemoteJobCanceller

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
	duplicationEnabled bool,
	dupConfigRepo data.DuplicationConfigRepository,
	logger *zap.Logger,
) *JobQueueFeeder {
	return &JobQueueFeeder{
		repo:               repo,
		sceneRepo:          sceneRepo,
		markerThumbGen:     markerThumbGen,
		animatedThumbGen:   animatedThumbGen,
		poolManager:        poolManager,
		duplicationEnabled: duplicationEnabled,
		dupConfigRepo:      dupConfigRepo,
		logger:             logger.With(zap.String("component", "job_queue_feeder")),
		pollInterval:       2 * time.Second,
		batchSize:          50,
		bufferMultiplier:   10, // Keep up to workerCount*10 jobs buffered per phase
		orphanTimeout:      30 * time.Second,
		stuckPendingTime:   10 * time.Minute,
		now:                time.Now,
		pausedPhases:       map[string]bool{},
		feedLocks:          newFeedLocks(),
	}
}

// SetScheduler sets the job schedule to apply. Must be called before Start.
func (f *JobQueueFeeder) SetScheduler(planner JobSchedulePlanner) {
	f.planner = planner
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

	// Evaluate the schedule before anything is claimed
	f.refreshSchedule()

	// Recover orphaned jobs from previous server crash
	f.recoverOrphanedJobs()

	// Start a feeder goroutine for each phase
	phases := []string{"metadata", "thumbnail", "sprites", "animated_thumbnails"}
	if f.duplicationEnabled {
		phases = append(phases, "fingerprint")
	}
	for _, phase := range phases {
		f.wg.Add(1)
		go f.runFeeder(phase)
	}
	if f.planner != nil {
		f.wg.Add(1)
		go f.runScheduleLoop()
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

// recoverOrphanedJobs marks jobs that were running when the server crashed as failed, then puts
// back to pending the jobs the previous run interrupted (shutdown, crash, stuck pending).
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

	// Recover stuck pending jobs (jobs stuck in pending state for too long). Phases restricted
	// to a time window are skipped: their jobs wait for the window on purpose.
	var windowed []string
	if plan := f.currentPlan(); plan != nil {
		windowed = plan.WindowedPhases()
	}
	// Jobs of phases only remote workers process wait for a worker on purpose.
	windowed = append(windowed, f.remoteOnlyPhases()...)
	stuckCount, err := f.repo.MarkStuckPendingJobsAsFailed(f.stuckPendingTime, windowed)
	if err != nil {
		f.logger.Error("Failed to recover stuck pending jobs", zap.Error(err))
	} else if stuckCount > 0 {
		f.logger.Info("Recovered stuck pending jobs from previous run",
			zap.Int64("count", stuckCount),
			zap.Duration("threshold", f.stuckPendingTime),
		)
	}

	// Put the interrupted jobs back in the queue (once, retry count kept, dedup respected)
	requeued, err := f.repo.RequeueInterruptedJobs()
	if err != nil {
		f.logger.Error("Failed to re-queue the jobs interrupted by the previous run", zap.Error(err))
	} else {
		f.logger.Info("Re-queued the jobs interrupted by the previous run", zap.Int64("count", requeued))
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

// runScheduleLoop re-evaluates the job schedule periodically
func (f *JobQueueFeeder) runScheduleLoop() {
	defer f.wg.Done()

	ticker := time.NewTicker(f.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-f.ctx.Done():
			return
		case <-ticker.C:
			f.refreshSchedule()
		}
	}
}

// refreshSchedule evaluates the schedule now, stores the plan for the phase feeders and applies
// the pool settings (quiet-hours sizes, global cap, priorities) when they changed.
func (f *JobQueueFeeder) refreshSchedule() {
	if f.planner == nil {
		return
	}
	plan := f.planner.Plan(f.now())
	f.planMu.Lock()
	f.plan = &plan
	f.planMu.Unlock()

	settings := plan.PoolSettings()
	if f.lastApplied != nil && reflect.DeepEqual(*f.lastApplied, settings) {
		return
	}
	f.poolManager.ApplySchedule(settings)
	f.lastApplied = &settings
}

// currentPlan returns the last evaluated plan (nil without scheduling)
func (f *JobQueueFeeder) currentPlan() *SchedulePlan {
	f.planMu.RLock()
	defer f.planMu.RUnlock()
	return f.plan
}

// setPhasePaused records whether a phase is paused by its time window. When it becomes paused,
// the jobs claimed but still waiting in its pool queue go back to pending.
func (f *JobQueueFeeder) setPhasePaused(phase string, paused bool, next time.Time) {
	f.pausedMu.Lock()
	was := f.pausedPhases[phase]
	f.pausedPhases[phase] = paused
	f.pausedMu.Unlock()
	if was == paused {
		return
	}

	until := zap.Skip()
	if !next.IsZero() {
		until = zap.String("until", next.Format(time.RFC3339))
	}
	if !paused {
		f.logger.Info("Phase allowed to run again, resuming", zap.String("phase", phase), until)
		return
	}

	reclaimed := 0
	if ids := f.poolManager.DrainQueued(phase); len(ids) > 0 {
		count, err := f.repo.ResetJobsToPending(ids)
		if err != nil {
			f.logger.Error("Failed to put queued jobs back to pending, they will be recovered as orphans",
				zap.String("phase", phase), zap.Int("count", len(ids)), zap.Error(err))
		}
		reclaimed = int(count)
	}
	f.logger.Info("Phase outside its time window, pausing (running jobs finish)",
		zap.String("phase", phase),
		until,
		zap.Int("queued_jobs_back_to_pending", reclaimed),
	)
}

// feedPhase checks if the worker pool has capacity and claims pending jobs
func (f *JobQueueFeeder) feedPhase(phase string) {
	lock := f.feedLock(phase)
	lock.Lock()
	defer lock.Unlock()

	if !f.localAllowed(phase) {
		return
	}

	var claimOpts data.ClaimOptions
	if plan := f.currentPlan(); plan != nil {
		pp := plan.Phase(phase)
		f.setPhasePaused(phase, !pp.Allowed, pp.NextChange)
		if !pp.Allowed {
			return
		}
		claimOpts.NewSceneSince = plan.NewSceneSince
	}

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
	case "fingerprint":
		currentQueued = queueStatus.FingerprintQueued
		workerCount = poolConfig.FingerprintWorkers
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
	claimedJobs, err := f.repo.ClaimPendingJobs(phase, claimLimit, claimOpts)
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
		spritesJob.SetStagingRoot(jobs.StagingRoot(cfg.StagingDir, cfg.SpriteDir))
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
		animatedJob := jobs.NewAnimatedThumbnailJobWithID(
			jobRecord.JobID,
			jobRecord.SceneID,
			jobRecord.ForceTarget,
			f.animatedThumbGen,
			f.logger,
		)
		animatedJob.SetStagingRoot(jobs.StagingRoot(cfg.StagingDir, cfg.SpriteDir))
		return f.poolManager.SubmitToAnimatedThumbnailsPool(animatedJob)

	case "fingerprint":
		if scene.Duration == 0 {
			return fmt.Errorf("scene duration is 0: metadata not yet extracted")
		}
		fpMode := "audio_only"
		if f.dupConfigRepo != nil {
			if dupCfg, err := f.dupConfigRepo.Get(); err == nil && dupCfg != nil && dupCfg.FingerprintMode != "" {
				fpMode = dupCfg.FingerprintMode
			}
		}
		fpJob := jobs.NewFingerprintJobWithID(
			jobRecord.JobID,
			jobRecord.SceneID,
			scene.StoredPath,
			scene.AudioCodec,
			fpMode,
			f.logger,
		)
		fpJob.SetSceneDuration(scene.Duration)
		fpJob.SetCommitter(f.poolManager.FingerprintCommitter())
		job = fpJob
		return f.poolManager.SubmitToFingerprintPool(job)
	}

	return nil
}
