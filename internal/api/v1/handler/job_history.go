package handler

import (
	"fmt"
	"goonhub/internal/api/v1/response"
	"goonhub/internal/api/v1/validators"
	"goonhub/internal/core"
	"goonhub/internal/data"
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
	status := c.Query("status")

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	jobs, total, err := h.jobHistoryService.ListJobs(page, limit, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list jobs"})
		return
	}

	activeJobs, err := h.jobHistoryService.ListActiveJobs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list active jobs"})
		return
	}

	// Filter active jobs to only those actually in a worker pool.
	// DB marks jobs as 'running' when claimed, but they may have completed
	// before the result handler updates the DB status.
	var verifiedActive []data.JobHistory
	for _, job := range activeJobs {
		if _, inPool := h.processingService.GetJob(job.JobID); inPool {
			verifiedActive = append(verifiedActive, job)
		}
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
		"active_count": len(verifiedActive),
		"active_jobs":  verifiedActive,
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

	forceTarget := c.Query("force_target")
	if forceTarget != "" {
		if phase != "animated_thumbnails" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "force_target is only supported for animated_thumbnails phase"})
			return
		}
		if err := validators.ValidateForceTarget(forceTarget); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	if err := h.processingService.SubmitPhaseWithForce(uint(sceneID), phase, 1, forceTarget); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Phase %s triggered for scene %d", phase, sceneID)})
}

// TriggerBulkPhase triggers a processing phase for multiple scenes
func (h *JobHandler) TriggerBulkPhase(c *gin.Context) {
	var req struct {
		Phase       string `json:"phase"`
		Mode        string `json:"mode"`
		ForceTarget string `json:"force_target"`
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

	if req.ForceTarget != "" {
		if req.Phase != "animated_thumbnails" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "force_target is only supported for animated_thumbnails phase"})
			return
		}
		if err := validators.ValidateForceTarget(req.ForceTarget); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	result, err := h.processingService.SubmitBulkPhase(req.Phase, req.Mode, req.ForceTarget)
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

// RetryJob manually retries a failed job
func (h *JobHandler) RetryJob(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "job ID is required"})
		return
	}

	if err := h.jobHistoryService.RetryJob(jobID); err != nil {
		response.Error(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Job retried", "job_id": jobID})
}

// ListRecentFailed returns recently failed jobs
func (h *JobHandler) ListRecentFailed(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))
	if limit < 1 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}

	jobs, err := h.jobHistoryService.ListRecentFailed(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list recent failed jobs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": jobs})
}

// RetryAllFailed retries all failed jobs
func (h *JobHandler) RetryAllFailed(c *gin.Context) {
	retried, err := h.jobHistoryService.RetryAllFailed()
	if err != nil {
		response.Error(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Retried %d failed jobs", retried),
		"retried": retried,
	})
}

// RetryBatch retries a batch of failed jobs by their IDs
func (h *JobHandler) RetryBatch(c *gin.Context) {
	var req struct {
		JobIDs []string `json:"job_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if len(req.JobIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "job_ids must not be empty"})
		return
	}
	if len(req.JobIDs) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "job_ids must not exceed 100 items"})
		return
	}

	retried, errors := h.jobHistoryService.RetryBatch(req.JobIDs)

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Retried %d jobs (%d errors)", retried, errors),
		"retried": retried,
		"errors":  errors,
	})
}

// ClearFailed deletes all failed jobs from history
func (h *JobHandler) ClearFailed(c *gin.Context) {
	deleted, err := h.jobHistoryService.ClearFailed()
	if err != nil {
		response.Error(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Cleared %d failed jobs", deleted),
		"deleted": deleted,
	})
}
