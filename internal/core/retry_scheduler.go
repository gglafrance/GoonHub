package core

import (
	"context"
	"math"
	"sync"
	"time"

	"goonhub/internal/data"

	"go.uber.org/zap"
)

// RetryScheduler polls for retryable jobs and schedules their retry.
type RetryScheduler struct {
	jobHistoryRepo    data.JobHistoryRepository
	dlqRepo           data.DLQRepository
	retryConfigRepo   data.RetryConfigRepository
	sceneRepo         data.SceneRepository
	eventBus          *EventBus
	logger            *zap.Logger
	processingService *SceneProcessingService
	jobHistoryService *JobHistoryService

	configCache   map[string]data.RetryConfigRecord
	configCacheMu sync.RWMutex

	cancel     context.CancelFunc
	pollTicker *time.Ticker
}

// NewRetryScheduler creates a new RetryScheduler.
func NewRetryScheduler(
	jobHistoryRepo data.JobHistoryRepository,
	dlqRepo data.DLQRepository,
	retryConfigRepo data.RetryConfigRepository,
	sceneRepo data.SceneRepository,
	eventBus *EventBus,
	logger *zap.Logger,
) *RetryScheduler {
	return &RetryScheduler{
		jobHistoryRepo:  jobHistoryRepo,
		dlqRepo:         dlqRepo,
		retryConfigRepo: retryConfigRepo,
		sceneRepo:       sceneRepo,
		eventBus:        eventBus,
		logger:          logger.With(zap.String("component", "retry_scheduler")),
		configCache:     make(map[string]data.RetryConfigRecord),
	}
}

// SetProcessingService sets the scene processing service for resubmitting jobs.
func (rs *RetryScheduler) SetProcessingService(svc *SceneProcessingService) {
	rs.processingService = svc
}

// SetJobHistoryService sets the job history service for recording retries.
func (rs *RetryScheduler) SetJobHistoryService(svc *JobHistoryService) {
	rs.jobHistoryService = svc
}

// Start begins the retry scheduler's background polling.
func (rs *RetryScheduler) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	rs.cancel = cancel
	rs.pollTicker = time.NewTicker(30 * time.Second)

	// Load config cache initially
	if err := rs.refreshConfigCache(); err != nil {
		rs.logger.Warn("Failed to load retry config cache on start", zap.Error(err))
	}

	// Start the polling goroutine
	go func() {
		// Run immediately on startup
		rs.processRetries()
		rs.cleanupOldDLQEntries()

		hourlyTicker := time.NewTicker(1 * time.Hour)
		defer hourlyTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-rs.pollTicker.C:
				rs.processRetries()
			case <-hourlyTicker.C:
				rs.cleanupOldDLQEntries()
			}
		}
	}()

	rs.logger.Info("Retry scheduler started")
}

// Stop halts the retry scheduler.
func (rs *RetryScheduler) Stop() {
	if rs.cancel != nil {
		rs.cancel()
	}
	if rs.pollTicker != nil {
		rs.pollTicker.Stop()
	}
	rs.logger.Info("Retry scheduler stopped")
}

// refreshConfigCache reloads the retry configuration from the database.
func (rs *RetryScheduler) refreshConfigCache() error {
	configs, err := rs.retryConfigRepo.GetAll()
	if err != nil {
		return err
	}

	rs.configCacheMu.Lock()
	defer rs.configCacheMu.Unlock()

	rs.configCache = make(map[string]data.RetryConfigRecord)
	for _, cfg := range configs {
		rs.configCache[cfg.Phase] = cfg
	}

	return nil
}

// GetConfigForPhase returns the retry configuration for a specific phase.
func (rs *RetryScheduler) GetConfigForPhase(phase string) data.RetryConfigRecord {
	rs.configCacheMu.RLock()
	defer rs.configCacheMu.RUnlock()

	if cfg, ok := rs.configCache[phase]; ok {
		return cfg
	}

	// Return default config if not found
	return data.RetryConfigRecord{
		Phase:               phase,
		MaxRetries:          3,
		InitialDelaySeconds: 30,
		MaxDelaySeconds:     3600,
		BackoffFactor:       2.0,
	}
}

// CalculateNextRetryTime calculates the next retry time based on retry count.
func (rs *RetryScheduler) CalculateNextRetryTime(phase string, retryCount int) time.Time {
	cfg := rs.GetConfigForPhase(phase)

	// Calculate delay with exponential backoff
	delay := float64(cfg.InitialDelaySeconds) * math.Pow(cfg.BackoffFactor, float64(retryCount))

	// Cap at max delay
	if delay > float64(cfg.MaxDelaySeconds) {
		delay = float64(cfg.MaxDelaySeconds)
	}

	return time.Now().Add(time.Duration(delay) * time.Second)
}

// ScheduleRetry schedules a retry for a failed job.
func (rs *RetryScheduler) ScheduleRetry(jobID string, phase string, sceneID uint, retryCount int, errorMsg string) error {
	cfg := rs.GetConfigForPhase(phase)

	// Check if we've exhausted retries.
	// retryCount tracks attempts already made. Move to DLQ when retryCount >= maxRetries.
	if retryCount >= cfg.MaxRetries {
		// Update retry info to reflect final state before moving to DLQ
		if err := rs.jobHistoryRepo.UpdateRetryInfo(jobID, retryCount, cfg.MaxRetries, nil); err != nil {
			rs.logger.Warn("Failed to update final retry info before DLQ",
				zap.String("job_id", jobID),
				zap.Error(err),
			)
		}
		return rs.moveToDLQ(jobID, phase, sceneID, errorMsg, retryCount)
	}

	// Calculate next retry time
	nextRetryAt := rs.CalculateNextRetryTime(phase, retryCount)

	// Update job history with retry info
	if err := rs.jobHistoryRepo.UpdateRetryInfo(jobID, retryCount+1, cfg.MaxRetries, &nextRetryAt); err != nil {
		rs.logger.Error("Failed to update retry info",
			zap.String("job_id", jobID),
			zap.Error(err),
		)
		return err
	}

	// Publish SSE event
	rs.eventBus.Publish(SceneEvent{
		Type:    "scene:retry_scheduled",
		SceneID: sceneID,
		Data: map[string]any{
			"job_id":       jobID,
			"phase":        phase,
			"retry_count":  retryCount + 1,
			"max_retries":  cfg.MaxRetries,
			"next_retry_at": nextRetryAt.Format(time.RFC3339),
		},
	})

	rs.logger.Info("Scheduled job retry",
		zap.String("job_id", jobID),
		zap.String("phase", phase),
		zap.Uint("scene_id", sceneID),
		zap.Int("retry_count", retryCount+1),
		zap.Time("next_retry_at", nextRetryAt),
	)

	return nil
}

// moveToDLQ moves a job to the dead letter queue.
func (rs *RetryScheduler) moveToDLQ(jobID string, phase string, sceneID uint, errorMsg string, failureCount int) error {
	// Mark job as not retryable
	if err := rs.jobHistoryRepo.MarkNotRetryable(jobID); err != nil {
		rs.logger.Warn("Failed to mark job as not retryable", zap.String("job_id", jobID), zap.Error(err))
	}

	// Get scene title
	sceneTitle := ""
	if scene, err := rs.sceneRepo.GetByID(sceneID); err == nil {
		sceneTitle = scene.Title
	}

	// Create DLQ entry
	entry := &data.DLQEntry{
		JobID:         jobID,
		SceneID:       sceneID,
		SceneTitle:    sceneTitle,
		Phase:         phase,
		OriginalError: errorMsg,
		FailureCount:  failureCount,
		LastError:     errorMsg,
		Status:        "pending_review",
	}

	if err := rs.dlqRepo.Create(entry); err != nil {
		rs.logger.Error("Failed to create DLQ entry",
			zap.String("job_id", jobID),
			zap.Error(err),
		)
		return err
	}

	// Publish SSE event
	rs.eventBus.Publish(SceneEvent{
		Type:    "scene:dlq_added",
		SceneID: sceneID,
		Data: map[string]any{
			"job_id":        jobID,
			"phase":         phase,
			"failure_count": failureCount,
		},
	})

	rs.logger.Info("Moved job to DLQ",
		zap.String("job_id", jobID),
		zap.String("phase", phase),
		zap.Uint("scene_id", sceneID),
		zap.Int("failure_count", failureCount),
	)

	return nil
}

// processRetries processes all jobs ready for retry.
func (rs *RetryScheduler) processRetries() {
	jobs, err := rs.jobHistoryRepo.GetRetryableJobs()
	if err != nil {
		rs.logger.Error("Failed to get retryable jobs", zap.Error(err))
		return
	}

	if len(jobs) == 0 {
		return
	}

	rs.logger.Debug("Processing retryable jobs", zap.Int("count", len(jobs)))

	for _, job := range jobs {
		rs.retryJob(job)
	}
}

// retryJob resubmits a single job for retry.
func (rs *RetryScheduler) retryJob(job data.JobHistory) {
	if rs.processingService == nil {
		rs.logger.Error("Processing service not set, cannot retry job", zap.String("job_id", job.JobID))
		return
	}

	cfg := rs.GetConfigForPhase(job.Phase)

	// Check if we've exhausted retries (should have been moved to DLQ already, but handle edge case)
	if job.RetryCount >= cfg.MaxRetries {
		rs.logger.Warn("Job picked up for retry but already at max retries, moving to DLQ",
			zap.String("job_id", job.JobID),
			zap.Int("retry_count", job.RetryCount),
			zap.Int("max_retries", cfg.MaxRetries),
		)
		errorMsg := ""
		if job.ErrorMessage != nil {
			errorMsg = *job.ErrorMessage
		}
		if err := rs.moveToDLQ(job.JobID, job.Phase, job.SceneID, errorMsg, job.RetryCount); err != nil {
			rs.logger.Error("Failed to move job to DLQ", zap.String("job_id", job.JobID), zap.Error(err))
		}
		return
	}

	// Mark job as not retryable to prevent double processing
	if err := rs.jobHistoryRepo.MarkNotRetryable(job.JobID); err != nil {
		rs.logger.Warn("Failed to mark job as not retryable before retry", zap.String("job_id", job.JobID), zap.Error(err))
	}

	// Resubmit the job with retry count so the new job inherits the retry state
	if err := rs.processingService.SubmitPhaseWithRetry(job.SceneID, job.Phase, job.RetryCount, cfg.MaxRetries); err != nil {
		rs.logger.Error("Failed to resubmit job for retry",
			zap.String("job_id", job.JobID),
			zap.Uint("scene_id", job.SceneID),
			zap.String("phase", job.Phase),
			zap.Int("retry_count", job.RetryCount),
			zap.Error(err),
		)

		// If resubmission fails, schedule another retry or move to DLQ
		errorMsg := err.Error()
		if job.RetryCount >= cfg.MaxRetries {
			if dlqErr := rs.moveToDLQ(job.JobID, job.Phase, job.SceneID, errorMsg, job.RetryCount); dlqErr != nil {
				rs.logger.Error("Failed to move job to DLQ after retry failure", zap.Error(dlqErr))
			}
		} else {
			nextRetryAt := rs.CalculateNextRetryTime(job.Phase, job.RetryCount+1)
			if updateErr := rs.jobHistoryRepo.UpdateRetryInfo(job.JobID, job.RetryCount+1, cfg.MaxRetries, &nextRetryAt); updateErr != nil {
				rs.logger.Error("Failed to reschedule retry", zap.Error(updateErr))
			}
		}
		return
	}

	rs.logger.Info("Resubmitted job for retry",
		zap.String("original_job_id", job.JobID),
		zap.Uint("scene_id", job.SceneID),
		zap.String("phase", job.Phase),
		zap.Int("retry_count", job.RetryCount),
	)
}

// cleanupOldDLQEntries auto-abandons DLQ entries older than 7 days.
func (rs *RetryScheduler) cleanupOldDLQEntries() {
	abandoned, err := rs.dlqRepo.AutoAbandon(7 * 24 * time.Hour)
	if err != nil {
		rs.logger.Error("Failed to auto-abandon old DLQ entries", zap.Error(err))
		return
	}

	if abandoned > 0 {
		rs.logger.Info("Auto-abandoned old DLQ entries", zap.Int64("count", abandoned))
	}
}

// RefreshConfigCache refreshes the retry configuration cache.
func (rs *RetryScheduler) RefreshConfigCache() error {
	return rs.refreshConfigCache()
}
