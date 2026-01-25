package processing

import (
	"fmt"
	"goonhub/internal/data"
	"goonhub/internal/jobs"

	"go.uber.org/zap"
)

// ResultHandler processes job results from worker pools
type ResultHandler struct {
	repo         data.VideoRepository
	eventBus     EventPublisher
	jobHistory   JobHistoryRecorder
	phaseTracker *PhaseTracker
	poolManager  *PoolManager
	indexer      VideoIndexer
	logger       *zap.Logger

	// onPhaseComplete is called when a phase completes to submit follow-up phases
	onPhaseComplete func(videoID uint, phase string) error
}

// NewResultHandler creates a new ResultHandler
func NewResultHandler(
	repo data.VideoRepository,
	eventBus EventPublisher,
	jobHistory JobHistoryRecorder,
	phaseTracker *PhaseTracker,
	poolManager *PoolManager,
	logger *zap.Logger,
) *ResultHandler {
	return &ResultHandler{
		repo:         repo,
		eventBus:     eventBus,
		jobHistory:   jobHistory,
		phaseTracker: phaseTracker,
		poolManager:  poolManager,
		logger:       logger,
	}
}

// SetIndexer sets the video indexer for search index updates
func (rh *ResultHandler) SetIndexer(indexer VideoIndexer) {
	rh.indexer = indexer
}

// SetOnPhaseComplete sets the callback for phase completion
func (rh *ResultHandler) SetOnPhaseComplete(fn func(videoID uint, phase string) error) {
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
		zap.Uint("video_id", result.VideoID),
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
		rh.logger.Error("Invalid metadata job result data", zap.Uint("video_id", result.VideoID))
		return
	}

	meta := metadataJob.GetResult()
	if meta == nil {
		rh.logger.Error("Metadata result is nil", zap.Uint("video_id", result.VideoID))
		return
	}

	// Re-index video after metadata extraction (duration/resolution now available)
	if rh.indexer != nil {
		if video, err := rh.repo.GetByID(result.VideoID); err == nil {
			if err := rh.indexer.UpdateVideoIndex(video); err != nil {
				rh.logger.Warn("Failed to update video in search index after metadata",
					zap.Uint("video_id", result.VideoID),
					zap.Error(err),
				)
			}
		}
	}

	rh.eventBus.Publish(VideoEvent{
		Type:    "video:metadata_complete",
		VideoID: result.VideoID,
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
			zap.Uint("video_id", result.VideoID),
		)
		rh.checkAndMarkComplete(result.VideoID, "metadata")
		return
	}

	// Initialize phase tracking for this video
	rh.phaseTracker.InitPhaseState(result.VideoID)

	// Retrieve the video path from the metadata job
	videoPath := metadataJob.GetVideoPath()

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
			zap.Uint("result_video_id", result.VideoID),
			zap.Uint("metadata_job_video_id", metadataJob.GetVideoID()),
			zap.String("video_path", videoPath),
		)
		thumbnailJob = jobs.NewThumbnailJob(
			result.VideoID,
			videoPath,
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
		)

		thumbnailErr := rh.poolManager.SubmitToThumbnailPool(thumbnailJob)
		if thumbnailErr != nil {
			if jobs.IsDuplicateJobError(thumbnailErr) {
				rh.logger.Info("Duplicate thumbnail job skipped",
					zap.Uint("video_id", result.VideoID),
				)
				thumbnailJob = nil
			} else {
				rh.logger.Error("Failed to submit thumbnail job",
					zap.Uint("video_id", result.VideoID),
					zap.Error(thumbnailErr),
				)
				rh.repo.UpdateProcessingStatus(result.VideoID, "failed", "failed to submit thumbnail job")
				return
			}
		}
	}

	if submitSprites {
		spritesJob = jobs.NewSpritesJob(
			result.VideoID,
			videoPath,
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

		spritesErr := rh.poolManager.SubmitToSpritesPool(spritesJob)
		if spritesErr != nil {
			if jobs.IsDuplicateJobError(spritesErr) {
				rh.logger.Info("Duplicate sprites job skipped",
					zap.Uint("video_id", result.VideoID),
				)
				spritesJob = nil
			} else {
				rh.logger.Error("Failed to submit sprites job",
					zap.Uint("video_id", result.VideoID),
					zap.Error(spritesErr),
				)
				rh.repo.UpdateProcessingStatus(result.VideoID, "failed", "failed to submit sprites job")
				return
			}
		}
	}

	if rh.jobHistory != nil {
		videoTitle := ""
		if v, err := rh.repo.GetByID(result.VideoID); err == nil {
			videoTitle = v.Title
		}
		if thumbnailJob != nil {
			rh.jobHistory.RecordJobStart(thumbnailJob.GetID(), result.VideoID, videoTitle, "thumbnail")
		}
		if spritesJob != nil {
			rh.jobHistory.RecordJobStart(spritesJob.GetID(), result.VideoID, videoTitle, "sprites")
		}
	}

	rh.logger.Info("Submitted trigger-based jobs after metadata",
		zap.Uint("video_id", result.VideoID),
		zap.Bool("thumbnail", submitThumbnail),
		zap.Bool("sprites", submitSprites),
	)
}

func (rh *ResultHandler) onThumbnailComplete(result jobs.JobResult) {
	thumbnailJob, ok := result.Data.(*jobs.ThumbnailJob)
	if ok {
		thumbResult := thumbnailJob.GetResult()
		if thumbResult != nil {
			rh.eventBus.Publish(VideoEvent{
				Type:    "video:thumbnail_complete",
				VideoID: result.VideoID,
				Data: map[string]any{
					"thumbnail_path": thumbResult.ThumbnailPath,
				},
			})
		}
	}

	// Trigger any phases configured to run after thumbnail
	for _, phase := range rh.phaseTracker.GetPhasesTriggeredAfter("thumbnail") {
		if rh.onPhaseComplete != nil {
			if err := rh.onPhaseComplete(result.VideoID, phase); err != nil {
				rh.logger.Error("Failed to submit phase after thumbnail",
					zap.Uint("video_id", result.VideoID),
					zap.String("phase", phase),
					zap.Error(err),
				)
			}
		}
	}

	rh.phaseTracker.MarkPhaseComplete(result.VideoID, "thumbnail")
	rh.checkAndMarkComplete(result.VideoID, "thumbnail")
}

func (rh *ResultHandler) onSpritesComplete(result jobs.JobResult) {
	spritesJob, ok := result.Data.(*jobs.SpritesJob)
	if ok {
		spritesResult := spritesJob.GetResult()
		if spritesResult != nil {
			rh.eventBus.Publish(VideoEvent{
				Type:    "video:sprites_complete",
				VideoID: result.VideoID,
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
			if err := rh.onPhaseComplete(result.VideoID, phase); err != nil {
				rh.logger.Error("Failed to submit phase after sprites",
					zap.Uint("video_id", result.VideoID),
					zap.String("phase", phase),
					zap.Error(err),
				)
			}
		}
	}

	rh.phaseTracker.MarkPhaseComplete(result.VideoID, "sprites")
	rh.checkAndMarkComplete(result.VideoID, "sprites")
}

func (rh *ResultHandler) checkAndMarkComplete(videoID uint, completedPhase string) {
	if rh.phaseTracker.CheckAllPhasesComplete(videoID, completedPhase) {
		if err := rh.repo.UpdateProcessingStatus(videoID, "completed", ""); err != nil {
			rh.logger.Error("Failed to update processing status to completed",
				zap.Uint("video_id", videoID),
				zap.Error(err),
			)
			return
		}

		rh.eventBus.Publish(VideoEvent{
			Type:    "video:completed",
			VideoID: videoID,
		})

		rh.logger.Info("All processing phases completed for video",
			zap.Uint("video_id", videoID),
		)
	}
}

func (rh *ResultHandler) handleFailed(result jobs.JobResult) {
	rh.logger.Error("Job phase failed",
		zap.String("job_id", result.JobID),
		zap.String("phase", result.Phase),
		zap.Uint("video_id", result.VideoID),
		zap.Error(result.Error),
	)

	if rh.jobHistory != nil && result.Error != nil {
		rh.jobHistory.RecordJobFailedWithRetry(result.JobID, result.VideoID, result.Phase, result.Error)
	}

	rh.phaseTracker.ClearPhaseState(result.VideoID)

	rh.eventBus.Publish(VideoEvent{
		Type:    "video:failed",
		VideoID: result.VideoID,
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
		zap.Uint("video_id", result.VideoID),
	)

	if rh.jobHistory != nil {
		rh.jobHistory.RecordJobCancelled(result.JobID)
	}

	rh.phaseTracker.ClearPhaseState(result.VideoID)

	rh.eventBus.Publish(VideoEvent{
		Type:    "video:cancelled",
		VideoID: result.VideoID,
		Data: map[string]any{
			"phase": result.Phase,
		},
	})
}

func (rh *ResultHandler) handleTimedOut(result jobs.JobResult) {
	rh.logger.Error("Job timed out",
		zap.String("job_id", result.JobID),
		zap.String("phase", result.Phase),
		zap.Uint("video_id", result.VideoID),
	)

	if rh.jobHistory != nil {
		timeoutErr := fmt.Errorf("job timed out")
		rh.jobHistory.RecordJobFailedWithRetry(result.JobID, result.VideoID, result.Phase, timeoutErr)
	}

	rh.phaseTracker.ClearPhaseState(result.VideoID)

	rh.eventBus.Publish(VideoEvent{
		Type:    "video:timed_out",
		VideoID: result.VideoID,
		Data: map[string]any{
			"phase": result.Phase,
		},
	})
}
