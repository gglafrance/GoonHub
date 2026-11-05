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
	metadataPool            *jobs.WorkerPool
	thumbnailPool           *jobs.WorkerPool
	spritesPool             *jobs.WorkerPool
	animatedThumbnailsPool  *jobs.WorkerPool
	fingerprintPool         *jobs.WorkerPool
	duplicationEnabled      bool
	mu                      sync.RWMutex
	config                  config.ProcessingConfig
	qualityConfig           QualityConfig
	logger                  *zap.Logger

	// resultHandler is called when a job completes
	resultHandler func(*jobs.WorkerPool)

	// onJobExecuting is called when a worker picks up a job for execution
	onJobExecuting func(jobID string)
}

// NewPoolManager creates a new PoolManager with the given configuration
func NewPoolManager(
	cfg config.ProcessingConfig,
	logger *zap.Logger,
	poolConfigRepo data.PoolConfigRepository,
	processingConfigRepo data.ProcessingConfigRepository,
	duplicationCfg *config.DuplicationConfig,
) *PoolManager {
	// Check DB for persisted pool config overrides
	metadataWorkers := cfg.MetadataWorkers
	thumbnailWorkers := cfg.ThumbnailWorkers
	spritesWorkers := cfg.SpritesWorkers
	animatedThumbnailsWorkers := cfg.AnimatedThumbnailsWorkers
	if animatedThumbnailsWorkers <= 0 {
		animatedThumbnailsWorkers = 1
	}

	if poolConfigRepo != nil {
		if dbConfig, err := poolConfigRepo.Get(); err == nil && dbConfig != nil {
			metadataWorkers = dbConfig.MetadataWorkers
			thumbnailWorkers = dbConfig.ThumbnailWorkers
			spritesWorkers = dbConfig.SpritesWorkers
			if dbConfig.AnimatedThumbnailsWorkers > 0 {
				animatedThumbnailsWorkers = dbConfig.AnimatedThumbnailsWorkers
			}
			logger.Info("Loaded pool config from database",
				zap.Int("metadata_workers", metadataWorkers),
				zap.Int("thumbnail_workers", thumbnailWorkers),
				zap.Int("sprites_workers", spritesWorkers),
				zap.Int("animated_thumbnails_workers", animatedThumbnailsWorkers),
			)
		}
	}

	// Initialize processing quality config from YAML defaults
	markerThumbnailType := cfg.MarkerThumbnailType
	if markerThumbnailType == "" {
		markerThumbnailType = "static"
	}
	markerAnimatedDuration := cfg.MarkerAnimatedDuration
	if markerAnimatedDuration <= 0 {
		markerAnimatedDuration = 10
	}

	scenePreviewSegments := cfg.ScenePreviewSegments
	if scenePreviewSegments <= 0 {
		scenePreviewSegments = 12
	}
	scenePreviewSegmentDuration := cfg.ScenePreviewSegmentDuration
	if scenePreviewSegmentDuration <= 0 {
		scenePreviewSegmentDuration = 1.0
	}

	markerPreviewCRF := cfg.MarkerPreviewCRF
	if markerPreviewCRF <= 0 {
		markerPreviewCRF = 32
	}
	scenePreviewCRF := cfg.ScenePreviewCRF
	if scenePreviewCRF <= 0 {
		scenePreviewCRF = 27
	}

	qualityConfig := QualityConfig{
		MaxFrameDimensionSm:         cfg.MaxFrameDimension,
		MaxFrameDimensionLg:         cfg.MaxFrameDimensionLarge,
		FrameQualitySm:              cfg.FrameQuality,
		FrameQualityLg:              cfg.FrameQualityLg,
		FrameQualitySprites:         cfg.FrameQualitySprites,
		SpritesConcurrency:          cfg.SpritesConcurrency,
		MarkerThumbnailType:         markerThumbnailType,
		MarkerAnimatedDuration:      markerAnimatedDuration,
		ScenePreviewEnabled:         cfg.ScenePreviewEnabled,
		ScenePreviewSegments:        scenePreviewSegments,
		ScenePreviewSegmentDuration: scenePreviewSegmentDuration,
		MarkerPreviewCRF:            markerPreviewCRF,
		ScenePreviewCRF:             scenePreviewCRF,
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
			if dbConfig.MarkerThumbnailType != "" {
				qualityConfig.MarkerThumbnailType = dbConfig.MarkerThumbnailType
			}
			if dbConfig.MarkerAnimatedDuration > 0 {
				qualityConfig.MarkerAnimatedDuration = dbConfig.MarkerAnimatedDuration
			}
			qualityConfig.ScenePreviewEnabled = dbConfig.ScenePreviewEnabled
			if dbConfig.ScenePreviewSegments > 0 {
				qualityConfig.ScenePreviewSegments = dbConfig.ScenePreviewSegments
			}
			if dbConfig.ScenePreviewSegmentDuration > 0 {
				qualityConfig.ScenePreviewSegmentDuration = dbConfig.ScenePreviewSegmentDuration
			}
			if dbConfig.MarkerPreviewCRF > 0 {
				qualityConfig.MarkerPreviewCRF = dbConfig.MarkerPreviewCRF
			}
			if dbConfig.ScenePreviewCRF > 0 {
				qualityConfig.ScenePreviewCRF = dbConfig.ScenePreviewCRF
			}
			logger.Info("Loaded processing config from database",
				zap.Int("max_frame_dimension_sm", qualityConfig.MaxFrameDimensionSm),
				zap.Int("max_frame_dimension_lg", qualityConfig.MaxFrameDimensionLg),
				zap.Int("frame_quality_sm", qualityConfig.FrameQualitySm),
				zap.Int("frame_quality_lg", qualityConfig.FrameQualityLg),
				zap.Int("frame_quality_sprites", qualityConfig.FrameQualitySprites),
				zap.Int("sprites_concurrency", qualityConfig.SpritesConcurrency),
				zap.String("marker_thumbnail_type", qualityConfig.MarkerThumbnailType),
				zap.Int("marker_animated_duration", qualityConfig.MarkerAnimatedDuration),
				zap.Bool("scene_preview_enabled", qualityConfig.ScenePreviewEnabled),
				zap.Int("scene_preview_segments", qualityConfig.ScenePreviewSegments),
				zap.Float64("scene_preview_segment_duration", qualityConfig.ScenePreviewSegmentDuration),
				zap.Int("marker_preview_crf", qualityConfig.MarkerPreviewCRF),
				zap.Int("scene_preview_crf", qualityConfig.ScenePreviewCRF),
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

	animatedThumbnailsPool := jobs.NewWorkerPool(animatedThumbnailsWorkers, queueBufferSize)
	animatedThumbnailsPool.SetLogger(logger.With(zap.String("pool", "animated_thumbnails")))
	if cfg.AnimatedThumbnailsTimeout > 0 {
		animatedThumbnailsPool.SetTimeout(cfg.AnimatedThumbnailsTimeout)
		logger.Info("Animated thumbnails pool timeout set", zap.Duration("timeout", cfg.AnimatedThumbnailsTimeout))
	}

	// Create fingerprint pool if duplication is enabled
	var fingerprintPool *jobs.WorkerPool
	if duplicationCfg != nil && duplicationCfg.Enabled {
		fpWorkers := duplicationCfg.FingerprintWorkers
		if fpWorkers <= 0 {
			fpWorkers = 1
		}
		if poolConfigRepo != nil {
			if dbConfig, err := poolConfigRepo.Get(); err == nil && dbConfig != nil && dbConfig.FingerprintWorkers > 0 {
				fpWorkers = dbConfig.FingerprintWorkers
			}
		}
		fingerprintPool = jobs.NewWorkerPool(fpWorkers, queueBufferSize)
		fingerprintPool.SetLogger(logger.With(zap.String("pool", "fingerprint")))
		if duplicationCfg.FingerprintTimeout > 0 {
			fingerprintPool.SetTimeout(duplicationCfg.FingerprintTimeout)
		}
	}

	// Create output directories
	createDirIfNotExists(cfg.SpriteDir, logger)
	createDirIfNotExists(cfg.VttDir, logger)
	createDirIfNotExists(cfg.ThumbnailDir, logger)
	createDirIfNotExists(cfg.MarkerThumbnailDir, logger)
	createDirIfNotExists(cfg.ScenePreviewDir, logger)

	return &PoolManager{
		metadataPool:           metadataPool,
		thumbnailPool:          thumbnailPool,
		spritesPool:            spritesPool,
		animatedThumbnailsPool: animatedThumbnailsPool,
		fingerprintPool:        fingerprintPool,
		duplicationEnabled:     duplicationCfg != nil && duplicationCfg.Enabled,
		config:                 cfg,
		qualityConfig:          qualityConfig,
		logger:                 logger,
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

// SetOnJobExecuting sets a callback invoked when a worker picks up a job for execution.
// Used to update started_at in the DB to reflect actual execution start time.
func (pm *PoolManager) SetOnJobExecuting(fn func(jobID string)) {
	pm.onJobExecuting = fn
	pm.metadataPool.SetOnJobExecuting(fn)
	pm.thumbnailPool.SetOnJobExecuting(fn)
	pm.spritesPool.SetOnJobExecuting(fn)
	pm.animatedThumbnailsPool.SetOnJobExecuting(fn)
	if pm.fingerprintPool != nil {
		pm.fingerprintPool.SetOnJobExecuting(fn)
	}
}

// Start starts all worker pools and their result handlers
func (pm *PoolManager) Start() {
	pm.migrateOldThumbnails()

	pm.metadataPool.Start()
	pm.thumbnailPool.Start()
	pm.spritesPool.Start()
	pm.animatedThumbnailsPool.Start()

	if pm.resultHandler != nil {
		go pm.resultHandler(pm.metadataPool)
		go pm.resultHandler(pm.thumbnailPool)
		go pm.resultHandler(pm.spritesPool)
		go pm.resultHandler(pm.animatedThumbnailsPool)
	}

	if pm.fingerprintPool != nil {
		pm.fingerprintPool.Start()
		if pm.resultHandler != nil {
			go pm.resultHandler(pm.fingerprintPool)
		}
	}

	logFields := []zap.Field{
		zap.Int("metadata_workers", pm.metadataPool.ActiveWorkers()),
		zap.Int("thumbnail_workers", pm.thumbnailPool.ActiveWorkers()),
		zap.Int("sprites_workers", pm.spritesPool.ActiveWorkers()),
		zap.Int("animated_thumbnails_workers", pm.animatedThumbnailsPool.ActiveWorkers()),
	}
	if pm.fingerprintPool != nil {
		logFields = append(logFields, zap.Int("fingerprint_workers", pm.fingerprintPool.ActiveWorkers()))
	}
	pm.logger.Info("Pool manager started", logFields...)
}

// Stop stops all worker pools
func (pm *PoolManager) Stop() {
	pm.logger.Info("Stopping pool manager")
	pm.metadataPool.Stop()
	pm.thumbnailPool.Stop()
	pm.spritesPool.Stop()
	pm.animatedThumbnailsPool.Stop()
	if pm.fingerprintPool != nil {
		pm.fingerprintPool.Stop()
	}
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

	poolCount := 4
	if pm.fingerprintPool != nil {
		poolCount = 5
	}
	resultChan := make(chan poolResult, poolCount)

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
	go func() {
		jobIDs := pm.animatedThumbnailsPool.GracefulStop(timeout)
		resultChan <- poolResult{phase: "animated_thumbnails", jobIDs: jobIDs}
	}()
	if pm.fingerprintPool != nil {
		go func() {
			jobIDs := pm.fingerprintPool.GracefulStop(timeout)
			resultChan <- poolResult{phase: "fingerprint", jobIDs: jobIDs}
		}()
	}

	// Collect results
	for i := 0; i < poolCount; i++ {
		res := <-resultChan
		if len(res.jobIDs) > 0 {
			result[res.phase] = res.jobIDs
		}
	}

	totalReclaimed := 0
	for _, ids := range result {
		totalReclaimed += len(ids)
	}

	logFields := []zap.Field{
		zap.Int("total_jobs_reclaimed", totalReclaimed),
		zap.Int("metadata_reclaimed", len(result["metadata"])),
		zap.Int("thumbnail_reclaimed", len(result["thumbnail"])),
		zap.Int("sprites_reclaimed", len(result["sprites"])),
		zap.Int("animated_thumbnails_reclaimed", len(result["animated_thumbnails"])),
	}
	if pm.fingerprintPool != nil {
		logFields = append(logFields, zap.Int("fingerprint_reclaimed", len(result["fingerprint"])))
	}
	pm.logger.Info("Pool manager graceful shutdown complete", logFields...)

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
	cfg := PoolConfig{
		MetadataWorkers:           pm.metadataPool.ActiveWorkers(),
		ThumbnailWorkers:          pm.thumbnailPool.ActiveWorkers(),
		SpritesWorkers:            pm.spritesPool.ActiveWorkers(),
		AnimatedThumbnailsWorkers: pm.animatedThumbnailsPool.ActiveWorkers(),
		DuplicationEnabled:        pm.duplicationEnabled,
	}
	if pm.fingerprintPool != nil {
		cfg.FingerprintWorkers = pm.fingerprintPool.ActiveWorkers()
	}
	return cfg
}

// GetQueueStatus returns the current queue status
func (pm *PoolManager) GetQueueStatus() QueueStatus {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	qs := QueueStatus{
		MetadataQueued:           pm.metadataPool.QueueSize(),
		ThumbnailQueued:          pm.thumbnailPool.QueueSize(),
		SpritesQueued:            pm.spritesPool.QueueSize(),
		AnimatedThumbnailsQueued: pm.animatedThumbnailsPool.QueueSize(),
		MetadataActive:           pm.metadataPool.ActiveJobCount(),
		ThumbnailActive:          pm.thumbnailPool.ActiveJobCount(),
		SpritesActive:            pm.spritesPool.ActiveJobCount(),
		AnimatedThumbnailsActive: pm.animatedThumbnailsPool.ActiveJobCount(),
	}
	if pm.fingerprintPool != nil {
		qs.FingerprintQueued = pm.fingerprintPool.QueueSize()
		qs.FingerprintActive = pm.fingerprintPool.ActiveJobCount()
	}
	return qs
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

var validMarkerThumbnailTypes = map[string]bool{"static": true, "animated": true}

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
	if cfg.MarkerThumbnailType != "" && !validMarkerThumbnailTypes[cfg.MarkerThumbnailType] {
		return fmt.Errorf("marker_thumbnail_type must be one of: static, animated")
	}
	if cfg.MarkerAnimatedDuration != 0 && (cfg.MarkerAnimatedDuration < 3 || cfg.MarkerAnimatedDuration > 15) {
		return fmt.Errorf("marker_animated_duration must be between 3 and 15")
	}
	if cfg.ScenePreviewSegments != 0 && (cfg.ScenePreviewSegments < 2 || cfg.ScenePreviewSegments > 24) {
		return fmt.Errorf("scene_preview_segments must be between 2 and 24")
	}
	if cfg.ScenePreviewSegmentDuration != 0 && (cfg.ScenePreviewSegmentDuration < 0.75 || cfg.ScenePreviewSegmentDuration > 5.0) {
		return fmt.Errorf("scene_preview_segment_duration must be between 0.75 and 5.0")
	}
	if cfg.MarkerPreviewCRF != 0 && (cfg.MarkerPreviewCRF < 18 || cfg.MarkerPreviewCRF > 40) {
		return fmt.Errorf("marker_preview_crf must be between 18 and 40")
	}
	if cfg.ScenePreviewCRF != 0 && (cfg.ScenePreviewCRF < 18 || cfg.ScenePreviewCRF > 40) {
		return fmt.Errorf("scene_preview_crf must be between 18 and 40")
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
		zap.String("marker_thumbnail_type", cfg.MarkerThumbnailType),
		zap.Int("marker_animated_duration", cfg.MarkerAnimatedDuration),
		zap.Bool("scene_preview_enabled", cfg.ScenePreviewEnabled),
		zap.Int("scene_preview_segments", cfg.ScenePreviewSegments),
		zap.Float64("scene_preview_segment_duration", cfg.ScenePreviewSegmentDuration),
		zap.Int("marker_preview_crf", cfg.MarkerPreviewCRF),
		zap.Int("scene_preview_crf", cfg.ScenePreviewCRF),
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
	if cfg.AnimatedThumbnailsWorkers < 1 || cfg.AnimatedThumbnailsWorkers > 10 {
		return fmt.Errorf("animated_thumbnails_workers must be between 1 and 10")
	}
	if pm.duplicationEnabled && (cfg.FingerprintWorkers < 1 || cfg.FingerprintWorkers > 10) {
		return fmt.Errorf("fingerprint_workers must be between 1 and 10")
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	const queueBufferSize = 1000

	// Resize metadata pool if needed
	if cfg.MetadataWorkers != pm.metadataPool.ActiveWorkers() {
		newPool := jobs.NewWorkerPool(cfg.MetadataWorkers, queueBufferSize)
		newPool.SetLogger(pm.logger.With(zap.String("pool", "metadata")))
		newPool.SetOnJobExecuting(pm.onJobExecuting)
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
		newPool.SetOnJobExecuting(pm.onJobExecuting)
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
		newPool.SetOnJobExecuting(pm.onJobExecuting)
		newPool.Start()
		if pm.resultHandler != nil {
			go pm.resultHandler(newPool)
		}

		oldPool := pm.spritesPool
		pm.spritesPool = newPool
		oldPool.Stop()

		pm.logger.Info("Resized sprites pool", zap.Int("workers", cfg.SpritesWorkers))
	}

	// Resize animated thumbnails pool if needed
	if cfg.AnimatedThumbnailsWorkers != pm.animatedThumbnailsPool.ActiveWorkers() {
		newPool := jobs.NewWorkerPool(cfg.AnimatedThumbnailsWorkers, queueBufferSize)
		newPool.SetLogger(pm.logger.With(zap.String("pool", "animated_thumbnails")))
		newPool.SetOnJobExecuting(pm.onJobExecuting)
		newPool.Start()
		if pm.resultHandler != nil {
			go pm.resultHandler(newPool)
		}

		oldPool := pm.animatedThumbnailsPool
		pm.animatedThumbnailsPool = newPool
		oldPool.Stop()

		pm.logger.Info("Resized animated thumbnails pool", zap.Int("workers", cfg.AnimatedThumbnailsWorkers))
	}

	// Resize fingerprint pool if needed (only if duplication is enabled)
	if pm.fingerprintPool != nil && cfg.FingerprintWorkers != pm.fingerprintPool.ActiveWorkers() {
		newPool := jobs.NewWorkerPool(cfg.FingerprintWorkers, queueBufferSize)
		newPool.SetLogger(pm.logger.With(zap.String("pool", "fingerprint")))
		newPool.SetOnJobExecuting(pm.onJobExecuting)
		newPool.Start()
		if pm.resultHandler != nil {
			go pm.resultHandler(newPool)
		}

		oldPool := pm.fingerprintPool
		pm.fingerprintPool = newPool
		oldPool.Stop()

		pm.logger.Info("Resized fingerprint pool", zap.Int("workers", cfg.FingerprintWorkers))
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

	if err := pm.animatedThumbnailsPool.CancelJob(jobID); err == nil {
		pm.logger.Info("Job cancelled in animated thumbnails pool", zap.String("job_id", jobID))
		return nil
	}

	if pm.fingerprintPool != nil {
		if err := pm.fingerprintPool.CancelJob(jobID); err == nil {
			pm.logger.Info("Job cancelled in fingerprint pool", zap.String("job_id", jobID))
			return nil
		}
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
	if job, ok := pm.animatedThumbnailsPool.GetJob(jobID); ok {
		return job, true
	}
	if pm.fingerprintPool != nil {
		if job, ok := pm.fingerprintPool.GetJob(jobID); ok {
			return job, true
		}
	}
	return nil, false
}

// GetExecutingJob retrieves a job by its ID only if it is actively being executed by a worker.
// Unlike GetJob, this excludes jobs sitting in channel buffers waiting to be picked up.
func (pm *PoolManager) GetExecutingJob(jobID string) (jobs.Job, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if job, ok := pm.metadataPool.GetExecutingJob(jobID); ok {
		return job, true
	}
	if job, ok := pm.thumbnailPool.GetExecutingJob(jobID); ok {
		return job, true
	}
	if job, ok := pm.spritesPool.GetExecutingJob(jobID); ok {
		return job, true
	}
	if job, ok := pm.animatedThumbnailsPool.GetExecutingJob(jobID); ok {
		return job, true
	}
	if pm.fingerprintPool != nil {
		if job, ok := pm.fingerprintPool.GetExecutingJob(jobID); ok {
			return job, true
		}
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

// SubmitToAnimatedThumbnailsPool submits a job to the animated thumbnails pool
func (pm *PoolManager) SubmitToAnimatedThumbnailsPool(job jobs.Job) error {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.animatedThumbnailsPool.Submit(job)
}

// SubmitToFingerprintPool submits a job to the fingerprint pool
func (pm *PoolManager) SubmitToFingerprintPool(job jobs.Job) error {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	if pm.fingerprintPool == nil {
		return fmt.Errorf("fingerprint pool not enabled: duplication detection is disabled")
	}
	return pm.fingerprintPool.Submit(job)
}

// IsDuplicationEnabled returns whether duplication detection is enabled
func (pm *PoolManager) IsDuplicationEnabled() bool {
	return pm.duplicationEnabled
}

// LogStatus logs the status of all pools
func (pm *PoolManager) LogStatus() {
	pm.logger.Info("Pool manager status")
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	pm.metadataPool.LogStatus()
	pm.thumbnailPool.LogStatus()
	pm.spritesPool.LogStatus()
	pm.animatedThumbnailsPool.LogStatus()
	if pm.fingerprintPool != nil {
		pm.fingerprintPool.LogStatus()
	}
}
