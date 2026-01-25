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
	repo         data.VideoRepository
	poolManager  *PoolManager
	phaseTracker *PhaseTracker
	jobHistory   JobHistoryRecorder
	logger       *zap.Logger
}

// NewJobSubmitter creates a new JobSubmitter
func NewJobSubmitter(
	repo data.VideoRepository,
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

// SubmitVideo submits a new video for processing (metadata extraction)
func (js *JobSubmitter) SubmitVideo(videoID uint, videoPath string) error {
	js.logger.Info("Video submitted for processing",
		zap.Uint("video_id", videoID),
		zap.String("video_path", videoPath),
	)

	// Check if metadata trigger is on_import
	metaTrigger := js.phaseTracker.GetTriggerForPhase("metadata")
	if metaTrigger != nil && metaTrigger.TriggerType != "on_import" {
		js.logger.Info("Metadata trigger is not on_import, skipping auto-submit",
			zap.Uint("video_id", videoID),
			zap.String("trigger_type", metaTrigger.TriggerType),
		)
		return nil
	}

	qualityConfig := js.poolManager.GetQualityConfig()

	job := jobs.NewMetadataJob(
		videoID,
		videoPath,
		qualityConfig.MaxFrameDimensionSm,
		qualityConfig.MaxFrameDimensionLg,
		js.repo,
		js.logger,
	)

	err := js.poolManager.SubmitToMetadataPool(job)
	if err != nil {
		if jobs.IsDuplicateJobError(err) {
			js.logger.Info("Duplicate metadata job skipped",
				zap.Uint("video_id", videoID),
				zap.Error(err),
			)
			return nil
		}
		js.logger.Error("Failed to submit metadata job",
			zap.Uint("video_id", videoID),
			zap.Error(err),
		)
		return err
	}

	if js.jobHistory != nil {
		videoTitle := ""
		if v, err := js.repo.GetByID(videoID); err == nil {
			videoTitle = v.Title
		}
		js.jobHistory.RecordJobStart(job.GetID(), videoID, videoTitle, "metadata")
	}

	return nil
}

// SubmitPhase submits a specific phase for a video
func (js *JobSubmitter) SubmitPhase(videoID uint, phase string) error {
	return js.SubmitPhaseWithRetry(videoID, phase, 0, 0)
}

// SubmitPhaseWithRetry submits a phase for processing with retry tracking.
// retryCount is the current retry attempt (0 for first attempt).
// maxRetries is the maximum number of retries allowed (0 uses default from config).
func (js *JobSubmitter) SubmitPhaseWithRetry(videoID uint, phase string, retryCount, maxRetries int) error {
	video, err := js.repo.GetByID(videoID)
	if err != nil {
		return fmt.Errorf("failed to get video: %w", err)
	}

	qualityConfig := js.poolManager.GetQualityConfig()
	cfg := js.poolManager.GetConfig()

	// Helper to record job start with or without retry info
	recordJobStart := func(jobID string, phase string) {
		if js.jobHistory == nil {
			return
		}
		if retryCount > 0 || maxRetries > 0 {
			js.jobHistory.RecordJobStartWithRetry(jobID, videoID, video.Title, phase, maxRetries, retryCount)
		} else {
			js.jobHistory.RecordJobStart(jobID, videoID, video.Title, phase)
		}
	}

	switch phase {
	case "metadata":
		job := jobs.NewMetadataJob(
			videoID, video.StoredPath,
			qualityConfig.MaxFrameDimensionSm, qualityConfig.MaxFrameDimensionLg,
			js.repo, js.logger,
		)
		err = js.poolManager.SubmitToMetadataPool(job)
		if err != nil {
			if jobs.IsDuplicateJobError(err) {
				js.logger.Info("Duplicate metadata job skipped", zap.Uint("video_id", videoID))
				return nil
			}
			return fmt.Errorf("failed to submit metadata job: %w", err)
		}
		recordJobStart(job.GetID(), "metadata")

	case "thumbnail":
		if video.Duration == 0 {
			return fmt.Errorf("metadata must be extracted before thumbnail generation")
		}
		js.logger.Info("SubmitPhase: Creating thumbnail job",
			zap.Uint("video_id", videoID),
			zap.Uint("video_db_id", video.ID),
			zap.String("video_stored_path", video.StoredPath),
			zap.String("video_title", video.Title),
		)
		tileWidthLg, tileHeightLg := ffmpeg.CalculateTileDimensions(video.Width, video.Height, cfg.MaxFrameDimensionLarge)
		thumbnailJob := jobs.NewThumbnailJob(
			videoID, video.StoredPath, cfg.ThumbnailDir,
			video.ThumbnailWidth, video.ThumbnailHeight,
			tileWidthLg, tileHeightLg,
			video.Duration, qualityConfig.FrameQualitySm, qualityConfig.FrameQualityLg,
			js.repo, js.logger,
		)
		err = js.poolManager.SubmitToThumbnailPool(thumbnailJob)
		if err != nil {
			if jobs.IsDuplicateJobError(err) {
				js.logger.Info("Duplicate thumbnail job skipped", zap.Uint("video_id", videoID))
				return nil
			}
			return fmt.Errorf("failed to submit thumbnail job: %w", err)
		}
		recordJobStart(thumbnailJob.GetID(), "thumbnail")

	case "sprites":
		if video.Duration == 0 {
			return fmt.Errorf("metadata must be extracted before sprite generation")
		}
		spritesJob := jobs.NewSpritesJob(
			videoID, video.StoredPath, cfg.SpriteDir, cfg.VttDir,
			video.ThumbnailWidth, video.ThumbnailHeight, video.Duration,
			cfg.FrameInterval, qualityConfig.FrameQualitySprites, cfg.GridCols, cfg.GridRows,
			qualityConfig.SpritesConcurrency, js.repo, js.logger,
		)
		err = js.poolManager.SubmitToSpritesPool(spritesJob)
		if err != nil {
			if jobs.IsDuplicateJobError(err) {
				js.logger.Info("Duplicate sprites job skipped", zap.Uint("video_id", videoID))
				return nil
			}
			return fmt.Errorf("failed to submit sprites job: %w", err)
		}
		recordJobStart(spritesJob.GetID(), "sprites")

	default:
		return fmt.Errorf("unknown phase: %s", phase)
	}

	js.logger.Info("Phase submitted",
		zap.Uint("video_id", videoID),
		zap.String("phase", phase),
		zap.Int("retry_count", retryCount),
	)
	return nil
}

// SubmitBulkPhase submits a processing phase for multiple videos
// mode can be "missing" (only videos needing the phase) or "all" (all videos)
func (js *JobSubmitter) SubmitBulkPhase(phase string, mode string) (*BulkPhaseResult, error) {
	var videos []data.Video
	var err error

	if mode == "all" {
		videos, err = js.repo.GetAll()
		if err != nil {
			return nil, fmt.Errorf("failed to get videos: %w", err)
		}
	} else {
		// Default to "missing" mode
		videos, err = js.repo.GetVideosNeedingPhase(phase)
		if err != nil {
			return nil, fmt.Errorf("failed to get videos needing %s: %w", phase, err)
		}
	}

	result := &BulkPhaseResult{}

	for _, video := range videos {
		// For thumbnail/sprites in "all" mode, skip videos without metadata
		if mode == "all" && (phase == "thumbnail" || phase == "sprites") && video.Duration == 0 {
			result.Skipped++
			continue
		}

		if err := js.SubmitPhase(video.ID, phase); err != nil {
			js.logger.Warn("Failed to submit bulk phase job",
				zap.Uint("video_id", video.ID),
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
