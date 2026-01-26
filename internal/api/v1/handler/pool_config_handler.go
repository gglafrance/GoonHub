package handler

import (
	"goonhub/internal/api/v1/validators"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"net/http"

	"github.com/gin-gonic/gin"
)

// PoolConfigHandler handles worker pool configuration requests
type PoolConfigHandler struct {
	processingService *core.VideoProcessingService
	poolConfigRepo    data.PoolConfigRepository
}

// NewPoolConfigHandler creates a new PoolConfigHandler
func NewPoolConfigHandler(
	processingService *core.VideoProcessingService,
	poolConfigRepo data.PoolConfigRepository,
) *PoolConfigHandler {
	return &PoolConfigHandler{
		processingService: processingService,
		poolConfigRepo:    poolConfigRepo,
	}
}

// GetPoolConfig returns the current pool configuration
func (h *PoolConfigHandler) GetPoolConfig(c *gin.Context) {
	poolConfig := h.processingService.GetPoolConfig()
	c.JSON(http.StatusOK, poolConfig)
}

// UpdatePoolConfig updates the pool configuration
func (h *PoolConfigHandler) UpdatePoolConfig(c *gin.Context) {
	var req core.PoolConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate pool configuration
	if err := validators.ValidatePoolConfig(validators.PoolConfigInput{
		MetadataWorkers:  req.MetadataWorkers,
		ThumbnailWorkers: req.ThumbnailWorkers,
		SpritesWorkers:   req.SpritesWorkers,
	}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.processingService.UpdatePoolConfig(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update pool config: " + err.Error()})
		return
	}

	record := &data.PoolConfigRecord{
		MetadataWorkers:  req.MetadataWorkers,
		ThumbnailWorkers: req.ThumbnailWorkers,
		SpritesWorkers:   req.SpritesWorkers,
	}
	if err := h.poolConfigRepo.Upsert(record); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Pool config applied but failed to persist: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, h.processingService.GetPoolConfig())
}
