package core

import (
	"fmt"

	"goonhub/internal/data"

	"go.uber.org/zap"
)

// DLQService manages the dead letter queue.
type DLQService struct {
	dlqRepo           data.DLQRepository
	jobHistoryRepo    data.JobHistoryRepository
	sceneRepo         data.SceneRepository
	processingService *SceneProcessingService
	eventBus          *EventBus
	logger            *zap.Logger
}

// NewDLQService creates a new DLQService.
func NewDLQService(
	dlqRepo data.DLQRepository,
	jobHistoryRepo data.JobHistoryRepository,
	sceneRepo data.SceneRepository,
	eventBus *EventBus,
	logger *zap.Logger,
) *DLQService {
	return &DLQService{
		dlqRepo:        dlqRepo,
		jobHistoryRepo: jobHistoryRepo,
		sceneRepo:      sceneRepo,
		eventBus:       eventBus,
		logger:         logger.With(zap.String("component", "dlq_service")),
	}
}

// SetProcessingService sets the scene processing service for resubmitting jobs.
func (s *DLQService) SetProcessingService(svc *SceneProcessingService) {
	s.processingService = svc
}

// ListPending lists DLQ entries with pending_review status.
func (s *DLQService) ListPending(page, limit int) ([]data.DLQEntry, int64, error) {
	return s.dlqRepo.ListPending(page, limit)
}

// ListByStatus lists DLQ entries filtered by status.
func (s *DLQService) ListByStatus(status string, page, limit int) ([]data.DLQEntry, int64, error) {
	return s.dlqRepo.ListByStatus(status, page, limit)
}

// ListAll lists all DLQ entries.
func (s *DLQService) ListAll(page, limit int) ([]data.DLQEntry, int64, error) {
	return s.dlqRepo.ListByStatus("", page, limit)
}

// RetryFromDLQ resubmits a job from the DLQ.
func (s *DLQService) RetryFromDLQ(jobID string) error {
	if s.processingService == nil {
		return fmt.Errorf("processing service not configured")
	}

	// Get the DLQ entry
	entry, err := s.dlqRepo.GetByJobID(jobID)
	if err != nil {
		return fmt.Errorf("failed to get DLQ entry: %w", err)
	}

	// Update status to retrying
	if err := s.dlqRepo.UpdateStatus(jobID, "retrying"); err != nil {
		s.logger.Warn("Failed to update DLQ status to retrying", zap.String("job_id", jobID), zap.Error(err))
	}

	// Resubmit the job
	if err := s.processingService.SubmitPhase(entry.SceneID, entry.Phase); err != nil {
		// Revert status on failure
		if revertErr := s.dlqRepo.UpdateStatus(jobID, "pending_review"); revertErr != nil {
			s.logger.Warn("Failed to revert DLQ status", zap.String("job_id", jobID), zap.Error(revertErr))
		}
		return fmt.Errorf("failed to resubmit job: %w", err)
	}

	// Remove from DLQ on successful submission
	if err := s.dlqRepo.Delete(jobID); err != nil {
		s.logger.Warn("Failed to delete DLQ entry after retry", zap.String("job_id", jobID), zap.Error(err))
	}

	// Publish SSE event
	s.eventBus.Publish(SceneEvent{
		Type:    "scene:dlq_retry",
		SceneID: entry.SceneID,
		Data: map[string]any{
			"job_id": jobID,
			"phase":  entry.Phase,
		},
	})

	s.logger.Info("Resubmitted job from DLQ",
		zap.String("job_id", jobID),
		zap.Uint("scene_id", entry.SceneID),
		zap.String("phase", entry.Phase),
	)

	return nil
}

// Abandon marks a DLQ entry as abandoned.
func (s *DLQService) Abandon(jobID string) error {
	entry, err := s.dlqRepo.GetByJobID(jobID)
	if err != nil {
		return fmt.Errorf("failed to get DLQ entry: %w", err)
	}

	if err := s.dlqRepo.MarkAbandoned(jobID); err != nil {
		return fmt.Errorf("failed to abandon DLQ entry: %w", err)
	}

	s.eventBus.Publish(SceneEvent{
		Type:    "scene:dlq_abandoned",
		SceneID: entry.SceneID,
		Data: map[string]any{
			"job_id": jobID,
			"phase":  entry.Phase,
		},
	})

	s.logger.Info("Abandoned DLQ entry",
		zap.String("job_id", jobID),
		zap.Uint("scene_id", entry.SceneID),
	)

	return nil
}

// GetStats returns counts of DLQ entries by status.
func (s *DLQService) GetStats() (map[string]int64, error) {
	stats := make(map[string]int64)

	pendingCount, err := s.dlqRepo.CountByStatus("pending_review")
	if err != nil {
		return nil, fmt.Errorf("failed to count pending DLQ entries: %w", err)
	}
	stats["pending_review"] = pendingCount

	retryingCount, err := s.dlqRepo.CountByStatus("retrying")
	if err != nil {
		return nil, fmt.Errorf("failed to count retrying DLQ entries: %w", err)
	}
	stats["retrying"] = retryingCount

	abandonedCount, err := s.dlqRepo.CountByStatus("abandoned")
	if err != nil {
		return nil, fmt.Errorf("failed to count abandoned DLQ entries: %w", err)
	}
	stats["abandoned"] = abandonedCount

	stats["total"] = pendingCount + retryingCount + abandonedCount

	return stats, nil
}

// GetByJobID retrieves a single DLQ entry.
func (s *DLQService) GetByJobID(jobID string) (*data.DLQEntry, error) {
	return s.dlqRepo.GetByJobID(jobID)
}
