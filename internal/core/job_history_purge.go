package core

import (
	"context"
	"fmt"
	"time"

	"goonhub/internal/apperrors"
	"goonhub/internal/config"
	"goonhub/internal/data"

	"go.uber.org/zap"
)

// Job history purge: finished jobs are deleted from job_history once older than their retention
// (completed and cancelled jobs after processing.job_history_retention, failed and timed out jobs
// after processing.job_history_failed_retention). Pending and running jobs (the job queue), the
// dead letter queue and every other table are never touched. Rows are deleted in batches so that
// a large table is never locked for long.

const (
	defaultCompletedRetention = "30d"
	defaultFailedRetention    = "90d"
	defaultPurgeInterval      = time.Hour
	defaultPurgeBatchSize     = 5000

	// HistoryPurgeScheduled and HistoryPurgeManual tell what started a purge.
	HistoryPurgeScheduled = "scheduled"
	HistoryPurgeManual    = "manual"
)

var (
	purgeCompletedStatuses = []string{data.JobStatusCompleted, data.JobStatusCancelled}
	purgeFailedStatuses    = []string{data.JobStatusFailed, data.JobStatusTimedOut}
)

// HistoryPurgeSettings is the job history purge configuration (config.yaml, processing section).
type HistoryPurgeSettings struct {
	// CompletedRetention is the configured retention of completed and cancelled jobs ("30d").
	CompletedRetention string `json:"completed_retention"`
	// FailedRetention is the configured retention of failed and timed out jobs ("90d").
	FailedRetention string `json:"failed_retention"`
	// CompletedEnabled and FailedEnabled are false when the retention is 0 (rows kept forever).
	CompletedEnabled bool   `json:"completed_enabled"`
	FailedEnabled    bool   `json:"failed_enabled"`
	Interval         string `json:"interval"`
	BatchSize        int    `json:"batch_size"`

	completedAge time.Duration
	failedAge    time.Duration
	interval     time.Duration
}

// HistoryPurgePreview counts the rows a purge would delete now, per status.
type HistoryPurgePreview struct {
	Completed       int64      `json:"completed"`
	Cancelled       int64      `json:"cancelled"`
	Failed          int64      `json:"failed"`
	TimedOut        int64      `json:"timed_out"`
	Total           int64      `json:"total"`
	CompletedBefore *time.Time `json:"completed_before,omitempty"`
	FailedBefore    *time.Time `json:"failed_before,omitempty"`
}

// HistoryPurgeResult is the outcome of a purge run.
type HistoryPurgeResult struct {
	Trigger string `json:"trigger"`
	// CompletedDeleted counts the completed and cancelled rows deleted.
	CompletedDeleted int64 `json:"completed_deleted"`
	// FailedDeleted counts the failed and timed out rows deleted.
	FailedDeleted int64     `json:"failed_deleted"`
	Total         int64     `json:"total"`
	Batches       int       `json:"batches"`
	StartedAt     time.Time `json:"started_at"`
	DurationMs    int64     `json:"duration_ms"`
	Error         string    `json:"error,omitempty"`
}

// parseRetention parses a retention setting. Empty uses the default; 0 or less disables the purge
// of that group (returned as 0).
func parseRetention(value, fallback, key string, logger *zap.Logger) (string, time.Duration) {
	if value == "" {
		value = fallback
	}
	d, err := config.ParseRetentionDuration(value)
	if err != nil {
		logger.Warn("Invalid job history retention, using the default",
			zap.String("key", key), zap.String("value", value), zap.String("default", fallback), zap.Error(err))
		value = fallback
		d, _ = config.ParseRetentionDuration(fallback)
	}
	if d < 0 {
		d = 0
	}
	return value, d
}

// formatInterval renders a duration the way config.yaml usually writes it ("1h", "30m").
func formatInterval(d time.Duration) string {
	switch {
	case d%time.Hour == 0:
		return fmt.Sprintf("%dh", d/time.Hour)
	case d%time.Minute == 0:
		return fmt.Sprintf("%dm", d/time.Minute)
	default:
		return d.String()
	}
}

func newHistoryPurgeSettings(cfg config.ProcessingConfig, logger *zap.Logger) HistoryPurgeSettings {
	s := HistoryPurgeSettings{}
	s.CompletedRetention, s.completedAge = parseRetention(cfg.JobHistoryRetention, defaultCompletedRetention, "job_history_retention", logger)
	s.FailedRetention, s.failedAge = parseRetention(cfg.JobHistoryFailedRetention, defaultFailedRetention, "job_history_failed_retention", logger)
	s.CompletedEnabled = s.completedAge > 0
	s.FailedEnabled = s.failedAge > 0

	s.interval = cfg.JobHistoryPurgeInterval
	if s.interval <= 0 {
		s.interval = defaultPurgeInterval
	}
	s.Interval = formatInterval(s.interval)

	s.BatchSize = cfg.JobHistoryPurgeBatchSize
	if s.BatchSize <= 0 {
		s.BatchSize = defaultPurgeBatchSize
	}
	return s
}

// GetPurgeSettings returns the job history purge configuration.
func (s *JobHistoryService) GetPurgeSettings() HistoryPurgeSettings {
	return s.purge
}

// LastPurge returns the result of the last purge run since startup (nil if none ran yet).
func (s *JobHistoryService) LastPurge() *HistoryPurgeResult {
	s.lastMu.RLock()
	defer s.lastMu.RUnlock()
	if s.lastPurge == nil {
		return nil
	}
	last := *s.lastPurge
	return &last
}

// PreviewPurge counts the rows a purge would delete right now.
func (s *JobHistoryService) PreviewPurge() (*HistoryPurgePreview, error) {
	now := s.now()
	preview := &HistoryPurgePreview{}
	if s.purge.CompletedEnabled {
		before := now.Add(-s.purge.completedAge)
		counts, err := s.repo.CountPurgeable(purgeCompletedStatuses, before)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to count purgeable jobs", err)
		}
		preview.Completed = counts[data.JobStatusCompleted]
		preview.Cancelled = counts[data.JobStatusCancelled]
		preview.CompletedBefore = &before
	}
	if s.purge.FailedEnabled {
		before := now.Add(-s.purge.failedAge)
		counts, err := s.repo.CountPurgeable(purgeFailedStatuses, before)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to count purgeable jobs", err)
		}
		preview.Failed = counts[data.JobStatusFailed]
		preview.TimedOut = counts[data.JobStatusTimedOut]
		preview.FailedBefore = &before
	}
	preview.Total = preview.Completed + preview.Cancelled + preview.Failed + preview.TimedOut
	return preview, nil
}

// PurgeHistory deletes the finished jobs older than their retention, in batches of
// BatchSize rows, and records the result as the last purge. Only one purge runs at a time.
func (s *JobHistoryService) PurgeHistory(ctx context.Context, trigger string) (*HistoryPurgeResult, error) {
	s.purgeMu.Lock()
	defer s.purgeMu.Unlock()

	start := s.now()
	result := &HistoryPurgeResult{Trigger: trigger, StartedAt: start}

	var err error
	if s.purge.CompletedEnabled {
		result.CompletedDeleted, err = s.purgeGroup(ctx, purgeCompletedStatuses, start.Add(-s.purge.completedAge), result)
	}
	if err == nil && s.purge.FailedEnabled {
		result.FailedDeleted, err = s.purgeGroup(ctx, purgeFailedStatuses, start.Add(-s.purge.failedAge), result)
	}
	result.Total = result.CompletedDeleted + result.FailedDeleted
	result.DurationMs = s.now().Sub(start).Milliseconds()
	if err != nil {
		result.Error = err.Error()
	}

	s.lastMu.Lock()
	last := *result
	s.lastPurge = &last
	s.lastMu.Unlock()

	if err != nil {
		return result, apperrors.NewInternalError("failed to purge job history", err)
	}
	return result, nil
}

// purgeGroup deletes the rows of one status group in batches until a batch comes back short.
func (s *JobHistoryService) purgeGroup(ctx context.Context, statuses []string, before time.Time, result *HistoryPurgeResult) (int64, error) {
	var deleted int64
	for {
		if err := ctx.Err(); err != nil {
			return deleted, err
		}
		n, err := s.repo.PurgeFinishedBatch(statuses, before, s.purge.BatchSize)
		if err != nil {
			return deleted, err
		}
		deleted += n
		result.Batches++
		if n < int64(s.purge.BatchSize) {
			return deleted, nil
		}
	}
}

// Cleanup runs a scheduled purge and logs its outcome.
func (s *JobHistoryService) Cleanup(ctx context.Context) {
	result, err := s.PurgeHistory(ctx, HistoryPurgeScheduled)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		s.logger.Error("Failed to purge job history", zap.Error(err))
		return
	}
	if result.Total > 0 {
		s.logger.Info("Purged old job history records",
			zap.Int64("completed_cancelled", result.CompletedDeleted),
			zap.Int64("failed_timed_out", result.FailedDeleted),
			zap.Int("batches", result.Batches),
			zap.Int64("duration_ms", result.DurationMs),
		)
	}
}

// StartCleanupTicker purges the job history right away, then every purge interval.
func (s *JobHistoryService) StartCleanupTicker() {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	go func() {
		s.Cleanup(ctx)
		ticker := time.NewTicker(s.purge.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.Cleanup(ctx)
			}
		}
	}()

	s.logger.Info("Job history purge ticker started",
		zap.String("completed_retention", s.purge.CompletedRetention),
		zap.String("failed_retention", s.purge.FailedRetention),
		zap.Duration("interval", s.purge.interval),
		zap.Int("batch_size", s.purge.BatchSize),
	)
}

// StopCleanupTicker stops the purge ticker (an ongoing purge stops after its current batch).
func (s *JobHistoryService) StopCleanupTicker() {
	if s.cancel != nil {
		s.cancel()
		s.logger.Info("Job history purge ticker stopped")
	}
}
