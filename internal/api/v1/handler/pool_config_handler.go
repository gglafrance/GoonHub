package handler

import (
	"goonhub/internal/api/v1/response"
	"goonhub/internal/api/v1/validators"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"net/http"

	"github.com/gin-gonic/gin"
)

// PoolConfigHandler handles worker pool configuration requests
type PoolConfigHandler struct {
	processingService *core.SceneProcessingService
	poolConfigRepo    data.PoolConfigRepository
	schedulingService *core.JobSchedulingService
}

// NewPoolConfigHandler creates a new PoolConfigHandler
func NewPoolConfigHandler(
	processingService *core.SceneProcessingService,
	poolConfigRepo data.PoolConfigRepository,
	schedulingService *core.JobSchedulingService,
) *PoolConfigHandler {
	return &PoolConfigHandler{
		processingService: processingService,
		poolConfigRepo:    poolConfigRepo,
		schedulingService: schedulingService,
	}
}

// GetPoolConfig returns the configured pool sizes (without the quiet-hours overrides)
func (h *PoolConfigHandler) GetPoolConfig(c *gin.Context) {
	poolConfig := h.processingService.GetBasePoolConfig()
	c.JSON(http.StatusOK, poolConfig)
}

// jobScheduleResponse is the body of the job schedule endpoints: the settings and the
// effective state they produce now.
type jobScheduleResponse struct {
	Config core.JobScheduleConfig `json:"config"`
	Status core.JobScheduleStatus `json:"status"`
}

func (h *PoolConfigHandler) respondJobSchedule(c *gin.Context) {
	status, err := h.schedulingService.Status()
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, jobScheduleResponse{Config: h.schedulingService.GetConfig(), Status: status})
}

// GetJobSchedule returns the job scheduling settings and the current effective state
func (h *PoolConfigHandler) GetJobSchedule(c *gin.Context) {
	if h.schedulingService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Job scheduling not available"})
		return
	}
	h.respondJobSchedule(c)
}

// UpdateJobSchedule replaces the job scheduling settings; the feeder applies them within seconds
func (h *PoolConfigHandler) UpdateJobSchedule(c *gin.Context) {
	if h.schedulingService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Job scheduling not available"})
		return
	}
	var req core.JobScheduleConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if _, err := h.schedulingService.UpdateConfig(req); err != nil {
		response.Error(c, err)
		return
	}
	h.respondJobSchedule(c)
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
		MetadataWorkers:           req.MetadataWorkers,
		ThumbnailWorkers:          req.ThumbnailWorkers,
		SpritesWorkers:            req.SpritesWorkers,
		AnimatedThumbnailsWorkers: req.AnimatedThumbnailsWorkers,
	}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.processingService.UpdatePoolConfig(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update pool config: " + err.Error()})
		return
	}

	record := &data.PoolConfigRecord{
		MetadataWorkers:           req.MetadataWorkers,
		ThumbnailWorkers:          req.ThumbnailWorkers,
		SpritesWorkers:            req.SpritesWorkers,
		AnimatedThumbnailsWorkers: req.AnimatedThumbnailsWorkers,
		FingerprintWorkers:        req.FingerprintWorkers,
	}
	if err := h.poolConfigRepo.Upsert(record); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Pool config applied but failed to persist: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, h.processingService.GetBasePoolConfig())
}
