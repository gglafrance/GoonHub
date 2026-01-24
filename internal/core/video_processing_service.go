package core

import (
	"fmt"
	"goonhub/internal/config"
	"goonhub/internal/data"
	"goonhub/internal/jobs"
	"os"
	"sync"

	"go.uber.org/zap"
)

type PoolConfig struct {
	MetadataWorkers  int `json:"metadata_workers"`
	ThumbnailWorkers int `json:"thumbnail_workers"`
	SpritesWorkers   int `json:"sprites_workers"`
}

type phaseState struct {
	thumbnailDone bool
	spritesDone   bool
}

type VideoProcessingService struct {
	metadataPool  *jobs.WorkerPool
	thumbnailPool *jobs.WorkerPool
	spritesPool   *jobs.WorkerPool
	poolMu        sync.RWMutex
	repo          data.VideoRepository
	config        config.ProcessingConfig
	logger        *zap.Logger
	eventBus      *EventBus
	jobHistory    *JobHistoryService
	phases        sync.Map // map[uint]*phaseState
}

func NewVideoProcessingService(
	repo data.VideoRepository,
	cfg config.ProcessingConfig,
	logger *zap.Logger,
	eventBus *EventBus,
	jobHistory *JobHistoryService,
	poolConfigRepo data.PoolConfigRepository,
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

	logger.Info("Initializing video processing service",
		zap.Int("metadata_workers", metadataWorkers),
		zap.Int("thumbnail_workers", thumbnailWorkers),
		zap.Int("sprites_workers", spritesWorkers),
		zap.Int("frame_interval", cfg.FrameInterval),
		zap.Int("max_frame_dimension", cfg.MaxFrameDimension),
		zap.Int("frame_quality", cfg.FrameQuality),
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
		metadataPool:  metadataPool,
		thumbnailPool: thumbnailPool,
		spritesPool:   spritesPool,
		repo:          repo,
		config:        cfg,
		logger:        logger,
		eventBus:      eventBus,
		jobHistory:    jobHistory,
	}
}

func (s *VideoProcessingService) Start() {
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

func (s *VideoProcessingService) SubmitVideo(videoID uint, videoPath string) error {
	s.logger.Info("Video submitted for processing",
		zap.Uint("video_id", videoID),
		zap.String("video_path", videoPath),
	)

	job := jobs.NewMetadataJob(
		videoID,
		videoPath,
		s.config.MaxFrameDimension,
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

	// Submit thumbnail and sprites jobs to their respective pools
	thumbnailJob := jobs.NewThumbnailJob(
		result.VideoID,
		videoPath,
		s.config.ThumbnailDir,
		meta.TileWidth,
		meta.TileHeight,
		meta.Duration,
		s.config.FrameQuality,
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
		s.config.FrameQuality,
		s.config.GridCols,
		s.config.GridRows,
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

func (s *VideoProcessingService) LogStatus() {
	s.logger.Info("Video processing service status")
	s.poolMu.RLock()
	defer s.poolMu.RUnlock()
	s.metadataPool.LogStatus()
	s.thumbnailPool.LogStatus()
	s.spritesPool.LogStatus()
}
