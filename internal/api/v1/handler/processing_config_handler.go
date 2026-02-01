package handler

import (
	"goonhub/internal/core"
	"goonhub/internal/data"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ProcessingConfigHandler handles processing quality configuration requests
type ProcessingConfigHandler struct {
	processingService    *core.SceneProcessingService
	processingConfigRepo data.ProcessingConfigRepository
}

// NewProcessingConfigHandler creates a new ProcessingConfigHandler
func NewProcessingConfigHandler(
	processingService *core.SceneProcessingService,
	processingConfigRepo data.ProcessingConfigRepository,
) *ProcessingConfigHandler {
	return &ProcessingConfigHandler{
		processingService:    processingService,
		processingConfigRepo: processingConfigRepo,
	}
}

// GetProcessingConfig returns the current processing quality configuration
func (h *ProcessingConfigHandler) GetProcessingConfig(c *gin.Context) {
	cfg := h.processingService.GetProcessingQualityConfig()
	c.JSON(http.StatusOK, cfg)
}

// UpdateProcessingConfig updates the processing quality configuration
func (h *ProcessingConfigHandler) UpdateProcessingConfig(c *gin.Context) {
	var req core.ProcessingQualityConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := h.processingService.UpdateProcessingQualityConfig(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	record := &data.ProcessingConfigRecord{
		MaxFrameDimensionSm: req.MaxFrameDimensionSm,
		MaxFrameDimensionLg: req.MaxFrameDimensionLg,
		FrameQualitySm:      req.FrameQualitySm,
		FrameQualityLg:      req.FrameQualityLg,
		FrameQualitySprites: req.FrameQualitySprites,
		SpritesConcurrency:  req.SpritesConcurrency,
	}
	if err := h.processingConfigRepo.Upsert(record); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Processing config applied but failed to persist: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, h.processingService.GetProcessingQualityConfig())
}
