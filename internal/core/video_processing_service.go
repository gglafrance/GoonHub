package core

import (
	"goonhub/internal/config"
	"goonhub/internal/data"
	"goonhub/internal/jobs"
	"os"
	"sync"

	"go.uber.org/zap"
)

type phaseState struct {
	thumbnailDone bool
	spritesDone   bool
}

type VideoProcessingService struct {
	pool       *jobs.WorkerPool
	repo       data.VideoRepository
	config     config.ProcessingConfig
	logger     *zap.Logger
	eventBus   *EventBus
	phases     sync.Map // map[uint]*phaseState
}

func NewVideoProcessingService(
	repo data.VideoRepository,
	config config.ProcessingConfig,
	logger *zap.Logger,
	eventBus *EventBus,
) *VideoProcessingService {
	logger.Info("Initializing video processing service",
		zap.Int("worker_count", config.WorkerCount),
		zap.Int("frame_interval", config.FrameInterval),
		zap.Int("max_frame_dimension", config.MaxFrameDimension),
		zap.Int("frame_quality", config.FrameQuality),
		zap.Int("grid_cols", config.GridCols),
		zap.Int("grid_rows", config.GridRows),
		zap.String("sprite_dir", config.SpriteDir),
		zap.String("vtt_dir", config.VttDir),
		zap.String("thumbnail_dir", config.ThumbnailDir),
	)

	pool := jobs.NewWorkerPool(config.WorkerCount, 100)
	pool.SetLogger(logger)

	if err := os.MkdirAll(config.SpriteDir, 0755); err != nil {
		logger.Error("Failed to create sprite directory",
			zap.String("directory", config.SpriteDir),
			zap.Error(err),
		)
	} else {
		logger.Info("Sprite directory ready", zap.String("directory", config.SpriteDir))
	}

	if err := os.MkdirAll(config.VttDir, 0755); err != nil {
		logger.Error("Failed to create VTT directory",
			zap.String("directory", config.VttDir),
			zap.Error(err),
		)
	} else {
		logger.Info("VTT directory ready", zap.String("directory", config.VttDir))
	}

	if err := os.MkdirAll(config.ThumbnailDir, 0755); err != nil {
		logger.Error("Failed to create thumbnail directory",
			zap.String("directory", config.ThumbnailDir),
			zap.Error(err),
		)
	} else {
		logger.Info("Thumbnail directory ready", zap.String("directory", config.ThumbnailDir))
	}

	return &VideoProcessingService{
		pool:     pool,
		repo:     repo,
		config:   config,
		logger:   logger,
		eventBus: eventBus,
	}
}

func (s *VideoProcessingService) Start() {
	s.pool.Start()

	go s.processResults()

	s.logger.Info("Video processing service started",
		zap.Int("worker_count", s.config.WorkerCount),
		zap.Int("queue_capacity", 100),
	)
}

func (s *VideoProcessingService) SubmitVideo(videoID uint, videoPath string) error {
	s.logger.Info("Video submitted for processing",
		zap.Uint("video_id", videoID),
		zap.String("video_path", videoPath),
		zap.Int("current_queue_depth", s.pool.QueueSize()),
	)

	job := jobs.NewMetadataJob(
		videoID,
		videoPath,
		s.config.MaxFrameDimension,
		s.repo,
		s.logger,
	)

	if err := s.pool.Submit(job); err != nil {
		s.logger.Error("Failed to submit metadata job",
			zap.Uint("video_id", videoID),
			zap.Error(err),
		)
		return err
	}

	return nil
}

func (s *VideoProcessingService) processResults() {
	s.logger.Info("Job result processor started")

	for result := range s.pool.Results() {
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

	// Submit thumbnail and sprites jobs in parallel
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

	if err := s.pool.Submit(thumbnailJob); err != nil {
		s.logger.Error("Failed to submit thumbnail job",
			zap.Uint("video_id", result.VideoID),
			zap.Error(err),
		)
		s.repo.UpdateProcessingStatus(result.VideoID, "failed", "failed to submit thumbnail job")
		return
	}

	if err := s.pool.Submit(spritesJob); err != nil {
		s.logger.Error("Failed to submit sprites job",
			zap.Uint("video_id", result.VideoID),
			zap.Error(err),
		)
		s.repo.UpdateProcessingStatus(result.VideoID, "failed", "failed to submit sprites job")
		return
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
	s.pool.Stop()
}

func (s *VideoProcessingService) GetPool() *jobs.WorkerPool {
	return s.pool
}

func (s *VideoProcessingService) LogStatus() {
	s.logger.Info("Video processing service status")
	s.pool.LogStatus()
}
