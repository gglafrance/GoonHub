package handler

import (
	"goonhub/internal/core"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type JobHandler struct {
	jobHistoryService *core.JobHistoryService
}

func NewJobHandler(jobHistoryService *core.JobHistoryService) *JobHandler {
	return &JobHandler{
		jobHistoryService: jobHistoryService,
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

	c.JSON(http.StatusOK, gin.H{
		"data":         jobs,
		"total":        total,
		"page":         page,
		"limit":        limit,
		"active_count": len(activeJobs),
		"retention":    h.jobHistoryService.GetRetention(),
	})
}
