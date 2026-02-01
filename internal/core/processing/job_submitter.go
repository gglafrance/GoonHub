package processing

import (
	"fmt"
	"goonhub/internal/data"
	"goonhub/internal/jobs"
	"goonhub/pkg/ffmpeg"

	"go.uber.org/zap"
)

// JobSubmitter handles job submission to worker pools
type JobSubmitter struct {
	repo         data.SceneRepository
	poolManager  *PoolManager
	phaseTracker *PhaseTracker
	jobHistory   JobHistoryRecorder
	logger       *zap.Logger
}

// NewJobSubmitter creates a new JobSubmitter
func NewJobSubmitter(
	repo data.SceneRepository,
	poolManager *PoolManager,
	phaseTracker *PhaseTracker,
	jobHistory JobHistoryRecorder,
	logger *zap.Logger,
) *JobSubmitter {
	return &JobSubmitter{
		repo:         repo,
		poolManager:  poolManager,
		phaseTracker: phaseTracker,
		jobHistory:   jobHistory,
		logger:       logger,
	}
}

// SubmitScene submits a new scene for processing (metadata extraction)
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

	qualityConfig := js.poolManager.GetQualityConfig()

	job := jobs.NewMetadataJob(
		sceneID,
		scenePath,
		qualityConfig.MaxFrameDimensionSm,
		qualityConfig.MaxFrameDimensionLg,
		js.repo,
		js.logger,
	)

	err := js.poolManager.SubmitToMetadataPool(job)
	if err != nil {
		if jobs.IsDuplicateJobError(err) {
			js.logger.Info("Duplicate metadata job skipped",
				zap.Uint("scene_id", sceneID),
				zap.Error(err),
			)
			return nil
		}
		js.logger.Error("Failed to submit metadata job",
			zap.Uint("scene_id", sceneID),
			zap.Error(err),
		)
		return err
	}

	if js.jobHistory != nil {
		sceneTitle := ""
		if s, err := js.repo.GetByID(sceneID); err == nil {
			sceneTitle = s.Title
		}
		js.jobHistory.RecordJobStart(job.GetID(), sceneID, sceneTitle, "metadata")
	}

	return nil
}

// SubmitPhase submits a specific phase for a scene
func (js *JobSubmitter) SubmitPhase(sceneID uint, phase string) error {
	return js.SubmitPhaseWithRetry(sceneID, phase, 0, 0)
}

// SubmitPhaseWithRetry submits a phase for processing with retry tracking.
// retryCount is the current retry attempt (0 for first attempt).
// maxRetries is the maximum number of retries allowed (0 uses default from config).
func (js *JobSubmitter) SubmitPhaseWithRetry(sceneID uint, phase string, retryCount, maxRetries int) error {
	scene, err := js.repo.GetByID(sceneID)
	if err != nil {
		return fmt.Errorf("failed to get scene: %w", err)
	}

	qualityConfig := js.poolManager.GetQualityConfig()
	cfg := js.poolManager.GetConfig()

	// Helper to record job start with or without retry info
	recordJobStart := func(jobID string, phase string) {
		if js.jobHistory == nil {
			return
		}
		if retryCount > 0 || maxRetries > 0 {
			js.jobHistory.RecordJobStartWithRetry(jobID, sceneID, scene.Title, phase, maxRetries, retryCount)
		} else {
			js.jobHistory.RecordJobStart(jobID, sceneID, scene.Title, phase)
		}
	}

	switch phase {
	case "metadata":
		job := jobs.NewMetadataJob(
			sceneID, scene.StoredPath,
			qualityConfig.MaxFrameDimensionSm, qualityConfig.MaxFrameDimensionLg,
			js.repo, js.logger,
		)
		err = js.poolManager.SubmitToMetadataPool(job)
		if err != nil {
			if jobs.IsDuplicateJobError(err) {
				js.logger.Info("Duplicate metadata job skipped", zap.Uint("scene_id", sceneID))
				return nil
			}
			return fmt.Errorf("failed to submit metadata job: %w", err)
		}
		recordJobStart(job.GetID(), "metadata")

	case "thumbnail":
		if scene.Duration == 0 {
			return fmt.Errorf("metadata must be extracted before thumbnail generation")
		}
		js.logger.Info("SubmitPhase: Creating thumbnail job",
			zap.Uint("scene_id", sceneID),
			zap.Uint("scene_db_id", scene.ID),
			zap.String("scene_stored_path", scene.StoredPath),
			zap.String("scene_title", scene.Title),
		)
		tileWidthLg, tileHeightLg := ffmpeg.CalculateTileDimensions(scene.Width, scene.Height, cfg.MaxFrameDimensionLarge)
		thumbnailJob := jobs.NewThumbnailJob(
			sceneID, scene.StoredPath, cfg.ThumbnailDir,
			scene.ThumbnailWidth, scene.ThumbnailHeight,
			tileWidthLg, tileHeightLg,
			scene.Duration, qualityConfig.FrameQualitySm, qualityConfig.FrameQualityLg,
			js.repo, js.logger,
		)
		err = js.poolManager.SubmitToThumbnailPool(thumbnailJob)
		if err != nil {
			if jobs.IsDuplicateJobError(err) {
				js.logger.Info("Duplicate thumbnail job skipped", zap.Uint("scene_id", sceneID))
				return nil
			}
			return fmt.Errorf("failed to submit thumbnail job: %w", err)
		}
		recordJobStart(thumbnailJob.GetID(), "thumbnail")

	case "sprites":
		if scene.Duration == 0 {
			return fmt.Errorf("metadata must be extracted before sprite generation")
		}
		spritesJob := jobs.NewSpritesJob(
			sceneID, scene.StoredPath, cfg.SpriteDir, cfg.VttDir,
			scene.ThumbnailWidth, scene.ThumbnailHeight, scene.Duration,
			cfg.FrameInterval, qualityConfig.FrameQualitySprites, cfg.GridCols, cfg.GridRows,
			qualityConfig.SpritesConcurrency, js.repo, js.logger,
		)
		err = js.poolManager.SubmitToSpritesPool(spritesJob)
		if err != nil {
			if jobs.IsDuplicateJobError(err) {
				js.logger.Info("Duplicate sprites job skipped", zap.Uint("scene_id", sceneID))
				return nil
			}
			return fmt.Errorf("failed to submit sprites job: %w", err)
		}
		recordJobStart(spritesJob.GetID(), "sprites")

	default:
		return fmt.Errorf("unknown phase: %s", phase)
	}

	js.logger.Info("Phase submitted",
		zap.Uint("scene_id", sceneID),
		zap.String("phase", phase),
		zap.Int("retry_count", retryCount),
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
