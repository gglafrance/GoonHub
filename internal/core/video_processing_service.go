package core

import (
	"goonhub/internal/config"
	"goonhub/internal/data"
	"goonhub/internal/jobs"
	"os"

	"go.uber.org/zap"
)

type VideoProcessingService struct {
	pool   *jobs.WorkerPool
	repo   data.VideoRepository
	config config.ProcessingConfig
	logger *zap.Logger
}

func NewVideoProcessingService(
	repo data.VideoRepository,
	config config.ProcessingConfig,
	logger *zap.Logger,
) *VideoProcessingService {
	logger.Info("Initializing video processing service",
		zap.Int("worker_count", config.WorkerCount),
		zap.Int("frame_interval", config.FrameInterval),
		zap.Int("frame_width", config.FrameWidth),
		zap.Int("frame_height", config.FrameHeight),
		zap.Int("frame_quality", config.FrameQuality),
		zap.String("frame_output_dir", config.FrameOutputDir),
		zap.String("thumbnail_dir", config.ThumbnailDir),
	)

	pool := jobs.NewWorkerPool(config.WorkerCount, 100)
	pool.SetLogger(logger)

	if err := os.MkdirAll(config.FrameOutputDir, 0755); err != nil {
		logger.Error("Failed to create frame output directory",
			zap.String("directory", config.FrameOutputDir),
			zap.Error(err),
		)
	} else {
		logger.Info("Frame output directory ready", zap.String("directory", config.FrameOutputDir))
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
		pool:   pool,
		repo:   repo,
		config: config,
		logger: logger,
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

	job := jobs.NewProcessVideoJob(
		videoID,
		videoPath,
		s.config.FrameOutputDir,
		s.config.ThumbnailDir,
		s.config.FrameInterval,
		s.config.FrameWidth,
		s.config.FrameHeight,
		s.config.FrameQuality,
		s.config.ThumbnailSeek,
		s.repo,
		s.logger,
	)

	if err := s.pool.Submit(job); err != nil {
		s.logger.Error("Failed to submit video for processing",
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
			s.logger.Info("Job completed successfully",
				zap.String("job_id", result.JobID),
				zap.Int("remaining_queue", s.pool.QueueSize()),
			)
		case jobs.JobStatusFailed:
			s.logger.Error("Job failed",
				zap.String("job_id", result.JobID),
				zap.Error(result.Error),
				zap.Int("remaining_queue", s.pool.QueueSize()),
			)
		case jobs.JobStatusCancelled:
			s.logger.Warn("Job cancelled",
				zap.String("job_id", result.JobID),
			)
		}
	}
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
