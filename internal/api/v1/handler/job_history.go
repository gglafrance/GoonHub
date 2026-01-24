package handler

import (
	"goonhub/internal/core"
	"goonhub/internal/data"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type JobHandler struct {
	jobHistoryService    *core.JobHistoryService
	processingService    *core.VideoProcessingService
	poolConfigRepo       data.PoolConfigRepository
	processingConfigRepo data.ProcessingConfigRepository
}

func NewJobHandler(jobHistoryService *core.JobHistoryService, processingService *core.VideoProcessingService, poolConfigRepo data.PoolConfigRepository, processingConfigRepo data.ProcessingConfigRepository) *JobHandler {
	return &JobHandler{
		jobHistoryService:    jobHistoryService,
		processingService:    processingService,
		poolConfigRepo:       poolConfigRepo,
		processingConfigRepo: processingConfigRepo,
	}
}

func (h *JobHandler) ListJobs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	jobs, total, err := h.jobHistoryService.ListJobs(page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list jobs"})
		return
	}

	activeJobs, err := h.jobHistoryService.ListActiveJobs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list active jobs"})
		return
	}

	poolConfig := h.processingService.GetPoolConfig()
	queueStatus := h.processingService.GetQueueStatus()

	// Count active (submitted) jobs per phase from DB
	metadataActive := 0
	thumbnailActive := 0
	spritesActive := 0
	for _, aj := range activeJobs {
		switch aj.Phase {
		case "metadata":
			metadataActive++
		case "thumbnail":
			thumbnailActive++
		case "sprites":
			spritesActive++
		}
	}

	// True running = active in DB minus those still in the queue buffer
	metadataRunning := metadataActive - queueStatus.MetadataQueued
	thumbnailRunning := thumbnailActive - queueStatus.ThumbnailQueued
	spritesRunning := spritesActive - queueStatus.SpritesQueued
	if metadataRunning < 0 {
		metadataRunning = 0
	}
	if thumbnailRunning < 0 {
		thumbnailRunning = 0
	}
	if spritesRunning < 0 {
		spritesRunning = 0
	}

	c.JSON(http.StatusOK, gin.H{
		"data":         jobs,
		"total":        total,
		"page":         page,
		"limit":        limit,
		"active_count": len(activeJobs),
		"active_jobs":  activeJobs,
		"retention":    h.jobHistoryService.GetRetention(),
		"pool_config":  poolConfig,
		"queue_status": gin.H{
			"metadata_queued":  queueStatus.MetadataQueued,
			"thumbnail_queued": queueStatus.ThumbnailQueued,
			"sprites_queued":   queueStatus.SpritesQueued,
			"metadata_running":  metadataRunning,
			"thumbnail_running": thumbnailRunning,
			"sprites_running":   spritesRunning,
		},
	})
}

func (h *JobHandler) GetPoolConfig(c *gin.Context) {
	poolConfig := h.processingService.GetPoolConfig()
	c.JSON(http.StatusOK, poolConfig)
}

func (h *JobHandler) UpdatePoolConfig(c *gin.Context) {
	var req core.PoolConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if req.MetadataWorkers < 1 || req.MetadataWorkers > 10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "metadata_workers must be between 1 and 10"})
		return
	}
	if req.ThumbnailWorkers < 1 || req.ThumbnailWorkers > 10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "thumbnail_workers must be between 1 and 10"})
		return
	}
	if req.SpritesWorkers < 1 || req.SpritesWorkers > 10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sprites_workers must be between 1 and 10"})
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

func (h *JobHandler) GetProcessingConfig(c *gin.Context) {
	cfg := h.processingService.GetProcessingQualityConfig()
	c.JSON(http.StatusOK, cfg)
}

func (h *JobHandler) UpdateProcessingConfig(c *gin.Context) {
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
