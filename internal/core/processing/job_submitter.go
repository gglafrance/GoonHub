package processing

import (
	"fmt"
	"goonhub/internal/data"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// JobSubmitter handles job submission to worker pools.
// With DB-backed queue, jobs are created as 'pending' in the database
// and later claimed by the JobQueueFeeder for execution.
type JobSubmitter struct {
	repo         data.SceneRepository
	poolManager  *PoolManager
	phaseTracker *PhaseTracker
	jobQueue     JobQueueRecorder
	logger       *zap.Logger
}

// NewJobSubmitter creates a new JobSubmitter
func NewJobSubmitter(
	repo data.SceneRepository,
	poolManager *PoolManager,
	phaseTracker *PhaseTracker,
	jobQueue JobQueueRecorder,
	logger *zap.Logger,
) *JobSubmitter {
	return &JobSubmitter{
		repo:         repo,
		poolManager:  poolManager,
		phaseTracker: phaseTracker,
		jobQueue:     jobQueue,
		logger:       logger,
	}
}

// SubmitScene submits a new scene for processing (metadata extraction).
// Creates a pending job in the database; the JobQueueFeeder will pick it up.
func (js *JobSubmitter) SubmitScene(sceneID uint, scenePath string) error {
	js.logger.Info("Scene submitted for processing",
		zap.Uint("scene_id", sceneID),
		zap.String("scene_path", scenePath),
	)

	// Check if metadata trigger is on_import
	metaTrigger := js.phaseTracker.GetTriggerForPhase("metadata")
	if metaTrigger != nil && metaTrigger.TriggerType != "on_import" {
		js.logger.Info("Metadata trigger is not on_import, skipping auto-submit",
			zap.Uint("scene_id", sceneID),
			zap.String("trigger_type", metaTrigger.TriggerType),
		)
		return nil
	}

	return js.createPendingJob(sceneID, "metadata")
}

// SubmitPhase submits a specific phase for a scene.
// Creates a pending job in the database; the JobQueueFeeder will pick it up.
func (js *JobSubmitter) SubmitPhase(sceneID uint, phase string) error {
	return js.SubmitPhaseWithRetry(sceneID, phase, 0, 0)
}

// SubmitPhaseWithRetry submits a phase for processing with retry tracking.
// Creates a pending job in the database; the JobQueueFeeder will pick it up.
// retryCount is the current retry attempt (0 for first attempt).
// maxRetries is the maximum number of retries allowed (0 uses default from config).
func (js *JobSubmitter) SubmitPhaseWithRetry(sceneID uint, phase string, retryCount, maxRetries int) error {
	// Validate the phase
	switch phase {
	case "metadata", "thumbnail", "sprites":
		// Valid phases
	default:
		return fmt.Errorf("unknown phase: %s", phase)
	}

	// For thumbnail/sprites, check if metadata is available
	if phase == "thumbnail" || phase == "sprites" {
		scene, err := js.repo.GetByID(sceneID)
		if err != nil {
			return fmt.Errorf("failed to get scene: %w", err)
		}
		if scene.Duration == 0 {
			return fmt.Errorf("metadata must be extracted before %s generation", phase)
		}
	}

	return js.createPendingJob(sceneID, phase)
}

// createPendingJob creates a pending job in the database.
// This is the core of the DB-backed queue approach.
func (js *JobSubmitter) createPendingJob(sceneID uint, phase string) error {
	if js.jobQueue == nil {
		return fmt.Errorf("job queue recorder not configured")
	}

	// Check for deduplication: skip if there's already a pending or running job
	exists, err := js.jobQueue.ExistsPendingOrRunning(sceneID, phase)
	if err != nil {
		js.logger.Error("Failed to check for existing job",
			zap.Uint("scene_id", sceneID),
			zap.String("phase", phase),
			zap.Error(err),
		)
		return fmt.Errorf("failed to check for existing job: %w", err)
	}
	if exists {
		js.logger.Debug("Job already pending or running, skipping",
			zap.Uint("scene_id", sceneID),
			zap.String("phase", phase),
		)
		return nil
	}

	// Get scene title for the job record
	sceneTitle := ""
	if s, err := js.repo.GetByID(sceneID); err == nil {
		sceneTitle = s.Title
	}

	// Generate a new job ID
	jobID := uuid.New().String()

	// Create the pending job in the database
	if err := js.jobQueue.CreatePendingJob(jobID, sceneID, sceneTitle, phase); err != nil {
		js.logger.Error("Failed to create pending job",
			zap.String("job_id", jobID),
			zap.Uint("scene_id", sceneID),
			zap.String("phase", phase),
			zap.Error(err),
		)
		return fmt.Errorf("failed to create pending job: %w", err)
	}

	js.logger.Info("Pending job created",
		zap.String("job_id", jobID),
		zap.Uint("scene_id", sceneID),
		zap.String("phase", phase),
	)
	return nil
}

// SubmitBulkPhase submits a processing phase for multiple scenes
// mode can be "missing" (only scenes needing the phase) or "all" (all scenes)
func (js *JobSubmitter) SubmitBulkPhase(phase string, mode string) (*BulkPhaseResult, error) {
	var scenes []data.Scene
	var err error

	if mode == "all" {
		scenes, err = js.repo.GetAll()
		if err != nil {
			return nil, fmt.Errorf("failed to get scenes: %w", err)
		}
	} else {
		// Default to "missing" mode
		scenes, err = js.repo.GetScenesNeedingPhase(phase)
		if err != nil {
			return nil, fmt.Errorf("failed to get scenes needing %s: %w", phase, err)
		}
	}

	result := &BulkPhaseResult{}

	for _, scene := range scenes {
		// For thumbnail/sprites in "all" mode, skip scenes without metadata
		if mode == "all" && (phase == "thumbnail" || phase == "sprites") && scene.Duration == 0 {
			result.Skipped++
			continue
		}

		if err := js.SubmitPhase(scene.ID, phase); err != nil {
			js.logger.Warn("Failed to submit bulk phase job",
				zap.Uint("scene_id", scene.ID),
				zap.String("phase", phase),
				zap.Error(err),
			)
			result.Errors++
		} else {
			result.Submitted++
		}
	}

	js.logger.Info("Bulk phase submission completed",
		zap.String("phase", phase),
		zap.String("mode", mode),
		zap.Int("submitted", result.Submitted),
		zap.Int("skipped", result.Skipped),
		zap.Int("errors", result.Errors),
	)

	return result, nil
}
