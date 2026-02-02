package handler

import (
	"fmt"
	"goonhub/internal/api/v1/validators"
	"goonhub/internal/core"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// JobHandler handles job-related requests
type JobHandler struct {
	jobHistoryService *core.JobHistoryService
	processingService *core.SceneProcessingService
}

// NewJobHandler creates a new JobHandler
func NewJobHandler(
	jobHistoryService *core.JobHistoryService,
	processingService *core.SceneProcessingService,
) *JobHandler {
	return &JobHandler{
		jobHistoryService: jobHistoryService,
		processingService: processingService,
	}
}

// ListJobs returns paginated job history with queue status
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

	// Use worker pool atomic active counters for accurate running numbers.
	// This matches the same data source used by JobStatusService for the SSE header,
	// avoiding the race-prone (DB count - channel size) calculation.
	pendingByPhase, _ := h.jobHistoryService.CountPendingByPhase()

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
			"metadata_queued":   queueStatus.MetadataQueued,
			"thumbnail_queued":  queueStatus.ThumbnailQueued,
			"sprites_queued":    queueStatus.SpritesQueued,
			"metadata_running":  queueStatus.MetadataActive,
			"thumbnail_running": queueStatus.ThumbnailActive,
			"sprites_running":   queueStatus.SpritesActive,
			"metadata_pending":  pendingByPhase["metadata"],
			"thumbnail_pending": pendingByPhase["thumbnail"],
			"sprites_pending":   pendingByPhase["sprites"],
		},
	})
}

// TriggerPhase manually triggers a processing phase for a scene
func (h *JobHandler) TriggerPhase(c *gin.Context) {
	idStr := c.Param("id")
	sceneID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scene ID"})
		return
	}

	phase := c.Param("phase")
	if err := validators.ValidatePhase(phase); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.processingService.SubmitPhase(uint(sceneID), phase); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Phase %s triggered for scene %d", phase, sceneID)})
}

// TriggerBulkPhase triggers a processing phase for multiple scenes
func (h *JobHandler) TriggerBulkPhase(c *gin.Context) {
	var req struct {
		Phase string `json:"phase"`
		Mode  string `json:"mode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := validators.ValidatePhase(req.Phase); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := validators.ValidateJobMode(req.Mode); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.processingService.SubmitBulkPhase(req.Phase, req.Mode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   fmt.Sprintf("Bulk %s phase triggered (%s mode)", req.Phase, req.Mode),
		"submitted": result.Submitted,
		"skipped":   result.Skipped,
		"errors":    result.Errors,
	})
}

// CancelJob cancels a running job
func (h *JobHandler) CancelJob(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "job ID is required"})
		return
	}

	if err := h.processingService.CancelJob(jobID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Job cancelled", "job_id": jobID})
}
