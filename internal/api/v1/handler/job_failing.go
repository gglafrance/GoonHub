package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"goonhub/internal/api/v1/response"
	"goonhub/internal/api/v1/validators"
	"goonhub/internal/apperrors"
	"goonhub/internal/core"

	"github.com/gin-gonic/gin"
)

// maxFailingBulkIDs bounds the IDs of a bulk action on failing scenes ("all" has no bound).
const maxFailingBulkIDs = 1000

// failingFilter reads the phase and error text filters of a request.
func failingFilter(phase, query string) (core.FailingFilter, error) {
	if phase != "" {
		if err := validators.ValidateProcessingPhase(phase); err != nil {
			return core.FailingFilter{}, apperrors.NewValidationError(err.Error())
		}
	}
	return core.FailingFilter{Phase: phase, ErrorQuery: query}, nil
}

// ListFailing returns a page of the failing (scene, phase): failures in a row on an unchanged
// file, skipped by the "missing" runs and the automatic retries.
// GET /api/v1/admin/jobs/failing?phase=&q=&page=&limit=
func (h *JobHandler) ListFailing(c *gin.Context) {
	filter, err := failingFilter(c.Query("phase"), c.Query("q"))
	if err != nil {
		response.Error(c, err)
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "25"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	list, err := h.failures.List(filter, page, limit)
	if err != nil {
		response.Error(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

// failingID reads the :id of a failing (scene, phase).
func failingID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		response.Error(c, apperrors.NewValidationError("invalid failing scene ID"))
		return 0, false
	}
	return uint(id), true
}

// RetryFailing forgets the failures of a (scene, phase) and queues its job.
// POST /api/v1/admin/jobs/failing/:id/retry
func (h *JobHandler) RetryFailing(c *gin.Context) {
	id, ok := failingID(c)
	if !ok {
		return
	}
	result, err := h.failures.Retry([]uint{id})
	if err != nil {
		response.Error(c, err)
		return
	}
	if result.Cleared == 0 {
		response.Error(c, apperrors.NewNotFoundError("failing scene", id))
		return
	}
	if result.Retried == 0 {
		response.Error(c, apperrors.NewValidationError("the failures were cleared but the job could not be queued"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Job queued", "result": result})
}

// ClearFailing forgets the failures of a (scene, phase) without queuing anything.
// POST /api/v1/admin/jobs/failing/:id/clear
func (h *JobHandler) ClearFailing(c *gin.Context) {
	id, ok := failingID(c)
	if !ok {
		return
	}
	result, err := h.failures.Clear([]uint{id})
	if err != nil {
		response.Error(c, err)
		return
	}
	if result.Cleared == 0 {
		response.Error(c, apperrors.NewNotFoundError("failing scene", id))
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Failures cleared", "result": result})
}

// failingBulkRequest is the body of POST /admin/jobs/failing/bulk: the action on the given IDs,
// or with all, on every failing (scene, phase) matching the phase and error filters.
type failingBulkRequest struct {
	Action string `json:"action"` // "retry" or "clear"
	IDs    []uint `json:"ids"`
	All    bool   `json:"all"`
	Phase  string `json:"phase"`
	Query  string `json:"q"`
}

// BulkFailing retries or clears many failing (scene, phase).
// POST /api/v1/admin/jobs/failing/bulk
func (h *JobHandler) BulkFailing(c *gin.Context) {
	var req failingBulkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.NewValidationError("invalid request body"))
		return
	}
	if req.Action != "retry" && req.Action != "clear" {
		response.Error(c, apperrors.NewValidationError(`action must be "retry" or "clear"`))
		return
	}
	ids := req.IDs
	if req.All {
		if len(ids) > 0 {
			response.Error(c, apperrors.NewValidationError("give either ids or all, not both"))
			return
		}
		filter, err := failingFilter(req.Phase, req.Query)
		if err != nil {
			response.Error(c, err)
			return
		}
		if ids, err = h.failures.FailingIDs(filter); err != nil {
			response.Error(c, err)
			return
		}
	} else {
		if len(ids) == 0 {
			response.Error(c, apperrors.NewValidationError("ids must not be empty"))
			return
		}
		if len(ids) > maxFailingBulkIDs {
			response.Error(c, apperrors.NewValidationError(fmt.Sprintf("ids must not exceed %d items", maxFailingBulkIDs)))
			return
		}
	}

	var (
		result *core.FailingActionResult
		err    error
	)
	if len(ids) == 0 {
		result = &core.FailingActionResult{}
	} else if req.Action == "retry" {
		result, err = h.failures.Retry(ids)
	} else {
		result, err = h.failures.Clear(ids)
	}
	if err != nil {
		response.Error(c, err)
		return
	}
	message := fmt.Sprintf("Cleared %d failing scenes", result.Cleared)
	if req.Action == "retry" {
		message = fmt.Sprintf("Queued %d jobs (%d errors)", result.Retried, result.Errors)
	}
	c.JSON(http.StatusOK, gin.H{"message": message, "action": req.Action, "result": result})
}
