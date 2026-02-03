package processing

import (
	"fmt"
	"goonhub/internal/data"
	"goonhub/internal/jobs"

	"go.uber.org/zap"
)

// ResultHandler processes job results from worker pools
type ResultHandler struct {
	repo           data.SceneRepository
	markerThumbGen jobs.MarkerThumbnailGenerator
	eventBus       EventPublisher
	jobHistory     JobHistoryRecorder
	phaseTracker   *PhaseTracker
	poolManager    *PoolManager
	indexer        SceneIndexer
	logger         *zap.Logger

	// onPhaseComplete is called when a phase completes to submit follow-up phases
	onPhaseComplete func(sceneID uint, phase string) error
}

// NewResultHandler creates a new ResultHandler
func NewResultHandler(
	repo data.SceneRepository,
	markerThumbGen jobs.MarkerThumbnailGenerator,
	eventBus EventPublisher,
	jobHistory JobHistoryRecorder,
	phaseTracker *PhaseTracker,
	poolManager *PoolManager,
	logger *zap.Logger,
) *ResultHandler {
	return &ResultHandler{
		repo:           repo,
		markerThumbGen: markerThumbGen,
		eventBus:       eventBus,
		jobHistory:     jobHistory,
		phaseTracker:   phaseTracker,
		poolManager:    poolManager,
		logger:         logger,
	}
}

// SetIndexer sets the scene indexer for search index updates
func (rh *ResultHandler) SetIndexer(indexer SceneIndexer) {
	rh.indexer = indexer
}

// SetOnPhaseComplete sets the callback for phase completion
func (rh *ResultHandler) SetOnPhaseComplete(fn func(sceneID uint, phase string) error) {
	rh.onPhaseComplete = fn
}

// ProcessPoolResults processes results from a worker pool
func (rh *ResultHandler) ProcessPoolResults(pool *jobs.WorkerPool) {
	for result := range pool.Results() {
		switch result.Status {
		case jobs.JobStatusCompleted:
			rh.handleCompleted(result)
		case jobs.JobStatusFailed:
			rh.handleFailed(result)
		case jobs.JobStatusCancelled:
			rh.handleCancelled(result)
		case jobs.JobStatusTimedOut:
			rh.handleTimedOut(result)
		}
	}
}

func (rh *ResultHandler) handleCompleted(result jobs.JobResult) {
	rh.logger.Info("Job phase completed",
		zap.String("job_id", result.JobID),
		zap.String("phase", result.Phase),
		zap.Uint("scene_id", result.SceneID),
	)

	if rh.jobHistory != nil {
		rh.jobHistory.RecordJobComplete(result.JobID)
	}

	switch result.Phase {
	case "metadata":
		rh.onMetadataComplete(result)
	case "thumbnail":
		rh.onThumbnailComplete(result)
	case "sprites":
		rh.onSpritesComplete(result)
	}
}

func (rh *ResultHandler) onMetadataComplete(result jobs.JobResult) {
	metadataJob, ok := result.Data.(*jobs.MetadataJob)
	if !ok {
		rh.logger.Error("Invalid metadata job result data", zap.Uint("scene_id", result.SceneID))
		return
	}

	meta := metadataJob.GetResult()
	if meta == nil {
		rh.logger.Error("Metadata result is nil", zap.Uint("scene_id", result.SceneID))
		return
	}

	// Re-index scene after metadata extraction (duration/resolution now available)
	if rh.indexer != nil {
		if scene, err := rh.repo.GetByID(result.SceneID); err == nil {
			if err := rh.indexer.UpdateSceneIndex(scene); err != nil {
				rh.logger.Warn("Failed to update scene in search index after metadata",
					zap.Uint("scene_id", result.SceneID),
					zap.Error(err),
				)
			}
		}
	}

	rh.eventBus.Publish(SceneEvent{
		Type:    "scene:metadata_complete",
		SceneID: result.SceneID,
		Data: map[string]any{
			"duration": meta.Duration,
			"width":    meta.Width,
			"height":   meta.Height,
		},
	})

	// Determine which phases should be triggered after metadata
	phasesToTrigger := rh.phaseTracker.GetPhasesTriggeredAfter("metadata")

	// If no triggers configured, nothing follows metadata automatically
	if len(phasesToTrigger) == 0 {
		rh.logger.Info("No phases configured to trigger after metadata",
			zap.Uint("scene_id", result.SceneID),
		)
		rh.checkAndMarkComplete(result.SceneID, "metadata")
		return
	}

	// Initialize phase tracking for this scene
	rh.phaseTracker.InitPhaseState(result.SceneID)

	// Retrieve the scene path from the metadata job
	scenePath := metadataJob.GetScenePath()

	// Read runtime quality config
	qualityConfig := rh.poolManager.GetQualityConfig()
	cfg := rh.poolManager.GetConfig()

	submitThumbnail := false
	submitSprites := false
	for _, phase := range phasesToTrigger {
		if phase == "thumbnail" {
			submitThumbnail = true
		}
		if phase == "sprites" {
			submitSprites = true
		}
	}

	var thumbnailJob *jobs.ThumbnailJob
	var spritesJob *jobs.SpritesJob

	if submitThumbnail {
		rh.logger.Info("Creating thumbnail job from metadata result",
			zap.Uint("result_scene_id", result.SceneID),
			zap.Uint("metadata_job_scene_id", metadataJob.GetSceneID()),
			zap.String("scene_path", scenePath),
		)
		thumbnailJob = jobs.NewThumbnailJob(
			result.SceneID,
			scenePath,
			cfg.ThumbnailDir,
			meta.TileWidth,
			meta.TileHeight,
			meta.TileWidthLarge,
			meta.TileHeightLarge,
			meta.Duration,
			qualityConfig.FrameQualitySm,
			qualityConfig.FrameQualityLg,
			rh.repo,
			rh.logger,
			rh.markerThumbGen,
		)

		thumbnailErr := rh.poolManager.SubmitToThumbnailPool(thumbnailJob)
		if thumbnailErr != nil {
			if jobs.IsDuplicateJobError(thumbnailErr) {
				rh.logger.Info("Duplicate thumbnail job skipped",
					zap.Uint("scene_id", result.SceneID),
				)
				thumbnailJob = nil
			} else {
				rh.logger.Error("Failed to submit thumbnail job",
					zap.Uint("scene_id", result.SceneID),
					zap.Error(thumbnailErr),
				)
				rh.repo.UpdateProcessingStatus(result.SceneID, "failed", "failed to submit thumbnail job")
				return
			}
		}
	}

	if submitSprites {
		spritesJob = jobs.NewSpritesJob(
			result.SceneID,
			scenePath,
			cfg.SpriteDir,
			cfg.VttDir,
			meta.TileWidth,
			meta.TileHeight,
			meta.Duration,
			cfg.FrameInterval,
			qualityConfig.FrameQualitySprites,
			cfg.GridCols,
			cfg.GridRows,
			qualityConfig.SpritesConcurrency,
			rh.repo,
			rh.logger,
		)
		if rh.jobHistory != nil {
			jh := rh.jobHistory
			spritesJob.SetProgressCallback(func(jobID string, progress int) {
				jh.UpdateProgress(jobID, progress)
			})
		}

		spritesErr := rh.poolManager.SubmitToSpritesPool(spritesJob)
		if spritesErr != nil {
			if jobs.IsDuplicateJobError(spritesErr) {
				rh.logger.Info("Duplicate sprites job skipped",
					zap.Uint("scene_id", result.SceneID),
				)
				spritesJob = nil
			} else {
				rh.logger.Error("Failed to submit sprites job",
					zap.Uint("scene_id", result.SceneID),
					zap.Error(spritesErr),
				)
				rh.repo.UpdateProcessingStatus(result.SceneID, "failed", "failed to submit sprites job")
				return
			}
		}
	}

	if rh.jobHistory != nil {
		sceneTitle := ""
		if s, err := rh.repo.GetByID(result.SceneID); err == nil {
			sceneTitle = s.Title
		}
		if thumbnailJob != nil {
			rh.jobHistory.RecordJobStart(thumbnailJob.GetID(), result.SceneID, sceneTitle, "thumbnail")
		}
		if spritesJob != nil {
			rh.jobHistory.RecordJobStart(spritesJob.GetID(), result.SceneID, sceneTitle, "sprites")
		}
	}

	rh.logger.Info("Submitted trigger-based jobs after metadata",
		zap.Uint("scene_id", result.SceneID),
		zap.Bool("thumbnail", submitThumbnail),
		zap.Bool("sprites", submitSprites),
	)
}

func (rh *ResultHandler) onThumbnailComplete(result jobs.JobResult) {
	thumbnailJob, ok := result.Data.(*jobs.ThumbnailJob)
	if ok {
		thumbResult := thumbnailJob.GetResult()
		if thumbResult != nil {
			rh.eventBus.Publish(SceneEvent{
				Type:    "scene:thumbnail_complete",
				SceneID: result.SceneID,
				Data: map[string]any{
					"thumbnail_path": thumbResult.ThumbnailPath,
				},
			})
		}
	}

	// Trigger any phases configured to run after thumbnail
	for _, phase := range rh.phaseTracker.GetPhasesTriggeredAfter("thumbnail") {
		if rh.onPhaseComplete != nil {
			if err := rh.onPhaseComplete(result.SceneID, phase); err != nil {
				rh.logger.Error("Failed to submit phase after thumbnail",
					zap.Uint("scene_id", result.SceneID),
					zap.String("phase", phase),
					zap.Error(err),
				)
			}
		}
	}

	rh.phaseTracker.MarkPhaseComplete(result.SceneID, "thumbnail")
	rh.checkAndMarkComplete(result.SceneID, "thumbnail")
}

func (rh *ResultHandler) onSpritesComplete(result jobs.JobResult) {
	spritesJob, ok := result.Data.(*jobs.SpritesJob)
	if ok {
		spritesResult := spritesJob.GetResult()
		if spritesResult != nil {
			rh.eventBus.Publish(SceneEvent{
				Type:    "scene:sprites_complete",
				SceneID: result.SceneID,
				Data: map[string]any{
					"vtt_path":          spritesResult.VttPath,
					"sprite_sheet_path": spritesResult.SpriteSheetPath,
				},
			})
		}
	}

	// Trigger any phases configured to run after sprites
	for _, phase := range rh.phaseTracker.GetPhasesTriggeredAfter("sprites") {
		if rh.onPhaseComplete != nil {
			if err := rh.onPhaseComplete(result.SceneID, phase); err != nil {
				rh.logger.Error("Failed to submit phase after sprites",
					zap.Uint("scene_id", result.SceneID),
					zap.String("phase", phase),
					zap.Error(err),
				)
			}
		}
	}

	rh.phaseTracker.MarkPhaseComplete(result.SceneID, "sprites")
	rh.checkAndMarkComplete(result.SceneID, "sprites")
}

func (rh *ResultHandler) checkAndMarkComplete(sceneID uint, completedPhase string) {
	if rh.phaseTracker.CheckAllPhasesComplete(sceneID, completedPhase) {
		if err := rh.repo.UpdateProcessingStatus(sceneID, "completed", ""); err != nil {
			rh.logger.Error("Failed to update processing status to completed",
				zap.Uint("scene_id", sceneID),
				zap.Error(err),
			)
			return
		}

		rh.eventBus.Publish(SceneEvent{
			Type:    "scene:completed",
			SceneID: sceneID,
		})

		rh.logger.Info("All processing phases completed for scene",
			zap.Uint("scene_id", sceneID),
		)
	}
}

func (rh *ResultHandler) handleFailed(result jobs.JobResult) {
	rh.logger.Error("Job phase failed",
		zap.String("job_id", result.JobID),
		zap.String("phase", result.Phase),
		zap.Uint("scene_id", result.SceneID),
		zap.Error(result.Error),
	)

	if rh.jobHistory != nil && result.Error != nil {
		rh.jobHistory.RecordJobFailedWithRetry(result.JobID, result.SceneID, result.Phase, result.Error)
	}

	rh.phaseTracker.ClearPhaseState(result.SceneID)

	rh.eventBus.Publish(SceneEvent{
		Type:    "scene:failed",
		SceneID: result.SceneID,
		Data: map[string]any{
			"error": result.Error.Error(),
			"phase": result.Phase,
		},
	})
}

func (rh *ResultHandler) handleCancelled(result jobs.JobResult) {
	rh.logger.Warn("Job cancelled",
		zap.String("job_id", result.JobID),
		zap.String("phase", result.Phase),
		zap.Uint("scene_id", result.SceneID),
	)

	if rh.jobHistory != nil {
		rh.jobHistory.RecordJobCancelled(result.JobID)
	}

	rh.phaseTracker.ClearPhaseState(result.SceneID)

	rh.eventBus.Publish(SceneEvent{
		Type:    "scene:cancelled",
		SceneID: result.SceneID,
		Data: map[string]any{
			"phase": result.Phase,
		},
	})
}

func (rh *ResultHandler) handleTimedOut(result jobs.JobResult) {
	rh.logger.Error("Job timed out",
		zap.String("job_id", result.JobID),
		zap.String("phase", result.Phase),
		zap.Uint("scene_id", result.SceneID),
	)

	if rh.jobHistory != nil {
		timeoutErr := fmt.Errorf("job timed out")
		rh.jobHistory.RecordJobFailedWithRetry(result.JobID, result.SceneID, result.Phase, timeoutErr)
	}

	rh.phaseTracker.ClearPhaseState(result.SceneID)

	rh.eventBus.Publish(SceneEvent{
		Type:    "scene:timed_out",
		SceneID: result.SceneID,
		Data: map[string]any{
			"phase": result.Phase,
		},
	})
}
