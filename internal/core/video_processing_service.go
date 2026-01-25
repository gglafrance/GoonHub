package core

import (
	"goonhub/internal/config"
	"goonhub/internal/core/processing"
	"goonhub/internal/data"
	"goonhub/internal/jobs"

	"go.uber.org/zap"
)

// Type aliases for backward compatibility
type PoolConfig = processing.PoolConfig
type ProcessingQualityConfig = processing.QualityConfig
type QueueStatus = processing.QueueStatus
type BulkPhaseResult = processing.BulkPhaseResult

// eventBusAdapter adapts EventBus to the processing.EventPublisher interface
type eventBusAdapter struct {
	eventBus *EventBus
}

func (a *eventBusAdapter) Publish(event processing.VideoEvent) {
	a.eventBus.Publish(VideoEvent{
		Type:    event.Type,
		VideoID: event.VideoID,
		Data:    event.Data,
	})
}

// jobHistoryAdapter adapts JobHistoryService to the processing.JobHistoryRecorder interface
type jobHistoryAdapter struct {
	service *JobHistoryService
}

func (a *jobHistoryAdapter) RecordJobStart(jobID string, videoID uint, videoTitle string, phase string) {
	a.service.RecordJobStart(jobID, videoID, videoTitle, phase)
}

func (a *jobHistoryAdapter) RecordJobStartWithRetry(jobID string, videoID uint, videoTitle string, phase string, maxRetries int, retryCount int) {
	a.service.RecordJobStartWithRetry(jobID, videoID, videoTitle, phase, maxRetries, retryCount)
}

func (a *jobHistoryAdapter) RecordJobComplete(jobID string) {
	a.service.RecordJobComplete(jobID)
}

func (a *jobHistoryAdapter) RecordJobCancelled(jobID string) {
	a.service.RecordJobCancelled(jobID)
}

func (a *jobHistoryAdapter) RecordJobFailedWithRetry(jobID string, videoID uint, phase string, err error) {
	a.service.RecordJobFailedWithRetry(jobID, videoID, phase, err)
}

// VideoProcessingService orchestrates video processing using worker pools
type VideoProcessingService struct {
	poolManager   *processing.PoolManager
	phaseTracker  *processing.PhaseTracker
	resultHandler *processing.ResultHandler
	jobSubmitter  *processing.JobSubmitter
	logger        *zap.Logger
}

// NewVideoProcessingService creates a new VideoProcessingService
func NewVideoProcessingService(
	repo data.VideoRepository,
	cfg config.ProcessingConfig,
	logger *zap.Logger,
	eventBus *EventBus,
	jobHistory *JobHistoryService,
	poolConfigRepo data.PoolConfigRepository,
	processingConfigRepo data.ProcessingConfigRepository,
	triggerConfigRepo data.TriggerConfigRepository,
) *VideoProcessingService {
	// Create pool manager
	poolManager := processing.NewPoolManager(cfg, logger, poolConfigRepo, processingConfigRepo)

	// Create phase tracker
	phaseTracker := processing.NewPhaseTracker(triggerConfigRepo)
	if triggerConfigRepo != nil {
		if err := phaseTracker.RefreshTriggerCache(); err != nil {
			logger.Error("Failed to load trigger config cache", zap.Error(err))
		}
	}

	// Create adapters
	eventAdapter := &eventBusAdapter{eventBus: eventBus}
	var historyAdapter processing.JobHistoryRecorder
	if jobHistory != nil {
		historyAdapter = &jobHistoryAdapter{service: jobHistory}
	}

	// Create result handler
	resultHandler := processing.NewResultHandler(repo, eventAdapter, historyAdapter, phaseTracker, poolManager, logger)

	// Create job submitter
	jobSubmitter := processing.NewJobSubmitter(repo, poolManager, phaseTracker, historyAdapter, logger)

	// Wire up the result handler callback for phase completion
	resultHandler.SetOnPhaseComplete(func(videoID uint, phase string) error {
		return jobSubmitter.SubmitPhase(videoID, phase)
	})

	// Set the pool manager's result handler
	poolManager.SetResultHandler(resultHandler.ProcessPoolResults)

	return &VideoProcessingService{
		poolManager:   poolManager,
		phaseTracker:  phaseTracker,
		resultHandler: resultHandler,
		jobSubmitter:  jobSubmitter,
		logger:        logger,
	}
}

// SetIndexer sets the video indexer for search index updates
func (s *VideoProcessingService) SetIndexer(indexer VideoIndexer) {
	s.resultHandler.SetIndexer(indexer)
}

// Start starts all worker pools
func (s *VideoProcessingService) Start() {
	s.poolManager.Start()
	s.logger.Info("Video processing service started")
}

// Stop stops all worker pools
func (s *VideoProcessingService) Stop() {
	s.logger.Info("Stopping video processing service")
	s.poolManager.Stop()
}

// SubmitVideo submits a new video for processing
func (s *VideoProcessingService) SubmitVideo(videoID uint, videoPath string) error {
	return s.jobSubmitter.SubmitVideo(videoID, videoPath)
}

// SubmitPhase submits a specific phase for a video
func (s *VideoProcessingService) SubmitPhase(videoID uint, phase string) error {
	return s.jobSubmitter.SubmitPhase(videoID, phase)
}

// SubmitPhaseWithRetry submits a phase for processing with retry tracking
func (s *VideoProcessingService) SubmitPhaseWithRetry(videoID uint, phase string, retryCount, maxRetries int) error {
	return s.jobSubmitter.SubmitPhaseWithRetry(videoID, phase, retryCount, maxRetries)
}

// SubmitBulkPhase submits a processing phase for multiple videos
func (s *VideoProcessingService) SubmitBulkPhase(phase string, mode string) (*BulkPhaseResult, error) {
	return s.jobSubmitter.SubmitBulkPhase(phase, mode)
}

// CancelJob cancels a running job by its ID
func (s *VideoProcessingService) CancelJob(jobID string) error {
	return s.poolManager.CancelJob(jobID)
}

// GetJob retrieves a job by its ID from any pool
func (s *VideoProcessingService) GetJob(jobID string) (jobs.Job, bool) {
	return s.poolManager.GetJob(jobID)
}

// GetPoolConfig returns the current pool configuration
func (s *VideoProcessingService) GetPoolConfig() PoolConfig {
	return s.poolManager.GetPoolConfig()
}

// GetQueueStatus returns the current queue status
func (s *VideoProcessingService) GetQueueStatus() QueueStatus {
	return s.poolManager.GetQueueStatus()
}

// UpdatePoolConfig updates the pool configuration
func (s *VideoProcessingService) UpdatePoolConfig(cfg PoolConfig) error {
	return s.poolManager.UpdatePoolConfig(cfg)
}

// GetProcessingQualityConfig returns the current quality configuration
func (s *VideoProcessingService) GetProcessingQualityConfig() ProcessingQualityConfig {
	return s.poolManager.GetQualityConfig()
}

// UpdateProcessingQualityConfig updates the quality configuration
func (s *VideoProcessingService) UpdateProcessingQualityConfig(cfg ProcessingQualityConfig) error {
	return s.poolManager.UpdateQualityConfig(cfg)
}

// RefreshTriggerCache reloads the trigger configuration from the database
func (s *VideoProcessingService) RefreshTriggerCache() error {
	return s.phaseTracker.RefreshTriggerCache()
}

// LogStatus logs the status of all pools
func (s *VideoProcessingService) LogStatus() {
	s.logger.Info("Video processing service status")
	s.poolManager.LogStatus()
}
