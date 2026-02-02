package streaming

import (
	"fmt"
	"time"

	"goonhub/internal/config"
	"goonhub/internal/data"

	"go.uber.org/zap"
)

// Manager coordinates all streaming components (limiter, buffer pool, path cache).
// It provides a unified interface for the streaming handler.
type Manager struct {
	limiter    *StreamLimiter
	bufferPool *BufferPool
	pathCache  *PathCache
	sceneRepo  data.SceneRepository
	logger     *zap.Logger
}

// NewManager creates a new streaming manager with all components initialized.
func NewManager(cfg *config.StreamingConfig, sceneRepo data.SceneRepository, logger *zap.Logger) *Manager {
	return &Manager{
		limiter:    NewStreamLimiter(cfg.MaxGlobalStreams, cfg.MaxStreamsPerIP),
		bufferPool: NewBufferPool(cfg.BufferSize),
		pathCache:  NewPathCache(cfg.PathCacheTTL, cfg.PathCacheMaxSize),
		sceneRepo:  sceneRepo,
		logger:     logger,
	}
}

// Limiter returns the stream limiter for concurrent stream management.
func (m *Manager) Limiter() *StreamLimiter {
	return m.limiter
}

// BufferPool returns the buffer pool for efficient streaming.
func (m *Manager) BufferPool() *BufferPool {
	return m.bufferPool
}

// PathCache returns the path cache for scene file paths.
func (m *Manager) PathCache() *PathCache {
	return m.pathCache
}

// GetScenePath retrieves the stored path for a scene, using cache when possible.
// Returns the path and nil if found, empty string and error if not found or on DB error.
func (m *Manager) GetScenePath(sceneID uint) (string, error) {
	// Try cache first
	if path, ok := m.pathCache.Get(sceneID); ok {
		return path, nil
	}

	// Cache miss - query database
	scene, err := m.sceneRepo.GetByID(sceneID)
	if err != nil {
		return "", fmt.Errorf("failed to get scene %d: %w", sceneID, err)
	}
	if scene == nil {
		return "", fmt.Errorf("scene %d not found", sceneID)
	}

	// Store in cache for future requests
	m.pathCache.Set(sceneID, scene.StoredPath)

	return scene.StoredPath, nil
}

// InvalidateScenePath removes a scene from the path cache.
// Call this when a scene's stored path is updated.
func (m *Manager) InvalidateScenePath(sceneID uint) {
	m.pathCache.Invalidate(sceneID)
}

// Stats returns combined statistics from all components.
func (m *Manager) Stats() ManagerStats {
	return ManagerStats{
		Stream:    m.limiter.Stats(),
		CacheSize: m.pathCache.Size(),
	}
}

// Stop gracefully stops all background goroutines.
func (m *Manager) Stop() {
	m.logger.Info("Stopping streaming manager...")

	m.limiter.Stop()
	m.pathCache.Stop()

	m.logger.Info("Streaming manager stopped")
}

// ManagerStats combines statistics from all streaming components.
type ManagerStats struct {
	Stream    StreamStats `json:"stream"`
	CacheSize int         `json:"cache_size"`
}

// DefaultConfig returns a default streaming configuration.
func DefaultConfig() *config.StreamingConfig {
	return &config.StreamingConfig{
		MaxGlobalStreams: 100,
		MaxStreamsPerIP:  10,
		BufferSize:       262144, // 256KB
		PathCacheTTL:     5 * time.Minute,
		PathCacheMaxSize: 10000,
	}
}
