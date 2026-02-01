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
	sceneRepo         data.SceneRepository
	processingService *SceneProcessingService
	scanService       *ScanService
	logger            *zap.Logger
	mu                sync.Mutex
	entryIDs          []cron.EntryID
}

// SetScanService sets the scan service for scheduled library scans
func (s *TriggerScheduler) SetScanService(scanService *ScanService) {
	s.scanService = scanService
}

func NewTriggerScheduler(
	triggerConfigRepo data.TriggerConfigRepository,
	sceneRepo data.SceneRepository,
	processingService *SceneProcessingService,
	logger *zap.Logger,
) *TriggerScheduler {
	return &TriggerScheduler{
		triggerConfigRepo: triggerConfigRepo,
		sceneRepo:         sceneRepo,
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

	// Handle scan phase specially
	if phase == "scan" {
		s.runScheduledScan()
		return
	}

	scenes, err := s.sceneRepo.GetScenesNeedingPhase(phase)
	if err != nil {
		s.logger.Error("Failed to get scenes needing phase",
			zap.String("phase", phase),
			zap.Error(err),
		)
		return
	}

	if len(scenes) == 0 {
		s.logger.Debug("No scenes need processing for scheduled phase", zap.String("phase", phase))
		return
	}

	s.logger.Info("Found scenes needing scheduled phase",
		zap.String("phase", phase),
		zap.Int("count", len(scenes)),
	)

	for _, scene := range scenes {
		if err := s.processingService.SubmitPhase(scene.ID, phase); err != nil {
			s.logger.Error("Failed to submit scheduled phase job",
				zap.Uint("scene_id", scene.ID),
				zap.String("phase", phase),
				zap.Error(err),
			)
		}
	}
}

func (s *TriggerScheduler) runScheduledScan() {
	if s.scanService == nil {
		s.logger.Error("Scan service not configured for scheduled scan")
		return
	}

	// Check if a scan is already running
	status := s.scanService.GetStatus()
	if status.Running {
		s.logger.Info("Skipping scheduled scan: scan already running")
		return
	}

	s.logger.Info("Starting scheduled library scan")
	if _, err := s.scanService.StartScan(nil); err != nil {
		s.logger.Error("Failed to start scheduled library scan", zap.Error(err))
	}
}
