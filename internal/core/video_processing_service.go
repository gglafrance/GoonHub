package core

import (
	"fmt"
	"goonhub/internal/config"
	"goonhub/internal/data"
	"goonhub/internal/jobs"
	"goonhub/pkg/ffmpeg"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"go.uber.org/zap"
)

type PoolConfig struct {
	MetadataWorkers  int `json:"metadata_workers"`
	ThumbnailWorkers int `json:"thumbnail_workers"`
	SpritesWorkers   int `json:"sprites_workers"`
}

type ProcessingQualityConfig struct {
	MaxFrameDimensionSm int `json:"max_frame_dimension_sm"`
	MaxFrameDimensionLg int `json:"max_frame_dimension_lg"`
	FrameQualitySm      int `json:"frame_quality_sm"`
	FrameQualityLg      int `json:"frame_quality_lg"`
	FrameQualitySprites int `json:"frame_quality_sprites"`
	SpritesConcurrency  int `json:"sprites_concurrency"`
}

type QueueStatus struct {
	MetadataQueued  int `json:"metadata_queued"`
	ThumbnailQueued int `json:"thumbnail_queued"`
	SpritesQueued   int `json:"sprites_queued"`
}

type phaseState struct {
	thumbnailDone bool
	spritesDone   bool
}

type VideoProcessingService struct {
	metadataPool            *jobs.WorkerPool
	thumbnailPool           *jobs.WorkerPool
	spritesPool             *jobs.WorkerPool
	poolMu                  sync.RWMutex
	repo                    data.VideoRepository
	config                  config.ProcessingConfig
	processingQualityConfig ProcessingQualityConfig
	logger                  *zap.Logger
	eventBus                *EventBus
	jobHistory              *JobHistoryService
	phases                  sync.Map // map[uint]*phaseState
	triggerConfigRepo       data.TriggerConfigRepository
	triggerCache            []data.TriggerConfigRecord
	triggerCacheMu          sync.RWMutex
	indexer                 VideoIndexer
}

// SetIndexer sets the video indexer for search index updates.
func (s *VideoProcessingService) SetIndexer(indexer VideoIndexer) {
	s.indexer = indexer
}

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
	// Check DB for persisted pool config overrides
	metadataWorkers := cfg.MetadataWorkers
	thumbnailWorkers := cfg.ThumbnailWorkers
	spritesWorkers := cfg.SpritesWorkers

	if poolConfigRepo != nil {
		if dbConfig, err := poolConfigRepo.Get(); err == nil && dbConfig != nil {
			metadataWorkers = dbConfig.MetadataWorkers
			thumbnailWorkers = dbConfig.ThumbnailWorkers
			spritesWorkers = dbConfig.SpritesWorkers
			logger.Info("Loaded pool config from database",
				zap.Int("metadata_workers", metadataWorkers),
				zap.Int("thumbnail_workers", thumbnailWorkers),
				zap.Int("sprites_workers", spritesWorkers),
			)
		}
	}

	// Initialize processing quality config from YAML defaults
	qualityConfig := ProcessingQualityConfig{
		MaxFrameDimensionSm: cfg.MaxFrameDimension,
		MaxFrameDimensionLg: cfg.MaxFrameDimensionLarge,
		FrameQualitySm:      cfg.FrameQuality,
		FrameQualityLg:      cfg.FrameQualityLg,
		FrameQualitySprites: cfg.FrameQualitySprites,
		SpritesConcurrency:  cfg.SpritesConcurrency,
	}

	// Override with DB-persisted processing config if available
	if processingConfigRepo != nil {
		if dbConfig, err := processingConfigRepo.Get(); err == nil && dbConfig != nil {
			qualityConfig.MaxFrameDimensionSm = dbConfig.MaxFrameDimensionSm
			qualityConfig.MaxFrameDimensionLg = dbConfig.MaxFrameDimensionLg
			qualityConfig.FrameQualitySm = dbConfig.FrameQualitySm
			qualityConfig.FrameQualityLg = dbConfig.FrameQualityLg
			qualityConfig.FrameQualitySprites = dbConfig.FrameQualitySprites
			qualityConfig.SpritesConcurrency = dbConfig.SpritesConcurrency
			logger.Info("Loaded processing config from database",
				zap.Int("max_frame_dimension_sm", qualityConfig.MaxFrameDimensionSm),
				zap.Int("max_frame_dimension_lg", qualityConfig.MaxFrameDimensionLg),
				zap.Int("frame_quality_sm", qualityConfig.FrameQualitySm),
				zap.Int("frame_quality_lg", qualityConfig.FrameQualityLg),
				zap.Int("frame_quality_sprites", qualityConfig.FrameQualitySprites),
				zap.Int("sprites_concurrency", qualityConfig.SpritesConcurrency),
			)
		}
	}

	logger.Info("Initializing video processing service",
		zap.Int("metadata_workers", metadataWorkers),
		zap.Int("thumbnail_workers", thumbnailWorkers),
		zap.Int("sprites_workers", spritesWorkers),
		zap.Int("frame_interval", cfg.FrameInterval),
		zap.Int("max_frame_dimension_sm", qualityConfig.MaxFrameDimensionSm),
		zap.Int("max_frame_dimension_lg", qualityConfig.MaxFrameDimensionLg),
		zap.Int("frame_quality_sm", qualityConfig.FrameQualitySm),
		zap.Int("frame_quality_lg", qualityConfig.FrameQualityLg),
		zap.Int("frame_quality_sprites", qualityConfig.FrameQualitySprites),
		zap.Int("grid_cols", cfg.GridCols),
		zap.Int("grid_rows", cfg.GridRows),
		zap.String("sprite_dir", cfg.SpriteDir),
		zap.String("vtt_dir", cfg.VttDir),
		zap.String("thumbnail_dir", cfg.ThumbnailDir),
	)

	metadataPool := jobs.NewWorkerPool(metadataWorkers, 100)
	metadataPool.SetLogger(logger.With(zap.String("pool", "metadata")))
	if cfg.MetadataTimeout > 0 {
		metadataPool.SetTimeout(cfg.MetadataTimeout)
		logger.Info("Metadata pool timeout set", zap.Duration("timeout", cfg.MetadataTimeout))
	}

	thumbnailPool := jobs.NewWorkerPool(thumbnailWorkers, 100)
	thumbnailPool.SetLogger(logger.With(zap.String("pool", "thumbnail")))
	if cfg.ThumbnailTimeout > 0 {
		thumbnailPool.SetTimeout(cfg.ThumbnailTimeout)
		logger.Info("Thumbnail pool timeout set", zap.Duration("timeout", cfg.ThumbnailTimeout))
	}

	spritesPool := jobs.NewWorkerPool(spritesWorkers, 100)
	spritesPool.SetLogger(logger.With(zap.String("pool", "sprites")))
	if cfg.SpritesTimeout > 0 {
		spritesPool.SetTimeout(cfg.SpritesTimeout)
		logger.Info("Sprites pool timeout set", zap.Duration("timeout", cfg.SpritesTimeout))
	}

	if err := os.MkdirAll(cfg.SpriteDir, 0755); err != nil {
		logger.Error("Failed to create sprite directory",
			zap.String("directory", cfg.SpriteDir),
			zap.Error(err),
		)
	} else {
		logger.Info("Sprite directory ready", zap.String("directory", cfg.SpriteDir))
	}

	if err := os.MkdirAll(cfg.VttDir, 0755); err != nil {
		logger.Error("Failed to create VTT directory",
			zap.String("directory", cfg.VttDir),
			zap.Error(err),
		)
	} else {
		logger.Info("VTT directory ready", zap.String("directory", cfg.VttDir))
	}

	if err := os.MkdirAll(cfg.ThumbnailDir, 0755); err != nil {
		logger.Error("Failed to create thumbnail directory",
			zap.String("directory", cfg.ThumbnailDir),
			zap.Error(err),
		)
	} else {
		logger.Info("Thumbnail directory ready", zap.String("directory", cfg.ThumbnailDir))
	}

	svc := &VideoProcessingService{
		metadataPool:            metadataPool,
		thumbnailPool:           thumbnailPool,
		spritesPool:             spritesPool,
		repo:                    repo,
		config:                  cfg,
		processingQualityConfig: qualityConfig,
		logger:                  logger,
		eventBus:                eventBus,
		jobHistory:              jobHistory,
		triggerConfigRepo:       triggerConfigRepo,
	}

	// Load trigger config cache
	if triggerConfigRepo != nil {
		if err := svc.RefreshTriggerCache(); err != nil {
			logger.Error("Failed to load trigger config cache", zap.Error(err))
		}
	}

	return svc
}

func (s *VideoProcessingService) Start() {
	s.migrateOldThumbnails()

	s.metadataPool.Start()
	s.thumbnailPool.Start()
	s.spritesPool.Start()

	go s.processPoolResults(s.metadataPool)
	go s.processPoolResults(s.thumbnailPool)
	go s.processPoolResults(s.spritesPool)

	s.logger.Info("Video processing service started",
		zap.Int("metadata_workers", s.metadataPool.ActiveWorkers()),
		zap.Int("thumbnail_workers", s.thumbnailPool.ActiveWorkers()),
		zap.Int("sprites_workers", s.spritesPool.ActiveWorkers()),
	)
}

// migrateOldThumbnails renames legacy {id}_thumb.webp files to the new {id}_thumb_sm.webp naming.
func (s *VideoProcessingService) migrateOldThumbnails() {
	entries, err := os.ReadDir(s.config.ThumbnailDir)
	if err != nil {
		// Directory might not exist yet on first run
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, "_thumb.webp") {
			oldPath := filepath.Join(s.config.ThumbnailDir, name)
			newName := strings.TrimSuffix(name, "_thumb.webp") + "_thumb_sm.webp"
			newPath := filepath.Join(s.config.ThumbnailDir, newName)
			if err := os.Rename(oldPath, newPath); err != nil {
				s.logger.Error("Failed to migrate old thumbnail",
					zap.String("old_path", oldPath),
					zap.String("new_path", newPath),
					zap.Error(err),
				)
			} else {
				s.logger.Info("Migrated old thumbnail",
					zap.String("old_path", oldPath),
					zap.String("new_path", newPath),
				)
			}
		}
	}
}

func (s *VideoProcessingService) SubmitVideo(videoID uint, videoPath string) error {
	s.logger.Info("Video submitted for processing",
		zap.Uint("video_id", videoID),
		zap.String("video_path", videoPath),
	)

	// Check if metadata trigger is on_import
	metaTrigger := s.getTriggerForPhase("metadata")
	if metaTrigger != nil && metaTrigger.TriggerType != "on_import" {
		s.logger.Info("Metadata trigger is not on_import, skipping auto-submit",
			zap.Uint("video_id", videoID),
			zap.String("trigger_type", metaTrigger.TriggerType),
		)
		return nil
	}

	s.poolMu.RLock()
	dimSm := s.processingQualityConfig.MaxFrameDimensionSm
	dimLg := s.processingQualityConfig.MaxFrameDimensionLg
	s.poolMu.RUnlock()

	job := jobs.NewMetadataJob(
		videoID,
		videoPath,
		dimSm,
		dimLg,
		s.repo,
		s.logger,
	)

	s.poolMu.RLock()
	err := s.metadataPool.Submit(job)
	s.poolMu.RUnlock()

	if err != nil {
		// Handle duplicate job gracefully - not an error
		if jobs.IsDuplicateJobError(err) {
			s.logger.Info("Duplicate metadata job skipped",
				zap.Uint("video_id", videoID),
				zap.Error(err),
			)
			return nil
		}
		s.logger.Error("Failed to submit metadata job",
			zap.Uint("video_id", videoID),
			zap.Error(err),
		)
		return err
	}

	if s.jobHistory != nil {
		videoTitle := ""
		if v, err := s.repo.GetByID(videoID); err == nil {
			videoTitle = v.Title
		}
		s.jobHistory.RecordJobStart(job.GetID(), videoID, videoTitle, "metadata")
	}

	return nil
}

func (s *VideoProcessingService) processPoolResults(pool *jobs.WorkerPool) {
	for result := range pool.Results() {
		switch result.Status {
		case jobs.JobStatusCompleted:
			s.handleCompleted(result)
		case jobs.JobStatusFailed:
			s.handleFailed(result)
		case jobs.JobStatusCancelled:
			s.handleCancelled(result)
		case jobs.JobStatusTimedOut:
			s.handleTimedOut(result)
		}
	}
}

func (s *VideoProcessingService) handleCompleted(result jobs.JobResult) {
	s.logger.Info("Job phase completed",
		zap.String("job_id", result.JobID),
		zap.String("phase", result.Phase),
		zap.Uint("video_id", result.VideoID),
	)

	if s.jobHistory != nil {
		s.jobHistory.RecordJobComplete(result.JobID)
	}

	switch result.Phase {
	case "metadata":
		s.onMetadataComplete(result)
	case "thumbnail":
		s.onThumbnailComplete(result)
	case "sprites":
		s.onSpritesComplete(result)
	}
}

func (s *VideoProcessingService) onMetadataComplete(result jobs.JobResult) {
	metadataJob, ok := result.Data.(*jobs.MetadataJob)
	if !ok {
		s.logger.Error("Invalid metadata job result data", zap.Uint("video_id", result.VideoID))
		return
	}

	meta := metadataJob.GetResult()
	if meta == nil {
		s.logger.Error("Metadata result is nil", zap.Uint("video_id", result.VideoID))
		return
	}

	// Re-index video after metadata extraction (duration/resolution now available)
	if s.indexer != nil {
		if video, err := s.repo.GetByID(result.VideoID); err == nil {
			if err := s.indexer.UpdateVideoIndex(video); err != nil {
				s.logger.Warn("Failed to update video in search index after metadata",
					zap.Uint("video_id", result.VideoID),
					zap.Error(err),
				)
			}
		}
	}

	s.eventBus.Publish(VideoEvent{
		Type:    "video:metadata_complete",
		VideoID: result.VideoID,
		Data: map[string]any{
			"duration": meta.Duration,
			"width":    meta.Width,
			"height":   meta.Height,
		},
	})

	// Determine which phases should be triggered after metadata
	phasesToTrigger := s.getPhasesTriggeredAfter("metadata")

	// If no triggers configured, nothing follows metadata automatically
	if len(phasesToTrigger) == 0 {
		s.logger.Info("No phases configured to trigger after metadata",
			zap.Uint("video_id", result.VideoID),
		)
		// Check if video is complete (no auto phases follow)
		s.checkAllPhasesComplete(result.VideoID, "metadata")
		return
	}

	// Initialize phase tracking for this video
	s.phases.Store(result.VideoID, &phaseState{})

	// Retrieve the video path from the metadata job
	videoPath := metadataJob.GetVideoPath()

	// Read runtime quality config
	s.poolMu.RLock()
	qualitySm := s.processingQualityConfig.FrameQualitySm
	qualityLg := s.processingQualityConfig.FrameQualityLg
	qualitySprites := s.processingQualityConfig.FrameQualitySprites
	spritesConcurrency := s.processingQualityConfig.SpritesConcurrency
	s.poolMu.RUnlock()

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
		s.logger.Info("Creating thumbnail job from metadata result",
			zap.Uint("result_video_id", result.VideoID),
			zap.Uint("metadata_job_video_id", metadataJob.GetVideoID()),
			zap.String("video_path", videoPath),
		)
		thumbnailJob = jobs.NewThumbnailJob(
			result.VideoID,
			videoPath,
			s.config.ThumbnailDir,
			meta.TileWidth,
			meta.TileHeight,
			meta.TileWidthLarge,
			meta.TileHeightLarge,
			meta.Duration,
			qualitySm,
			qualityLg,
			s.repo,
			s.logger,
		)

		s.poolMu.RLock()
		thumbnailErr := s.thumbnailPool.Submit(thumbnailJob)
		s.poolMu.RUnlock()

		if thumbnailErr != nil {
			if jobs.IsDuplicateJobError(thumbnailErr) {
				s.logger.Info("Duplicate thumbnail job skipped",
					zap.Uint("video_id", result.VideoID),
				)
				thumbnailJob = nil // Don't record in history
			} else {
				s.logger.Error("Failed to submit thumbnail job",
					zap.Uint("video_id", result.VideoID),
					zap.Error(thumbnailErr),
				)
				s.repo.UpdateProcessingStatus(result.VideoID, "failed", "failed to submit thumbnail job")
				return
			}
		}
	}

	if submitSprites {
		spritesJob = jobs.NewSpritesJob(
			result.VideoID,
			videoPath,
			s.config.SpriteDir,
			s.config.VttDir,
			meta.TileWidth,
			meta.TileHeight,
			meta.Duration,
			s.config.FrameInterval,
			qualitySprites,
			s.config.GridCols,
			s.config.GridRows,
			spritesConcurrency,
			s.repo,
			s.logger,
		)

		s.poolMu.RLock()
		spritesErr := s.spritesPool.Submit(spritesJob)
		s.poolMu.RUnlock()

		if spritesErr != nil {
			if jobs.IsDuplicateJobError(spritesErr) {
				s.logger.Info("Duplicate sprites job skipped",
					zap.Uint("video_id", result.VideoID),
				)
				spritesJob = nil // Don't record in history
			} else {
				s.logger.Error("Failed to submit sprites job",
					zap.Uint("video_id", result.VideoID),
					zap.Error(spritesErr),
				)
				s.repo.UpdateProcessingStatus(result.VideoID, "failed", "failed to submit sprites job")
				return
			}
		}
	}

	if s.jobHistory != nil {
		videoTitle := ""
		if v, err := s.repo.GetByID(result.VideoID); err == nil {
			videoTitle = v.Title
		}
		if thumbnailJob != nil {
			s.jobHistory.RecordJobStart(thumbnailJob.GetID(), result.VideoID, videoTitle, "thumbnail")
		}
		if spritesJob != nil {
			s.jobHistory.RecordJobStart(spritesJob.GetID(), result.VideoID, videoTitle, "sprites")
		}
	}

	s.logger.Info("Submitted trigger-based jobs after metadata",
		zap.Uint("video_id", result.VideoID),
		zap.Bool("thumbnail", submitThumbnail),
		zap.Bool("sprites", submitSprites),
	)
}

func (s *VideoProcessingService) onThumbnailComplete(result jobs.JobResult) {
	thumbnailJob, ok := result.Data.(*jobs.ThumbnailJob)
	if ok {
		thumbResult := thumbnailJob.GetResult()
		if thumbResult != nil {
			s.eventBus.Publish(VideoEvent{
				Type:    "video:thumbnail_complete",
				VideoID: result.VideoID,
				Data: map[string]any{
					"thumbnail_path": thumbResult.ThumbnailPath,
				},
			})
		}
	}

	// Trigger any phases configured to run after thumbnail
	for _, phase := range s.getPhasesTriggeredAfter("thumbnail") {
		if err := s.SubmitPhase(result.VideoID, phase); err != nil {
			s.logger.Error("Failed to submit phase after thumbnail",
				zap.Uint("video_id", result.VideoID),
				zap.String("phase", phase),
				zap.Error(err),
			)
		}
	}

	s.checkAllPhasesComplete(result.VideoID, "thumbnail")
}

func (s *VideoProcessingService) onSpritesComplete(result jobs.JobResult) {
	spritesJob, ok := result.Data.(*jobs.SpritesJob)
	if ok {
		spritesResult := spritesJob.GetResult()
		if spritesResult != nil {
			s.eventBus.Publish(VideoEvent{
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
	for _, phase := range s.getPhasesTriggeredAfter("sprites") {
		if err := s.SubmitPhase(result.VideoID, phase); err != nil {
			s.logger.Error("Failed to submit phase after sprites",
				zap.Uint("video_id", result.VideoID),
				zap.String("phase", phase),
				zap.Error(err),
			)
		}
	}

	s.checkAllPhasesComplete(result.VideoID, "sprites")
}

func (s *VideoProcessingService) checkAllPhasesComplete(videoID uint, completedPhase string) {
	val, ok := s.phases.Load(videoID)
	if !ok {
		// No phase state means this was a standalone trigger (manual/scheduled)
		// or metadata completed with no auto-follow phases
		if completedPhase == "metadata" {
			// Check if neither thumbnail nor sprites are auto-dispatched
			phasesAfter := s.getPhasesTriggeredAfter("metadata")
			if len(phasesAfter) == 0 {
				if err := s.repo.UpdateProcessingStatus(videoID, "completed", ""); err != nil {
					s.logger.Error("Failed to update processing status to completed",
						zap.Uint("video_id", videoID),
						zap.Error(err),
					)
					return
				}
				s.eventBus.Publish(VideoEvent{
					Type:    "video:completed",
					VideoID: videoID,
				})
			}
		}
		return
	}

	state := val.(*phaseState)

	// Determine which phases are part of the auto-pipeline
	phasesAfterMeta := s.getPhasesTriggeredAfter("metadata")
	thumbnailInPipeline := false
	spritesInPipeline := false
	for _, p := range phasesAfterMeta {
		if p == "thumbnail" {
			thumbnailInPipeline = true
		}
		if p == "sprites" {
			spritesInPipeline = true
		}
	}

	switch completedPhase {
	case "thumbnail":
		state.thumbnailDone = true
	case "sprites":
		state.spritesDone = true
	}

	// Check completion: only phases in the pipeline matter
	thumbnailReady := !thumbnailInPipeline || state.thumbnailDone
	spritesReady := !spritesInPipeline || state.spritesDone

	if thumbnailReady && spritesReady {
		s.phases.Delete(videoID)

		if err := s.repo.UpdateProcessingStatus(videoID, "completed", ""); err != nil {
			s.logger.Error("Failed to update processing status to completed",
				zap.Uint("video_id", videoID),
				zap.Error(err),
			)
			return
		}

		s.eventBus.Publish(VideoEvent{
			Type:    "video:completed",
			VideoID: videoID,
		})

		s.logger.Info("All processing phases completed for video",
			zap.Uint("video_id", videoID),
		)
	}
}

func (s *VideoProcessingService) handleFailed(result jobs.JobResult) {
	s.logger.Error("Job phase failed",
		zap.String("job_id", result.JobID),
		zap.String("phase", result.Phase),
		zap.Uint("video_id", result.VideoID),
		zap.Error(result.Error),
	)

	if s.jobHistory != nil && result.Error != nil {
		// Use RecordJobFailedWithRetry to schedule automatic retry
		s.jobHistory.RecordJobFailedWithRetry(result.JobID, result.VideoID, result.Phase, result.Error)
	}

	s.phases.Delete(result.VideoID)

	s.eventBus.Publish(VideoEvent{
		Type:    "video:failed",
		VideoID: result.VideoID,
		Data: map[string]any{
			"error": result.Error.Error(),
			"phase": result.Phase,
		},
	})
}

func (s *VideoProcessingService) handleCancelled(result jobs.JobResult) {
	s.logger.Warn("Job cancelled",
		zap.String("job_id", result.JobID),
		zap.String("phase", result.Phase),
		zap.Uint("video_id", result.VideoID),
	)

	if s.jobHistory != nil {
		s.jobHistory.RecordJobCancelled(result.JobID)
	}

	s.phases.Delete(result.VideoID)

	s.eventBus.Publish(VideoEvent{
		Type:    "video:cancelled",
		VideoID: result.VideoID,
		Data: map[string]any{
			"phase": result.Phase,
		},
	})
}

func (s *VideoProcessingService) handleTimedOut(result jobs.JobResult) {
	s.logger.Error("Job timed out",
		zap.String("job_id", result.JobID),
		zap.String("phase", result.Phase),
		zap.Uint("video_id", result.VideoID),
	)

	if s.jobHistory != nil {
		// Use RecordJobFailedWithRetry to schedule automatic retry for timed out jobs
		timeoutErr := fmt.Errorf("job timed out")
		s.jobHistory.RecordJobFailedWithRetry(result.JobID, result.VideoID, result.Phase, timeoutErr)
	}

	s.phases.Delete(result.VideoID)

	s.eventBus.Publish(VideoEvent{
		Type:    "video:timed_out",
		VideoID: result.VideoID,
		Data: map[string]any{
			"phase": result.Phase,
		},
	})
}

func (s *VideoProcessingService) Stop() {
	s.logger.Info("Stopping video processing service")
	s.metadataPool.Stop()
	s.thumbnailPool.Stop()
	s.spritesPool.Stop()
}

// CancelJob cancels a running job by its ID. It searches all pools.
func (s *VideoProcessingService) CancelJob(jobID string) error {
	s.poolMu.RLock()
	defer s.poolMu.RUnlock()

	// Try to find and cancel the job in each pool
	if err := s.metadataPool.CancelJob(jobID); err == nil {
		s.logger.Info("Job cancelled in metadata pool", zap.String("job_id", jobID))
		return nil
	}

	if err := s.thumbnailPool.CancelJob(jobID); err == nil {
		s.logger.Info("Job cancelled in thumbnail pool", zap.String("job_id", jobID))
		return nil
	}

	if err := s.spritesPool.CancelJob(jobID); err == nil {
		s.logger.Info("Job cancelled in sprites pool", zap.String("job_id", jobID))
		return nil
	}

	return fmt.Errorf("job not found: %s", jobID)
}

// GetJob retrieves a job by its ID from any pool.
func (s *VideoProcessingService) GetJob(jobID string) (jobs.Job, bool) {
	s.poolMu.RLock()
	defer s.poolMu.RUnlock()

	if job, ok := s.metadataPool.GetJob(jobID); ok {
		return job, true
	}
	if job, ok := s.thumbnailPool.GetJob(jobID); ok {
		return job, true
	}
	if job, ok := s.spritesPool.GetJob(jobID); ok {
		return job, true
	}
	return nil, false
}

func (s *VideoProcessingService) RefreshTriggerCache() error {
	if s.triggerConfigRepo == nil {
		return nil
	}
	configs, err := s.triggerConfigRepo.GetAll()
	if err != nil {
		return fmt.Errorf("failed to load trigger configs: %w", err)
	}
	s.triggerCacheMu.Lock()
	s.triggerCache = configs
	s.triggerCacheMu.Unlock()
	return nil
}

func (s *VideoProcessingService) getTriggerForPhase(phase string) *data.TriggerConfigRecord {
	s.triggerCacheMu.RLock()
	defer s.triggerCacheMu.RUnlock()
	for i := range s.triggerCache {
		if s.triggerCache[i].Phase == phase {
			return &s.triggerCache[i]
		}
	}
	return nil
}

func (s *VideoProcessingService) shouldAutoDispatch(phase string) bool {
	trigger := s.getTriggerForPhase(phase)
	if trigger == nil {
		// Default behavior: metadata=on_import, thumbnail/sprites=after_job(metadata)
		return true
	}
	return trigger.TriggerType == "on_import" || trigger.TriggerType == "after_job"
}

func (s *VideoProcessingService) getPhasesTriggeredAfter(completedPhase string) []string {
	s.triggerCacheMu.RLock()
	defer s.triggerCacheMu.RUnlock()

	var phases []string
	for _, cfg := range s.triggerCache {
		if cfg.TriggerType == "after_job" && cfg.AfterPhase != nil && *cfg.AfterPhase == completedPhase {
			phases = append(phases, cfg.Phase)
		}
	}
	return phases
}

func (s *VideoProcessingService) SubmitPhase(videoID uint, phase string) error {
	return s.SubmitPhaseWithRetry(videoID, phase, 0, 0)
}

// SubmitPhaseWithRetry submits a phase for processing with retry tracking.
// retryCount is the current retry attempt (0 for first attempt).
// maxRetries is the maximum number of retries allowed (0 uses default from config).
func (s *VideoProcessingService) SubmitPhaseWithRetry(videoID uint, phase string, retryCount, maxRetries int) error {
	video, err := s.repo.GetByID(videoID)
	if err != nil {
		return fmt.Errorf("failed to get video: %w", err)
	}

	s.poolMu.RLock()
	dimSm := s.processingQualityConfig.MaxFrameDimensionSm
	dimLg := s.processingQualityConfig.MaxFrameDimensionLg
	qualitySm := s.processingQualityConfig.FrameQualitySm
	qualityLg := s.processingQualityConfig.FrameQualityLg
	qualitySprites := s.processingQualityConfig.FrameQualitySprites
	spritesConcurrency := s.processingQualityConfig.SpritesConcurrency
	s.poolMu.RUnlock()

	// Helper to record job start with or without retry info
	recordJobStart := func(jobID string, phase string) {
		if s.jobHistory == nil {
			return
		}
		if retryCount > 0 || maxRetries > 0 {
			s.jobHistory.RecordJobStartWithRetry(jobID, videoID, video.Title, phase, maxRetries, retryCount)
		} else {
			s.jobHistory.RecordJobStart(jobID, videoID, video.Title, phase)
		}
	}

	switch phase {
	case "metadata":
		job := jobs.NewMetadataJob(videoID, video.StoredPath, dimSm, dimLg, s.repo, s.logger)
		s.poolMu.RLock()
		err = s.metadataPool.Submit(job)
		s.poolMu.RUnlock()
		if err != nil {
			if jobs.IsDuplicateJobError(err) {
				s.logger.Info("Duplicate metadata job skipped",
					zap.Uint("video_id", videoID),
				)
				return nil
			}
			return fmt.Errorf("failed to submit metadata job: %w", err)
		}
		recordJobStart(job.GetID(), "metadata")

	case "thumbnail":
		if video.Duration == 0 {
			return fmt.Errorf("metadata must be extracted before thumbnail generation")
		}
		s.logger.Info("SubmitPhase: Creating thumbnail job",
			zap.Uint("video_id", videoID),
			zap.Uint("video_db_id", video.ID),
			zap.String("video_stored_path", video.StoredPath),
			zap.String("video_title", video.Title),
		)
		tileWidthLg, tileHeightLg := ffmpeg.CalculateTileDimensions(video.Width, video.Height, s.config.MaxFrameDimensionLarge)
		thumbnailJob := jobs.NewThumbnailJob(
			videoID, video.StoredPath, s.config.ThumbnailDir,
			video.ThumbnailWidth, video.ThumbnailHeight,
			tileWidthLg, tileHeightLg,
			video.Duration, qualitySm, qualityLg, s.repo, s.logger,
		)
		s.poolMu.RLock()
		err = s.thumbnailPool.Submit(thumbnailJob)
		s.poolMu.RUnlock()
		if err != nil {
			if jobs.IsDuplicateJobError(err) {
				s.logger.Info("Duplicate thumbnail job skipped",
					zap.Uint("video_id", videoID),
				)
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
			videoID, video.StoredPath, s.config.SpriteDir, s.config.VttDir,
			video.ThumbnailWidth, video.ThumbnailHeight, video.Duration,
			s.config.FrameInterval, qualitySprites, s.config.GridCols, s.config.GridRows,
			spritesConcurrency, s.repo, s.logger,
		)
		s.poolMu.RLock()
		err = s.spritesPool.Submit(spritesJob)
		s.poolMu.RUnlock()
		if err != nil {
			if jobs.IsDuplicateJobError(err) {
				s.logger.Info("Duplicate sprites job skipped",
					zap.Uint("video_id", videoID),
				)
				return nil
			}
			return fmt.Errorf("failed to submit sprites job: %w", err)
		}
		recordJobStart(spritesJob.GetID(), "sprites")

	default:
		return fmt.Errorf("unknown phase: %s", phase)
	}

	s.logger.Info("Phase submitted",
		zap.Uint("video_id", videoID),
		zap.String("phase", phase),
		zap.Int("retry_count", retryCount),
	)
	return nil
}

// BulkPhaseResult contains the results of a bulk phase submission
type BulkPhaseResult struct {
	Submitted int `json:"submitted"`
	Skipped   int `json:"skipped"`
	Errors    int `json:"errors"`
}

// SubmitBulkPhase submits a processing phase for multiple videos
// mode can be "missing" (only videos needing the phase) or "all" (all videos)
func (s *VideoProcessingService) SubmitBulkPhase(phase string, mode string) (*BulkPhaseResult, error) {
	var videos []data.Video
	var err error

	if mode == "all" {
		videos, err = s.repo.GetAll()
		if err != nil {
			return nil, fmt.Errorf("failed to get videos: %w", err)
		}
	} else {
		// Default to "missing" mode
		videos, err = s.repo.GetVideosNeedingPhase(phase)
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

		if err := s.SubmitPhase(video.ID, phase); err != nil {
			s.logger.Warn("Failed to submit bulk phase job",
				zap.Uint("video_id", video.ID),
				zap.String("phase", phase),
				zap.Error(err),
			)
			result.Errors++
		} else {
			result.Submitted++
		}
	}

	s.logger.Info("Bulk phase submission completed",
		zap.String("phase", phase),
		zap.String("mode", mode),
		zap.Int("submitted", result.Submitted),
		zap.Int("skipped", result.Skipped),
		zap.Int("errors", result.Errors),
	)

	return result, nil
}

func (s *VideoProcessingService) GetPoolConfig() PoolConfig {
	s.poolMu.RLock()
	defer s.poolMu.RUnlock()
	return PoolConfig{
		MetadataWorkers:  s.metadataPool.ActiveWorkers(),
		ThumbnailWorkers: s.thumbnailPool.ActiveWorkers(),
		SpritesWorkers:   s.spritesPool.ActiveWorkers(),
	}
}

func (s *VideoProcessingService) GetQueueStatus() QueueStatus {
	s.poolMu.RLock()
	defer s.poolMu.RUnlock()
	return QueueStatus{
		MetadataQueued:  s.metadataPool.QueueSize(),
		ThumbnailQueued: s.thumbnailPool.QueueSize(),
		SpritesQueued:   s.spritesPool.QueueSize(),
	}
}

func (s *VideoProcessingService) UpdatePoolConfig(cfg PoolConfig) error {
	s.poolMu.Lock()
	defer s.poolMu.Unlock()

	if cfg.MetadataWorkers < 1 || cfg.MetadataWorkers > 10 {
		return fmt.Errorf("metadata_workers must be between 1 and 10")
	}
	if cfg.ThumbnailWorkers < 1 || cfg.ThumbnailWorkers > 10 {
		return fmt.Errorf("thumbnail_workers must be between 1 and 10")
	}
	if cfg.SpritesWorkers < 1 || cfg.SpritesWorkers > 10 {
		return fmt.Errorf("sprites_workers must be between 1 and 10")
	}

	// Resize metadata pool if needed
	if cfg.MetadataWorkers != s.metadataPool.ActiveWorkers() {
		newPool := jobs.NewWorkerPool(cfg.MetadataWorkers, 100)
		newPool.SetLogger(s.logger.With(zap.String("pool", "metadata")))
		newPool.Start()
		go s.processPoolResults(newPool)

		oldPool := s.metadataPool
		s.metadataPool = newPool
		oldPool.Stop()

		s.logger.Info("Resized metadata pool", zap.Int("workers", cfg.MetadataWorkers))
	}

	// Resize thumbnail pool if needed
	if cfg.ThumbnailWorkers != s.thumbnailPool.ActiveWorkers() {
		newPool := jobs.NewWorkerPool(cfg.ThumbnailWorkers, 100)
		newPool.SetLogger(s.logger.With(zap.String("pool", "thumbnail")))
		newPool.Start()
		go s.processPoolResults(newPool)

		oldPool := s.thumbnailPool
		s.thumbnailPool = newPool
		oldPool.Stop()

		s.logger.Info("Resized thumbnail pool", zap.Int("workers", cfg.ThumbnailWorkers))
	}

	// Resize sprites pool if needed
	if cfg.SpritesWorkers != s.spritesPool.ActiveWorkers() {
		newPool := jobs.NewWorkerPool(cfg.SpritesWorkers, 100)
		newPool.SetLogger(s.logger.With(zap.String("pool", "sprites")))
		newPool.Start()
		go s.processPoolResults(newPool)

		oldPool := s.spritesPool
		s.spritesPool = newPool
		oldPool.Stop()

		s.logger.Info("Resized sprites pool", zap.Int("workers", cfg.SpritesWorkers))
	}

	return nil
}

func (s *VideoProcessingService) GetProcessingQualityConfig() ProcessingQualityConfig {
	s.poolMu.RLock()
	defer s.poolMu.RUnlock()
	return s.processingQualityConfig
}

var validDimensionsSm = map[int]bool{160: true, 240: true, 320: true, 480: true}
var validDimensionsLg = map[int]bool{640: true, 720: true, 960: true, 1280: true, 1920: true}

func (s *VideoProcessingService) UpdateProcessingQualityConfig(cfg ProcessingQualityConfig) error {
	if !validDimensionsSm[cfg.MaxFrameDimensionSm] {
		return fmt.Errorf("max_frame_dimension_sm must be one of: 160, 240, 320, 480")
	}
	if !validDimensionsLg[cfg.MaxFrameDimensionLg] {
		return fmt.Errorf("max_frame_dimension_lg must be one of: 640, 720, 960, 1280, 1920")
	}
	if cfg.FrameQualitySm < 1 || cfg.FrameQualitySm > 100 {
		return fmt.Errorf("frame_quality_sm must be between 1 and 100")
	}
	if cfg.FrameQualityLg < 1 || cfg.FrameQualityLg > 100 {
		return fmt.Errorf("frame_quality_lg must be between 1 and 100")
	}
	if cfg.FrameQualitySprites < 1 || cfg.FrameQualitySprites > 100 {
		return fmt.Errorf("frame_quality_sprites must be between 1 and 100")
	}
	if cfg.SpritesConcurrency < 0 || cfg.SpritesConcurrency > 64 {
		return fmt.Errorf("sprites_concurrency must be between 0 and 64 (0 = auto)")
	}

	s.poolMu.Lock()
	s.processingQualityConfig = cfg
	s.poolMu.Unlock()

	s.logger.Info("Updated processing quality config",
		zap.Int("max_frame_dimension_sm", cfg.MaxFrameDimensionSm),
		zap.Int("max_frame_dimension_lg", cfg.MaxFrameDimensionLg),
		zap.Int("frame_quality_sm", cfg.FrameQualitySm),
		zap.Int("frame_quality_lg", cfg.FrameQualityLg),
		zap.Int("frame_quality_sprites", cfg.FrameQualitySprites),
		zap.Int("sprites_concurrency", cfg.SpritesConcurrency),
	)

	return nil
}

func (s *VideoProcessingService) LogStatus() {
	s.logger.Info("Video processing service status")
	s.poolMu.RLock()
	defer s.poolMu.RUnlock()
	s.metadataPool.LogStatus()
	s.thumbnailPool.LogStatus()
	s.spritesPool.LogStatus()
}
