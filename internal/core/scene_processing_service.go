package core

import (
	"goonhub/internal/config"
	"goonhub/internal/core/matching"
	"goonhub/internal/core/processing"
	"goonhub/internal/data"
	"goonhub/internal/jobs"
	"time"

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

func (a *eventBusAdapter) Publish(event processing.SceneEvent) {
	a.eventBus.Publish(SceneEvent{
		Type:    event.Type,
		SceneID: event.SceneID,
		Data:    event.Data,
	})
}

// jobHistoryAdapter adapts JobHistoryService to the processing.JobQueueRecorder interface
type jobHistoryAdapter struct {
	service *JobHistoryService
}

func (a *jobHistoryAdapter) RecordJobStart(jobID string, sceneID uint, sceneTitle string, phase string) {
	a.service.RecordJobStart(jobID, sceneID, sceneTitle, phase)
}

func (a *jobHistoryAdapter) RecordJobStartWithRetry(jobID string, sceneID uint, sceneTitle string, phase string, maxRetries int, retryCount int) {
	a.service.RecordJobStartWithRetry(jobID, sceneID, sceneTitle, phase, maxRetries, retryCount)
}

func (a *jobHistoryAdapter) RecordJobComplete(jobID string) {
	a.service.RecordJobComplete(jobID)
}

func (a *jobHistoryAdapter) RecordJobCancelled(jobID string) {
	a.service.RecordJobCancelled(jobID)
}

func (a *jobHistoryAdapter) RecordJobFailedWithRetry(jobID string, sceneID uint, phase string, err error) {
	a.service.RecordJobFailedWithRetry(jobID, sceneID, phase, err)
}

func (a *jobHistoryAdapter) UpdateProgress(jobID string, progress int) {
	a.service.UpdateProgress(jobID, progress)
}

func (a *jobHistoryAdapter) CreatePendingJob(jobID string, sceneID uint, sceneTitle string, phase string, forceTarget string) error {
	return a.service.CreatePendingJob(jobID, sceneID, sceneTitle, phase, forceTarget)
}

func (a *jobHistoryAdapter) CreatePendingJobWithPriority(jobID string, sceneID uint, sceneTitle string, phase string, priority int, forceTarget string) error {
	return a.service.CreatePendingJobWithPriority(jobID, sceneID, sceneTitle, phase, priority, forceTarget)
}

func (a *jobHistoryAdapter) CreatePendingJobWithRetry(jobID string, sceneID uint, sceneTitle string, phase string, retryCount, maxRetries int, forceTarget string) error {
	return a.service.CreatePendingJobWithRetry(jobID, sceneID, sceneTitle, phase, retryCount, maxRetries, forceTarget)
}

func (a *jobHistoryAdapter) ExistsPendingOrRunning(sceneID uint, phase string) (bool, error) {
	return a.service.ExistsPendingOrRunning(sceneID, phase)
}

// matchingServiceAdapter adapts *MatchingService to the processing.MatchingService interface
type matchingServiceAdapter struct {
	ms *MatchingService
}

func (a *matchingServiceAdapter) IndexFingerprint(sceneID uint, fpType string, audioFP []int32, visualFP []uint64) error {
	return a.ms.IndexFingerprint(sceneID, fpType, audioFP, visualFP)
}

func (a *matchingServiceAdapter) FindMatches(sceneID uint, fpType string, audioFP []int32, visualFP []uint64) ([]processing.MatchResult, error) {
	results, err := a.ms.FindMatches(sceneID, fpType, audioFP, visualFP)
	if err != nil {
		return nil, err
	}
	out := make([]processing.MatchResult, len(results))
	for i, r := range results {
		out[i] = processing.MatchResult{
			SceneID:         r.SceneID,
			ConfidenceScore: r.ConfidenceScore,
			MatchType:       r.MatchType,
		}
	}
	return out, nil
}

func (a *matchingServiceAdapter) ProcessMatches(sceneID uint, matches []processing.MatchResult) error {
	converted := make([]matching.MatchResult, len(matches))
	for i, m := range matches {
		converted[i] = matching.MatchResult{
			SceneID:         m.SceneID,
			ConfidenceScore: m.ConfidenceScore,
			MatchType:       m.MatchType,
		}
	}
	return a.ms.ProcessMatches(sceneID, converted)
}

// SceneProcessingService orchestrates scene processing using worker pools
type SceneProcessingService struct {
	poolManager   *processing.PoolManager
	phaseTracker  *processing.PhaseTracker
	resultHandler *processing.ResultHandler
	jobSubmitter  *processing.JobSubmitter
	jobHistory    *JobHistoryService
	logger        *zap.Logger
}

// NewSceneProcessingService creates a new SceneProcessingService
func NewSceneProcessingService(
	repo data.SceneRepository,
	markerThumbGen jobs.MarkerThumbnailGenerator,
	cfg config.ProcessingConfig,
	logger *zap.Logger,
	eventBus *EventBus,
	jobHistory *JobHistoryService,
	poolConfigRepo data.PoolConfigRepository,
	processingConfigRepo data.ProcessingConfigRepository,
	triggerConfigRepo data.TriggerConfigRepository,
	duplicationCfg *config.DuplicationConfig,
) *SceneProcessingService {
	// Create pool manager
	poolManager := processing.NewPoolManager(cfg, logger, poolConfigRepo, processingConfigRepo, duplicationCfg)

	// Create phase tracker
	phaseTracker := processing.NewPhaseTracker(triggerConfigRepo)
	if triggerConfigRepo != nil {
		if err := phaseTracker.RefreshTriggerCache(); err != nil {
			logger.Error("Failed to load trigger config cache", zap.Error(err))
		}
	}

	// Create adapters
	eventAdapter := &eventBusAdapter{eventBus: eventBus}
	var historyAdapter processing.JobQueueRecorder
	if jobHistory != nil {
		historyAdapter = &jobHistoryAdapter{service: jobHistory}
	}

	// Create result handler
	resultHandler := processing.NewResultHandler(repo, markerThumbGen, eventAdapter, historyAdapter, phaseTracker, poolManager, logger)

	// Create job submitter
	jobSubmitter := processing.NewJobSubmitter(repo, poolManager, phaseTracker, historyAdapter, logger)

	// Wire up the result handler callback for phase completion
	resultHandler.SetOnPhaseComplete(func(sceneID uint, phase string) error {
		return jobSubmitter.SubmitPhase(sceneID, phase)
	})

	// Set the pool manager's result handler
	poolManager.SetResultHandler(resultHandler.ProcessPoolResults)

	return &SceneProcessingService{
		poolManager:   poolManager,
		phaseTracker:  phaseTracker,
		resultHandler: resultHandler,
		jobSubmitter:  jobSubmitter,
		jobHistory:    jobHistory,
		logger:        logger,
	}
}

// SetIndexer sets the scene indexer for search index updates
func (s *SceneProcessingService) SetIndexer(indexer SceneIndexer) {
	s.resultHandler.SetIndexer(indexer)
}

// SetMatchingService sets the matching service for fingerprint matching.
// Accepts *MatchingService and wraps it to satisfy the processing.MatchingService interface.
func (s *SceneProcessingService) SetMatchingService(ms *MatchingService) {
	if ms == nil {
		return
	}
	s.resultHandler.SetMatchingService(&matchingServiceAdapter{ms: ms})
}

// IsDuplicationEnabled returns whether duplication detection is enabled
func (s *SceneProcessingService) IsDuplicationEnabled() bool {
	return s.poolManager.IsDuplicationEnabled()
}

// Start starts all worker pools
func (s *SceneProcessingService) Start() {
	s.poolManager.Start()
	s.logger.Info("Scene processing service started")
}

// Stop stops all worker pools
func (s *SceneProcessingService) Stop() {
	s.logger.Info("Stopping scene processing service")
	s.poolManager.Stop()
}

// GracefulStop performs graceful shutdown of all worker pools.
// It waits for in-flight jobs to complete (up to timeout) and returns
// a map of phase -> buffered job IDs that were never executed.
func (s *SceneProcessingService) GracefulStop(timeout time.Duration) map[string][]string {
	s.logger.Info("Gracefully stopping scene processing service", zap.Duration("timeout", timeout))
	return s.poolManager.GracefulStop(timeout)
}

// SubmitScene submits a new scene for processing
func (s *SceneProcessingService) SubmitScene(sceneID uint, scenePath string) error {
	return s.jobSubmitter.SubmitScene(sceneID, scenePath)
}

// SubmitPhase submits a specific phase for a scene
func (s *SceneProcessingService) SubmitPhase(sceneID uint, phase string) error {
	return s.jobSubmitter.SubmitPhase(sceneID, phase)
}

// SubmitPhaseWithPriority submits a phase with a specific priority (higher = processed first).
// Used for manual triggers and DLQ retries.
func (s *SceneProcessingService) SubmitPhaseWithPriority(sceneID uint, phase string, priority int) error {
	return s.jobSubmitter.SubmitPhaseWithPriority(sceneID, phase, priority)
}

// SubmitPhaseWithForce submits a phase with priority and an optional force target.
// Used for manual per-scene triggers where force regeneration is requested.
func (s *SceneProcessingService) SubmitPhaseWithForce(sceneID uint, phase string, priority int, forceTarget string) error {
	return s.jobSubmitter.SubmitPhaseWithForce(sceneID, phase, priority, forceTarget)
}

// SubmitPhaseWithRetry submits a phase for processing with retry tracking
func (s *SceneProcessingService) SubmitPhaseWithRetry(sceneID uint, phase string, retryCount, maxRetries int) error {
	return s.jobSubmitter.SubmitPhaseWithRetry(sceneID, phase, retryCount, maxRetries)
}

// SubmitBulkPhase submits a processing phase for multiple scenes.
// sceneIDs optionally scopes the operation to specific scenes (nil = all scenes).
func (s *SceneProcessingService) SubmitBulkPhase(phase string, mode string, forceTarget string, sceneIDs []uint) (*BulkPhaseResult, error) {
	return s.jobSubmitter.SubmitBulkPhase(phase, mode, forceTarget, sceneIDs)
}

// CancelJob cancels a job by its ID.
// First attempts to cancel in the worker pool (running/queued jobs).
// Falls back to cancelling a pending job directly in the database.
func (s *SceneProcessingService) CancelJob(jobID string) error {
	err := s.poolManager.CancelJob(jobID)
	if err == nil {
		return nil
	}
	// Fallback: cancel pending job directly in DB
	if s.jobHistory != nil {
		return s.jobHistory.CancelPendingJob(jobID)
	}
	return err
}

// GetJob retrieves a job by its ID from any pool
func (s *SceneProcessingService) GetJob(jobID string) (jobs.Job, bool) {
	return s.poolManager.GetJob(jobID)
}

// GetPoolConfig returns the current pool configuration
func (s *SceneProcessingService) GetPoolConfig() PoolConfig {
	return s.poolManager.GetPoolConfig()
}

// GetQueueStatus returns the current queue status
func (s *SceneProcessingService) GetQueueStatus() QueueStatus {
	return s.poolManager.GetQueueStatus()
}

// UpdatePoolConfig updates the pool configuration
func (s *SceneProcessingService) UpdatePoolConfig(cfg PoolConfig) error {
	return s.poolManager.UpdatePoolConfig(cfg)
}

// GetProcessingQualityConfig returns the current quality configuration
func (s *SceneProcessingService) GetProcessingQualityConfig() ProcessingQualityConfig {
	return s.poolManager.GetQualityConfig()
}

// UpdateProcessingQualityConfig updates the quality configuration
func (s *SceneProcessingService) UpdateProcessingQualityConfig(cfg ProcessingQualityConfig) error {
	return s.poolManager.UpdateQualityConfig(cfg)
}

// RefreshTriggerCache reloads the trigger configuration from the database
func (s *SceneProcessingService) RefreshTriggerCache() error {
	return s.phaseTracker.RefreshTriggerCache()
}

// LogStatus logs the status of all pools
func (s *SceneProcessingService) LogStatus() {
	s.logger.Info("Scene processing service status")
	s.poolManager.LogStatus()
}

// GetPoolManager returns the underlying pool manager.
// Used by JobQueueFeeder to submit jobs directly to pools.
func (s *SceneProcessingService) GetPoolManager() *processing.PoolManager {
	return s.poolManager
}
