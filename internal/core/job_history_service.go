package core

import (
	"context"
	"time"

	"goonhub/internal/config"
	"goonhub/internal/data"

	"go.uber.org/zap"
)

type JobHistoryService struct {
	repo      data.JobHistoryRepository
	retention time.Duration
	retentionStr string
	logger    *zap.Logger
	cancel    context.CancelFunc
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

func (s *JobHistoryService) RecordJobStart(jobID string, videoID uint, videoTitle string, phase string) {
	now := time.Now()
	record := &data.JobHistory{
		JobID:      jobID,
		VideoID:    videoID,
		VideoTitle: videoTitle,
		Phase:      phase,
		Status:     "running",
		StartedAt:  now,
		CreatedAt:  now,
	}
	if err := s.repo.Create(record); err != nil {
		s.logger.Error("Failed to record job start",
			zap.String("job_id", jobID),
			zap.Uint("video_id", videoID),
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
