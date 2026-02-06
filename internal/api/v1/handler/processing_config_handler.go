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
	markerService        *core.MarkerService
}

// NewProcessingConfigHandler creates a new ProcessingConfigHandler
func NewProcessingConfigHandler(
	processingService *core.SceneProcessingService,
	processingConfigRepo data.ProcessingConfigRepository,
	markerService *core.MarkerService,
) *ProcessingConfigHandler {
	return &ProcessingConfigHandler{
		processingService:    processingService,
		processingConfigRepo: processingConfigRepo,
		markerService:        markerService,
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

	// Propagate scene preview config to marker service
	h.markerService.SetScenePreviewEnabled(req.ScenePreviewEnabled)
	if req.ScenePreviewSegments > 0 {
		h.markerService.SetScenePreviewSegments(req.ScenePreviewSegments)
	}
	if req.ScenePreviewSegmentDuration > 0 {
		h.markerService.SetScenePreviewSegmentDuration(req.ScenePreviewSegmentDuration)
	}
	// Also propagate marker thumbnail settings
	if req.MarkerThumbnailType != "" {
		h.markerService.SetMarkerThumbnailType(req.MarkerThumbnailType)
	}
	if req.MarkerAnimatedDuration > 0 {
		h.markerService.SetMarkerAnimatedDuration(req.MarkerAnimatedDuration)
	}
	if req.MarkerPreviewCRF > 0 {
		h.markerService.SetMarkerPreviewCRF(req.MarkerPreviewCRF)
	}
	if req.ScenePreviewCRF > 0 {
		h.markerService.SetScenePreviewCRF(req.ScenePreviewCRF)
	}

	record := &data.ProcessingConfigRecord{
		MaxFrameDimensionSm:         req.MaxFrameDimensionSm,
		MaxFrameDimensionLg:         req.MaxFrameDimensionLg,
		FrameQualitySm:              req.FrameQualitySm,
		FrameQualityLg:              req.FrameQualityLg,
		FrameQualitySprites:         req.FrameQualitySprites,
		SpritesConcurrency:          req.SpritesConcurrency,
		MarkerThumbnailType:         req.MarkerThumbnailType,
		MarkerAnimatedDuration:      req.MarkerAnimatedDuration,
		ScenePreviewEnabled:         req.ScenePreviewEnabled,
		ScenePreviewSegments:        req.ScenePreviewSegments,
		ScenePreviewSegmentDuration: req.ScenePreviewSegmentDuration,
		MarkerPreviewCRF:            req.MarkerPreviewCRF,
		ScenePreviewCRF:             req.ScenePreviewCRF,
	}
	if err := h.processingConfigRepo.Upsert(record); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Processing config applied but failed to persist: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, h.processingService.GetProcessingQualityConfig())
}
