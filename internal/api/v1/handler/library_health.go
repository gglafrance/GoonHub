package handler

import (
	"strconv"

	"goonhub/internal/api/v1/response"
	"goonhub/internal/core"

	"github.com/gin-gonic/gin"
)

// LibraryHealthHandler serves the admin library health page.
type LibraryHealthHandler struct {
	service *core.LibraryHealthService
}

// NewLibraryHealthHandler creates a library health handler.
func NewLibraryHealthHandler(service *core.LibraryHealthService) *LibraryHealthHandler {
	return &LibraryHealthHandler{service: service}
}

// GetSummary returns the health counts, storage path usage and last task results.
// GET /api/v1/admin/library-health
func (h *LibraryHealthHandler) GetSummary(c *gin.Context) {
	summary, err := h.service.GetSummary()
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, summary)
}

// ListScenes lists the scenes of a health category.
// GET /api/v1/admin/library-health/scenes?category=size_zero&page=1&limit=20
func (h *LibraryHealthHandler) ListScenes(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	rows, total, err := h.service.ListScenes(c.Query("category"), page, limit)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, response.NewPaginatedResponse(rows, page, limit, total))
}

// PreviewAction counts what an action would touch (jobs to queue, sizes to re-read, files to check).
// GET /api/v1/admin/library-health/preview?action=fingerprint
func (h *LibraryHealthHandler) PreviewAction(c *gin.Context) {
	preview, err := h.service.PreviewAction(c.Query("action"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, preview)
}

type startHealthTaskRequest struct {
	Kind string `json:"kind" binding:"required"`
}

// StartTask starts a background task: missing_files or fix_sizes.
// POST /api/v1/admin/library-health/tasks
func (h *LibraryHealthHandler) StartTask(c *gin.Context) {
	var req startHealthTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	task, err := h.service.StartTask(req.Kind)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, task)
}

// CancelTask cancels the running background task.
// POST /api/v1/admin/library-health/tasks/cancel
func (h *LibraryHealthHandler) CancelTask(c *gin.Context) {
	if err := h.service.CancelTask(); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"message": "Task cancelled"})
}

// GetTaskStatus returns the running background task, if any.
// GET /api/v1/admin/library-health/tasks/status
func (h *LibraryHealthHandler) GetTaskStatus(c *gin.Context) {
	response.OK(c, h.service.GetTaskStatus())
}
