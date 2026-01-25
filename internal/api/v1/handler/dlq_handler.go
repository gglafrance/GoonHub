package handler

import (
	"goonhub/internal/core"
	"goonhub/internal/data"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// DLQHandler handles dead letter queue requests
type DLQHandler struct {
	dlqService *core.DLQService
}

// NewDLQHandler creates a new DLQHandler
func NewDLQHandler(dlqService *core.DLQService) *DLQHandler {
	return &DLQHandler{
		dlqService: dlqService,
	}
}

// ListDLQ returns paginated dead letter queue entries
func (h *DLQHandler) ListDLQ(c *gin.Context) {
	if h.dlqService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "DLQ service not available"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	status := c.DefaultQuery("status", "")

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	var entries []data.DLQEntry
	var total int64
	var err error

	if status != "" {
		entries, total, err = h.dlqService.ListByStatus(status, page, limit)
	} else {
		entries, total, err = h.dlqService.ListAll(page, limit)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list DLQ entries"})
		return
	}

	stats, _ := h.dlqService.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"data":  entries,
		"total": total,
		"page":  page,
		"limit": limit,
		"stats": stats,
	})
}

// RetryFromDLQ retries a job from the dead letter queue
func (h *DLQHandler) RetryFromDLQ(c *gin.Context) {
	if h.dlqService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "DLQ service not available"})
		return
	}

	jobID := c.Param("job_id")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "job_id is required"})
		return
	}

	if err := h.dlqService.RetryFromDLQ(jobID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Job resubmitted from DLQ", "job_id": jobID})
}

// AbandonDLQ marks a DLQ entry as abandoned
func (h *DLQHandler) AbandonDLQ(c *gin.Context) {
	if h.dlqService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "DLQ service not available"})
		return
	}

	jobID := c.Param("job_id")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "job_id is required"})
		return
	}

	if err := h.dlqService.Abandon(jobID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "DLQ entry abandoned", "job_id": jobID})
}
