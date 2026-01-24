package handler

import (
	"goonhub/internal/core"
	"goonhub/internal/data"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type JobHandler struct {
	jobHistoryService   *core.JobHistoryService
	processingService   *core.VideoProcessingService
	poolConfigRepo      data.PoolConfigRepository
}

func NewJobHandler(jobHistoryService *core.JobHistoryService, processingService *core.VideoProcessingService, poolConfigRepo data.PoolConfigRepository) *JobHandler {
	return &JobHandler{
		jobHistoryService:   jobHistoryService,
		processingService:   processingService,
		poolConfigRepo:      poolConfigRepo,
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

	c.JSON(http.StatusOK, gin.H{
		"data":         jobs,
		"total":        total,
		"page":         page,
		"limit":        limit,
		"active_count": len(activeJobs),
		"retention":    h.jobHistoryService.GetRetention(),
		"pool_config":  poolConfig,
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
