package core

import (
	"fmt"
	"goonhub/internal/config"
	"goonhub/internal/data"
	"goonhub/internal/jobs"
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
	metadataPool           *jobs.WorkerPool
	thumbnailPool          *jobs.WorkerPool
	spritesPool            *jobs.WorkerPool
	poolMu                 sync.RWMutex
	repo                   data.VideoRepository
	config                 config.ProcessingConfig
	processingQualityConfig ProcessingQualityConfig
	logger                 *zap.Logger
	eventBus               *EventBus
	jobHistory             *JobHistoryService
	phases                 sync.Map // map[uint]*phaseState
}

func NewVideoProcessingService(
	repo data.VideoRepository,
	cfg config.ProcessingConfig,
	logger *zap.Logger,
	eventBus *EventBus,
	jobHistory *JobHistoryService,
	poolConfigRepo data.PoolConfigRepository,
	processingConfigRepo data.ProcessingConfigRepository,
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

	thumbnailPool := jobs.NewWorkerPool(thumbnailWorkers, 100)
	thumbnailPool.SetLogger(logger.With(zap.String("pool", "thumbnail")))

	spritesPool := jobs.NewWorkerPool(spritesWorkers, 100)
	spritesPool.SetLogger(logger.With(zap.String("pool", "sprites")))

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

	return &VideoProcessingService{
		metadataPool:            metadataPool,
		thumbnailPool:           thumbnailPool,
		spritesPool:             spritesPool,
		repo:                    repo,
		config:                  cfg,
		processingQualityConfig: qualityConfig,
		logger:                  logger,
		eventBus:                eventBus,
		jobHistory:              jobHistory,
	}
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
			s.logger.Warn("Job cancelled",
				zap.String("job_id", result.JobID),
				zap.String("phase", result.Phase),
				zap.Uint("video_id", result.VideoID),
			)
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

	s.eventBus.Publish(VideoEvent{
		Type:    "video:metadata_complete",
		VideoID: result.VideoID,
		Data: map[string]any{
			"duration": meta.Duration,
			"width":    meta.Width,
			"height":   meta.Height,
		},
	})

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

	// Submit thumbnail and sprites jobs to their respective pools
	thumbnailJob := jobs.NewThumbnailJob(
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

	spritesJob := jobs.NewSpritesJob(
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
	thumbnailErr := s.thumbnailPool.Submit(thumbnailJob)
	spritesErr := s.spritesPool.Submit(spritesJob)
	s.poolMu.RUnlock()

	if thumbnailErr != nil {
		s.logger.Error("Failed to submit thumbnail job",
			zap.Uint("video_id", result.VideoID),
			zap.Error(thumbnailErr),
		)
		s.repo.UpdateProcessingStatus(result.VideoID, "failed", "failed to submit thumbnail job")
		return
	}

	if spritesErr != nil {
		s.logger.Error("Failed to submit sprites job",
			zap.Uint("video_id", result.VideoID),
			zap.Error(spritesErr),
		)
		s.repo.UpdateProcessingStatus(result.VideoID, "failed", "failed to submit sprites job")
		return
	}

	if s.jobHistory != nil {
		videoTitle := ""
		if v, err := s.repo.GetByID(result.VideoID); err == nil {
			videoTitle = v.Title
		}
		s.jobHistory.RecordJobStart(thumbnailJob.GetID(), result.VideoID, videoTitle, "thumbnail")
		s.jobHistory.RecordJobStart(spritesJob.GetID(), result.VideoID, videoTitle, "sprites")
	}

	s.logger.Info("Submitted thumbnail and sprites jobs",
		zap.Uint("video_id", result.VideoID),
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

	s.checkAllPhasesComplete(result.VideoID, "sprites")
}

func (s *VideoProcessingService) checkAllPhasesComplete(videoID uint, completedPhase string) {
	val, ok := s.phases.Load(videoID)
	if !ok {
		s.logger.Warn("Phase state not found for video", zap.Uint("video_id", videoID))
		return
	}

	state := val.(*phaseState)
	switch completedPhase {
	case "thumbnail":
		state.thumbnailDone = true
	case "sprites":
		state.spritesDone = true
	}

	if state.thumbnailDone && state.spritesDone {
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
		s.jobHistory.RecordJobFailed(result.JobID, result.Error)
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

func (s *VideoProcessingService) Stop() {
	s.logger.Info("Stopping video processing service")
	s.metadataPool.Stop()
	s.thumbnailPool.Stop()
	s.spritesPool.Stop()
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
