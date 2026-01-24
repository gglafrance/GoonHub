package core

import (
	"fmt"
	"goonhub/internal/data"
	"sync"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

type TriggerScheduler struct {
	cron              *cron.Cron
	triggerConfigRepo data.TriggerConfigRepository
	videoRepo         data.VideoRepository
	processingService *VideoProcessingService
	logger            *zap.Logger
	mu                sync.Mutex
	entryIDs          []cron.EntryID
}

func NewTriggerScheduler(
	triggerConfigRepo data.TriggerConfigRepository,
	videoRepo data.VideoRepository,
	processingService *VideoProcessingService,
	logger *zap.Logger,
) *TriggerScheduler {
	return &TriggerScheduler{
		triggerConfigRepo: triggerConfigRepo,
		videoRepo:         videoRepo,
		processingService: processingService,
		logger:            logger,
	}
}

func (s *TriggerScheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cron = cron.New(cron.WithParser(cron.NewParser(
		cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
	)))

	if err := s.loadSchedules(); err != nil {
		s.logger.Error("Failed to load trigger schedules on start", zap.Error(err))
	}

	s.cron.Start()
	s.logger.Info("Trigger scheduler started")
}

func (s *TriggerScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cron != nil {
		ctx := s.cron.Stop()
		<-ctx.Done()
		s.logger.Info("Trigger scheduler stopped")
	}
}

func (s *TriggerScheduler) RefreshSchedules() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove existing entries
	for _, id := range s.entryIDs {
		s.cron.Remove(id)
	}
	s.entryIDs = nil

	return s.loadSchedules()
}

func (s *TriggerScheduler) loadSchedules() error {
	configs, err := s.triggerConfigRepo.GetAll()
	if err != nil {
		return fmt.Errorf("failed to load trigger configs: %w", err)
	}

	for _, cfg := range configs {
		if cfg.TriggerType != "scheduled" || cfg.CronExpression == nil {
			continue
		}

		phase := cfg.Phase
		expr := *cfg.CronExpression

		id, err := s.cron.AddFunc(expr, func() {
			s.runScheduledPhase(phase)
		})
		if err != nil {
			s.logger.Error("Failed to register cron entry",
				zap.String("phase", phase),
				zap.String("cron_expression", expr),
				zap.Error(err),
			)
			continue
		}

		s.entryIDs = append(s.entryIDs, id)
		s.logger.Info("Registered scheduled trigger",
			zap.String("phase", phase),
			zap.String("cron_expression", expr),
		)
	}

	return nil
}

func (s *TriggerScheduler) runScheduledPhase(phase string) {
	s.logger.Info("Running scheduled trigger", zap.String("phase", phase))

	videos, err := s.videoRepo.GetVideosNeedingPhase(phase)
	if err != nil {
		s.logger.Error("Failed to get videos needing phase",
			zap.String("phase", phase),
			zap.Error(err),
		)
		return
	}

	if len(videos) == 0 {
		s.logger.Debug("No videos need processing for scheduled phase", zap.String("phase", phase))
		return
	}

	s.logger.Info("Found videos needing scheduled phase",
		zap.String("phase", phase),
		zap.Int("count", len(videos)),
	)

	for _, video := range videos {
		if err := s.processingService.SubmitPhase(video.ID, phase); err != nil {
			s.logger.Error("Failed to submit scheduled phase job",
				zap.Uint("video_id", video.ID),
				zap.String("phase", phase),
				zap.Error(err),
			)
		}
	}
}
