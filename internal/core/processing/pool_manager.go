package processing

import (
	"fmt"
	"goonhub/internal/config"
	"goonhub/internal/data"
	"goonhub/internal/jobs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// PoolManager manages the worker pools for scene processing phases
type PoolManager struct {
	metadataPool  *jobs.WorkerPool
	thumbnailPool *jobs.WorkerPool
	spritesPool   *jobs.WorkerPool
	mu            sync.RWMutex
	config        config.ProcessingConfig
	qualityConfig QualityConfig
	logger        *zap.Logger

	// resultHandler is called when a job completes
	resultHandler func(*jobs.WorkerPool)
}

// NewPoolManager creates a new PoolManager with the given configuration
func NewPoolManager(
	cfg config.ProcessingConfig,
	logger *zap.Logger,
	poolConfigRepo data.PoolConfigRepository,
	processingConfigRepo data.ProcessingConfigRepository,
) *PoolManager {
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
	qualityConfig := QualityConfig{
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

	logger.Info("Initializing pool manager",
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

	const queueBufferSize = 1000

	metadataPool := jobs.NewWorkerPool(metadataWorkers, queueBufferSize)
	metadataPool.SetLogger(logger.With(zap.String("pool", "metadata")))
	if cfg.MetadataTimeout > 0 {
		metadataPool.SetTimeout(cfg.MetadataTimeout)
		logger.Info("Metadata pool timeout set", zap.Duration("timeout", cfg.MetadataTimeout))
	}

	thumbnailPool := jobs.NewWorkerPool(thumbnailWorkers, queueBufferSize)
	thumbnailPool.SetLogger(logger.With(zap.String("pool", "thumbnail")))
	if cfg.ThumbnailTimeout > 0 {
		thumbnailPool.SetTimeout(cfg.ThumbnailTimeout)
		logger.Info("Thumbnail pool timeout set", zap.Duration("timeout", cfg.ThumbnailTimeout))
	}

	spritesPool := jobs.NewWorkerPool(spritesWorkers, queueBufferSize)
	spritesPool.SetLogger(logger.With(zap.String("pool", "sprites")))
	if cfg.SpritesTimeout > 0 {
		spritesPool.SetTimeout(cfg.SpritesTimeout)
		logger.Info("Sprites pool timeout set", zap.Duration("timeout", cfg.SpritesTimeout))
	}

	// Create output directories
	createDirIfNotExists(cfg.SpriteDir, logger)
	createDirIfNotExists(cfg.VttDir, logger)
	createDirIfNotExists(cfg.ThumbnailDir, logger)

	return &PoolManager{
		metadataPool:  metadataPool,
		thumbnailPool: thumbnailPool,
		spritesPool:   spritesPool,
		config:        cfg,
		qualityConfig: qualityConfig,
		logger:        logger,
	}
}

func createDirIfNotExists(dir string, logger *zap.Logger) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		logger.Error("Failed to create directory",
			zap.String("directory", dir),
			zap.Error(err),
		)
	} else {
		logger.Info("Directory ready", zap.String("directory", dir))
	}
}

// SetResultHandler sets the function to be called for processing results from each pool
func (pm *PoolManager) SetResultHandler(handler func(*jobs.WorkerPool)) {
	pm.resultHandler = handler
}

// Start starts all worker pools and their result handlers
func (pm *PoolManager) Start() {
	pm.migrateOldThumbnails()

	pm.metadataPool.Start()
	pm.thumbnailPool.Start()
	pm.spritesPool.Start()

	if pm.resultHandler != nil {
		go pm.resultHandler(pm.metadataPool)
		go pm.resultHandler(pm.thumbnailPool)
		go pm.resultHandler(pm.spritesPool)
	}

	pm.logger.Info("Pool manager started",
		zap.Int("metadata_workers", pm.metadataPool.ActiveWorkers()),
		zap.Int("thumbnail_workers", pm.thumbnailPool.ActiveWorkers()),
		zap.Int("sprites_workers", pm.spritesPool.ActiveWorkers()),
	)
}

// Stop stops all worker pools
func (pm *PoolManager) Stop() {
	pm.logger.Info("Stopping pool manager")
	pm.metadataPool.Stop()
	pm.thumbnailPool.Stop()
	pm.spritesPool.Stop()
}

// GracefulStop performs graceful shutdown of all worker pools.
// It waits for in-flight jobs to complete (up to timeout) and returns
// a map of phase -> buffered job IDs that were never executed.
func (pm *PoolManager) GracefulStop(timeout time.Duration) map[string][]string {
	pm.logger.Info("Starting graceful shutdown of pool manager",
		zap.Duration("timeout", timeout),
	)

	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Divide timeout equally among pools (parallel shutdown)
	// Each pool gets the full timeout since they run in parallel
	result := make(map[string][]string)

	// Use channels to collect results from parallel graceful stops
	type poolResult struct {
		phase  string
		jobIDs []string
	}
	resultChan := make(chan poolResult, 3)

	// Gracefully stop all pools in parallel
	go func() {
		jobIDs := pm.metadataPool.GracefulStop(timeout)
		resultChan <- poolResult{phase: "metadata", jobIDs: jobIDs}
	}()
	go func() {
		jobIDs := pm.thumbnailPool.GracefulStop(timeout)
		resultChan <- poolResult{phase: "thumbnail", jobIDs: jobIDs}
	}()
	go func() {
		jobIDs := pm.spritesPool.GracefulStop(timeout)
		resultChan <- poolResult{phase: "sprites", jobIDs: jobIDs}
	}()

	// Collect results
	for i := 0; i < 3; i++ {
		res := <-resultChan
		if len(res.jobIDs) > 0 {
			result[res.phase] = res.jobIDs
		}
	}

	totalReclaimed := 0
	for _, ids := range result {
		totalReclaimed += len(ids)
	}

	pm.logger.Info("Pool manager graceful shutdown complete",
		zap.Int("total_jobs_reclaimed", totalReclaimed),
		zap.Int("metadata_reclaimed", len(result["metadata"])),
		zap.Int("thumbnail_reclaimed", len(result["thumbnail"])),
		zap.Int("sprites_reclaimed", len(result["sprites"])),
	)

	return result
}

// migrateOldThumbnails renames legacy {id}_thumb.webp files to the new {id}_thumb_sm.webp naming.
func (pm *PoolManager) migrateOldThumbnails() {
	entries, err := os.ReadDir(pm.config.ThumbnailDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, "_thumb.webp") {
			oldPath := filepath.Join(pm.config.ThumbnailDir, name)
			newName := strings.TrimSuffix(name, "_thumb.webp") + "_thumb_sm.webp"
			newPath := filepath.Join(pm.config.ThumbnailDir, newName)
			if err := os.Rename(oldPath, newPath); err != nil {
				pm.logger.Error("Failed to migrate old thumbnail",
					zap.String("old_path", oldPath),
					zap.String("new_path", newPath),
					zap.Error(err),
				)
			} else {
				pm.logger.Info("Migrated old thumbnail",
					zap.String("old_path", oldPath),
					zap.String("new_path", newPath),
				)
			}
		}
	}
}

// GetPoolConfig returns the current pool configuration
func (pm *PoolManager) GetPoolConfig() PoolConfig {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return PoolConfig{
		MetadataWorkers:  pm.metadataPool.ActiveWorkers(),
		ThumbnailWorkers: pm.thumbnailPool.ActiveWorkers(),
		SpritesWorkers:   pm.spritesPool.ActiveWorkers(),
	}
}

// GetQueueStatus returns the current queue status
func (pm *PoolManager) GetQueueStatus() QueueStatus {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return QueueStatus{
		MetadataQueued:  pm.metadataPool.QueueSize(),
		ThumbnailQueued: pm.thumbnailPool.QueueSize(),
		SpritesQueued:   pm.spritesPool.QueueSize(),
	}
}

// GetQualityConfig returns the current quality configuration
func (pm *PoolManager) GetQualityConfig() QualityConfig {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.qualityConfig
}

// GetConfig returns the processing config
func (pm *PoolManager) GetConfig() config.ProcessingConfig {
	return pm.config
}

var validDimensionsSm = map[int]bool{160: true, 240: true, 320: true, 480: true}
var validDimensionsLg = map[int]bool{640: true, 720: true, 960: true, 1280: true, 1920: true}

// UpdateQualityConfig updates the quality configuration
func (pm *PoolManager) UpdateQualityConfig(cfg QualityConfig) error {
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

	pm.mu.Lock()
	pm.qualityConfig = cfg
	pm.mu.Unlock()

	pm.logger.Info("Updated processing quality config",
		zap.Int("max_frame_dimension_sm", cfg.MaxFrameDimensionSm),
		zap.Int("max_frame_dimension_lg", cfg.MaxFrameDimensionLg),
		zap.Int("frame_quality_sm", cfg.FrameQualitySm),
		zap.Int("frame_quality_lg", cfg.FrameQualityLg),
		zap.Int("frame_quality_sprites", cfg.FrameQualitySprites),
		zap.Int("sprites_concurrency", cfg.SpritesConcurrency),
	)

	return nil
}

// UpdatePoolConfig updates the pool sizes and resizes pools as needed
func (pm *PoolManager) UpdatePoolConfig(cfg PoolConfig) error {
	if cfg.MetadataWorkers < 1 || cfg.MetadataWorkers > 10 {
		return fmt.Errorf("metadata_workers must be between 1 and 10")
	}
	if cfg.ThumbnailWorkers < 1 || cfg.ThumbnailWorkers > 10 {
		return fmt.Errorf("thumbnail_workers must be between 1 and 10")
	}
	if cfg.SpritesWorkers < 1 || cfg.SpritesWorkers > 10 {
		return fmt.Errorf("sprites_workers must be between 1 and 10")
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	const queueBufferSize = 1000

	// Resize metadata pool if needed
	if cfg.MetadataWorkers != pm.metadataPool.ActiveWorkers() {
		newPool := jobs.NewWorkerPool(cfg.MetadataWorkers, queueBufferSize)
		newPool.SetLogger(pm.logger.With(zap.String("pool", "metadata")))
		newPool.Start()
		if pm.resultHandler != nil {
			go pm.resultHandler(newPool)
		}

		oldPool := pm.metadataPool
		pm.metadataPool = newPool
		oldPool.Stop()

		pm.logger.Info("Resized metadata pool", zap.Int("workers", cfg.MetadataWorkers))
	}

	// Resize thumbnail pool if needed
	if cfg.ThumbnailWorkers != pm.thumbnailPool.ActiveWorkers() {
		newPool := jobs.NewWorkerPool(cfg.ThumbnailWorkers, queueBufferSize)
		newPool.SetLogger(pm.logger.With(zap.String("pool", "thumbnail")))
		newPool.Start()
		if pm.resultHandler != nil {
			go pm.resultHandler(newPool)
		}

		oldPool := pm.thumbnailPool
		pm.thumbnailPool = newPool
		oldPool.Stop()

		pm.logger.Info("Resized thumbnail pool", zap.Int("workers", cfg.ThumbnailWorkers))
	}

	// Resize sprites pool if needed
	if cfg.SpritesWorkers != pm.spritesPool.ActiveWorkers() {
		newPool := jobs.NewWorkerPool(cfg.SpritesWorkers, queueBufferSize)
		newPool.SetLogger(pm.logger.With(zap.String("pool", "sprites")))
		newPool.Start()
		if pm.resultHandler != nil {
			go pm.resultHandler(newPool)
		}

		oldPool := pm.spritesPool
		pm.spritesPool = newPool
		oldPool.Stop()

		pm.logger.Info("Resized sprites pool", zap.Int("workers", cfg.SpritesWorkers))
	}

	return nil
}

// CancelJob cancels a running job by its ID. It searches all pools.
func (pm *PoolManager) CancelJob(jobID string) error {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if err := pm.metadataPool.CancelJob(jobID); err == nil {
		pm.logger.Info("Job cancelled in metadata pool", zap.String("job_id", jobID))
		return nil
	}

	if err := pm.thumbnailPool.CancelJob(jobID); err == nil {
		pm.logger.Info("Job cancelled in thumbnail pool", zap.String("job_id", jobID))
		return nil
	}

	if err := pm.spritesPool.CancelJob(jobID); err == nil {
		pm.logger.Info("Job cancelled in sprites pool", zap.String("job_id", jobID))
		return nil
	}

	return fmt.Errorf("job not found: %s", jobID)
}

// GetJob retrieves a job by its ID from any pool
func (pm *PoolManager) GetJob(jobID string) (jobs.Job, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if job, ok := pm.metadataPool.GetJob(jobID); ok {
		return job, true
	}
	if job, ok := pm.thumbnailPool.GetJob(jobID); ok {
		return job, true
	}
	if job, ok := pm.spritesPool.GetJob(jobID); ok {
		return job, true
	}
	return nil, false
}

// SubmitToMetadataPool submits a job to the metadata pool
func (pm *PoolManager) SubmitToMetadataPool(job jobs.Job) error {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.metadataPool.Submit(job)
}

// SubmitToThumbnailPool submits a job to the thumbnail pool
func (pm *PoolManager) SubmitToThumbnailPool(job jobs.Job) error {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.thumbnailPool.Submit(job)
}

// SubmitToSpritesPool submits a job to the sprites pool
func (pm *PoolManager) SubmitToSpritesPool(job jobs.Job) error {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.spritesPool.Submit(job)
}

// LogStatus logs the status of all pools
func (pm *PoolManager) LogStatus() {
	pm.logger.Info("Pool manager status")
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	pm.metadataPool.LogStatus()
	pm.thumbnailPool.LogStatus()
	pm.spritesPool.LogStatus()
}
