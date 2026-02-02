package core

import (
	"goonhub/internal/data"
	"goonhub/internal/lifecycle"
	"time"

	"go.uber.org/zap"
)

// TrashCleanupWorker periodically cleans up expired trashed scenes.
type TrashCleanupWorker struct {
	sceneService    *SceneService
	sceneRepo       data.SceneRepository
	appSettingsRepo data.AppSettingsRepository
	lifecycle       *lifecycle.Manager
	logger          *zap.Logger
	stopCh          chan struct{}
}

// NewTrashCleanupWorker creates a new trash cleanup worker.
func NewTrashCleanupWorker(
	sceneService *SceneService,
	sceneRepo data.SceneRepository,
	appSettingsRepo data.AppSettingsRepository,
	lifecycle *lifecycle.Manager,
	logger *zap.Logger,
) *TrashCleanupWorker {
	return &TrashCleanupWorker{
		sceneService:    sceneService,
		sceneRepo:       sceneRepo,
		appSettingsRepo: appSettingsRepo,
		lifecycle:       lifecycle,
		logger:          logger.With(zap.String("component", "trash_cleanup_worker")),
		stopCh:          make(chan struct{}),
	}
}

// Start begins the cleanup worker loop.
func (w *TrashCleanupWorker) Start() {
	w.lifecycle.Go("trash-cleanup-worker", func(done <-chan struct{}) {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		// Run cleanup immediately on startup
		w.cleanup()

		for {
			select {
			case <-done:
				w.logger.Info("Trash cleanup worker stopping due to shutdown")
				return
			case <-w.stopCh:
				w.logger.Info("Trash cleanup worker stopping")
				return
			case <-ticker.C:
				w.cleanup()
			}
		}
	})
}

// Stop signals the worker to stop.
func (w *TrashCleanupWorker) Stop() {
	close(w.stopCh)
}

// cleanup performs the actual cleanup of expired trashed scenes.
func (w *TrashCleanupWorker) cleanup() {
	w.logger.Debug("Running trash cleanup")

	// Get retention days from settings
	retentionDays := 7 // default
	if w.appSettingsRepo != nil {
		settings, err := w.appSettingsRepo.Get()
		if err == nil && settings != nil {
			retentionDays = settings.TrashRetentionDays
		}
	}

	// Get expired trashed scenes
	expiredScenes, err := w.sceneRepo.GetExpiredTrashScenes(retentionDays)
	if err != nil {
		w.logger.Error("Failed to get expired trash scenes", zap.Error(err))
		return
	}

	if len(expiredScenes) == 0 {
		w.logger.Debug("No expired trash scenes to clean up")
		return
	}

	w.logger.Info("Found expired trash scenes to clean up",
		zap.Int("count", len(expiredScenes)),
		zap.Int("retention_days", retentionDays),
	)

	deleted := 0
	for _, scene := range expiredScenes {
		if err := w.sceneService.HardDeleteScene(scene.ID); err != nil {
			w.logger.Warn("Failed to hard delete expired scene",
				zap.Uint("scene_id", scene.ID),
				zap.String("title", scene.Title),
				zap.Error(err),
			)
			continue
		}
		deleted++
		w.logger.Info("Permanently deleted expired scene",
			zap.Uint("scene_id", scene.ID),
			zap.String("title", scene.Title),
		)
	}

	w.logger.Info("Trash cleanup completed",
		zap.Int("deleted", deleted),
		zap.Int("total_expired", len(expiredScenes)),
	)
}
