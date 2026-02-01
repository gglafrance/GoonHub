package core

import (
	"context"
	"time"

	"goonhub/internal/config"
	"goonhub/internal/data"

	"go.uber.org/zap"
)

type JobHistoryService struct {
	repo           data.JobHistoryRepository
	retention      time.Duration
	retentionStr   string
	logger         *zap.Logger
	cancel         context.CancelFunc
	retryScheduler *RetryScheduler
}

func NewJobHistoryService(repo data.JobHistoryRepository, cfg config.ProcessingConfig, logger *zap.Logger) *JobHistoryService {
	retention, err := config.ParseRetentionDuration(cfg.JobHistoryRetention)
	if err != nil {
		logger.Warn("Failed to parse job_history_retention, using default 7d",
			zap.String("value", cfg.JobHistoryRetention),
			zap.Error(err),
		)
		retention = 7 * 24 * time.Hour
	}

	retentionStr := cfg.JobHistoryRetention
	if retentionStr == "" {
		retentionStr = "7d"
	}

	return &JobHistoryService{
		repo:         repo,
		retention:    retention,
		retentionStr: retentionStr,
		logger:       logger.With(zap.String("component", "job_history")),
	}
}

func (s *JobHistoryService) RecordJobStart(jobID string, sceneID uint, sceneTitle string, phase string) {
	now := time.Now()
	record := &data.JobHistory{
		JobID:      jobID,
		SceneID:    sceneID,
		SceneTitle: sceneTitle,
		Phase:      phase,
		Status:     "running",
		StartedAt:  now,
		CreatedAt:  now,
	}
	if err := s.repo.Create(record); err != nil {
		s.logger.Error("Failed to record job start",
			zap.String("job_id", jobID),
			zap.Uint("scene_id", sceneID),
			zap.Error(err),
		)
	}
}

func (s *JobHistoryService) RecordJobComplete(jobID string) {
	now := time.Now()
	if err := s.repo.UpdateStatus(jobID, "completed", nil, &now); err != nil {
		s.logger.Error("Failed to record job completion",
			zap.String("job_id", jobID),
			zap.Error(err),
		)
	}
}

func (s *JobHistoryService) RecordJobFailed(jobID string, jobErr error) {
	now := time.Now()
	errMsg := jobErr.Error()
	if err := s.repo.UpdateStatus(jobID, "failed", &errMsg, &now); err != nil {
		s.logger.Error("Failed to record job failure",
			zap.String("job_id", jobID),
			zap.Error(err),
		)
	}
}

func (s *JobHistoryService) RecordJobCancelled(jobID string) {
	now := time.Now()
	errMsg := "job was cancelled"
	if err := s.repo.UpdateStatus(jobID, "cancelled", &errMsg, &now); err != nil {
		s.logger.Error("Failed to record job cancellation",
			zap.String("job_id", jobID),
			zap.Error(err),
		)
	}
}

func (s *JobHistoryService) RecordJobTimedOut(jobID string) {
	now := time.Now()
	errMsg := "job timed out"
	if err := s.repo.UpdateStatus(jobID, "timed_out", &errMsg, &now); err != nil {
		s.logger.Error("Failed to record job timeout",
			zap.String("job_id", jobID),
			zap.Error(err),
		)
	}
}

func (s *JobHistoryService) StartCleanupTicker() {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	// Run cleanup immediately on startup
	s.Cleanup()

	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.Cleanup()
			}
		}
	}()

	s.logger.Info("Job history cleanup ticker started",
		zap.Duration("retention", s.retention),
	)
}

func (s *JobHistoryService) StopCleanupTicker() {
	if s.cancel != nil {
		s.cancel()
		s.logger.Info("Job history cleanup ticker stopped")
	}
}

func (s *JobHistoryService) Cleanup() {
	before := time.Now().Add(-s.retention)
	deleted, err := s.repo.DeleteOlderThan(before)
	if err != nil {
		s.logger.Error("Failed to cleanup job history", zap.Error(err))
		return
	}
	if deleted > 0 {
		s.logger.Info("Cleaned up old job history records", zap.Int64("deleted", deleted))
	}
}

func (s *JobHistoryService) ListJobs(page, limit int) ([]data.JobHistory, int64, error) {
	return s.repo.ListAll(page, limit)
}

func (s *JobHistoryService) ListActiveJobs() ([]data.JobHistory, error) {
	return s.repo.ListActive()
}

func (s *JobHistoryService) GetRetention() string {
	return s.retentionStr
}

// SetRetryScheduler sets the retry scheduler for handling failed jobs.
func (s *JobHistoryService) SetRetryScheduler(scheduler *RetryScheduler) {
	s.retryScheduler = scheduler
}

// RecordJobStartWithRetry records a job start with retry configuration.
// retryCount is the current retry attempt (0 for first attempt, inherited from previous failed job).
func (s *JobHistoryService) RecordJobStartWithRetry(jobID string, sceneID uint, sceneTitle string, phase string, maxRetries int, retryCount int) {
	now := time.Now()
	record := &data.JobHistory{
		JobID:       jobID,
		SceneID:     sceneID,
		SceneTitle:  sceneTitle,
		Phase:       phase,
		Status:      "running",
		StartedAt:   now,
		CreatedAt:   now,
		MaxRetries:  maxRetries,
		RetryCount:  retryCount,
		IsRetryable: true,
	}
	if err := s.repo.Create(record); err != nil {
		s.logger.Error("Failed to record job start",
			zap.String("job_id", jobID),
			zap.Uint("scene_id", sceneID),
			zap.Int("retry_count", retryCount),
			zap.Error(err),
		)
	}
}

// UpdateProgress updates the progress of a running job.
func (s *JobHistoryService) UpdateProgress(jobID string, progress int) {
	if err := s.repo.UpdateProgress(jobID, progress); err != nil {
		s.logger.Error("Failed to update job progress",
			zap.String("job_id", jobID),
			zap.Int("progress", progress),
			zap.Error(err),
		)
	}
}

// RecordJobFailedWithRetry records a job failure and schedules a retry if configured.
func (s *JobHistoryService) RecordJobFailedWithRetry(jobID string, sceneID uint, phase string, jobErr error) {
	now := time.Now()
	errMsg := jobErr.Error()

	// Get the current job to check retry count
	job, err := s.repo.GetByJobID(jobID)
	if err != nil {
		s.logger.Error("Failed to get job for retry handling",
			zap.String("job_id", jobID),
			zap.Error(err),
		)
		// Fall back to basic failure recording
		if updateErr := s.repo.UpdateStatus(jobID, "failed", &errMsg, &now); updateErr != nil {
			s.logger.Error("Failed to record job failure", zap.String("job_id", jobID), zap.Error(updateErr))
		}
		return
	}

	// Update status to failed
	if err := s.repo.UpdateStatus(jobID, "failed", &errMsg, &now); err != nil {
		s.logger.Error("Failed to record job failure",
			zap.String("job_id", jobID),
			zap.Error(err),
		)
		return
	}

	// If retry scheduler is configured and job is retryable, schedule retry
	if s.retryScheduler != nil && job.IsRetryable {
		if err := s.retryScheduler.ScheduleRetry(jobID, phase, sceneID, job.RetryCount, errMsg); err != nil {
			s.logger.Error("Failed to schedule retry",
				zap.String("job_id", jobID),
				zap.Error(err),
			)
		}
	}
}

// GetByJobID retrieves a job by its ID.
func (s *JobHistoryService) GetByJobID(jobID string) (*data.JobHistory, error) {
	return s.repo.GetByJobID(jobID)
}

// CreatePendingJob creates a job with status='pending' in the database.
// Used for DB-backed job queue where jobs are created pending and later claimed by the feeder.
func (s *JobHistoryService) CreatePendingJob(jobID string, sceneID uint, sceneTitle string, phase string) error {
	now := time.Now()
	record := &data.JobHistory{
		JobID:       jobID,
		SceneID:     sceneID,
		SceneTitle:  sceneTitle,
		Phase:       phase,
		Status:      data.JobStatusPending,
		CreatedAt:   now,
		IsRetryable: true,
	}
	if err := s.repo.CreatePending(record); err != nil {
		s.logger.Error("Failed to create pending job",
			zap.String("job_id", jobID),
			zap.Uint("scene_id", sceneID),
			zap.String("phase", phase),
			zap.Error(err),
		)
		return err
	}
	s.logger.Debug("Created pending job",
		zap.String("job_id", jobID),
		zap.Uint("scene_id", sceneID),
		zap.String("phase", phase),
	)
	return nil
}

// ExistsPendingOrRunning checks if a pending or running job exists for scene+phase.
// Used for deduplication before creating new pending jobs.
func (s *JobHistoryService) ExistsPendingOrRunning(sceneID uint, phase string) (bool, error) {
	return s.repo.ExistsPendingOrRunning(sceneID, phase)
}

// CountPendingByPhase returns the count of pending jobs per phase.
func (s *JobHistoryService) CountPendingByPhase() (map[string]int, error) {
	return s.repo.CountPendingByPhase()
}
