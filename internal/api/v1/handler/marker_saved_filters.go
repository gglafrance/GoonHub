package handler

import (
	"goonhub/internal/api/v1/response"
	"goonhub/internal/apperrors"
	"goonhub/internal/core"
	"goonhub/internal/data"

	"github.com/gin-gonic/gin"
)

// SetSavedFilterService provides the saved marker filters service (routes /marker-filters).
func (h *MarkerHandler) SetSavedFilterService(service *core.SavedMarkerFilterService) {
	h.savedFilters = service
}

type createSavedMarkerFilterRequest struct {
	Name    string                  `json:"name" binding:"required,max=100"`
	Filters data.MarkerFilterValues `json:"filters"`
}

type updateSavedMarkerFilterRequest struct {
	Name    *string                  `json:"name,omitempty"`
	Filters *data.MarkerFilterValues `json:"filters,omitempty"`
}

func (h *MarkerHandler) savedFilterService(c *gin.Context) (*core.SavedMarkerFilterService, bool) {
	if h.savedFilters == nil {
		response.Error(c, apperrors.NewInternalError("saved marker filters are not available", nil))
		return nil, false
	}
	return h.savedFilters, true
}

// ListSavedFilters lists the user's saved marker filters.
func (h *MarkerHandler) ListSavedFilters(c *gin.Context) {
	userID, ok := h.requireAuth(c)
	if !ok {
		return
	}
	svc, ok := h.savedFilterService(c)
	if !ok {
		return
	}
	filters, err := svc.List(userID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"filters": filters})
}

// CreateSavedFilter saves the given marker filters under a name.
func (h *MarkerHandler) CreateSavedFilter(c *gin.Context) {
	userID, ok := h.requireAuth(c)
	if !ok {
		return
	}
	svc, ok := h.savedFilterService(c)
	if !ok {
		return
	}
	var req createSavedMarkerFilterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "name is required (max 100 characters)")
		return
	}
	filter, err := svc.Create(userID, req.Name, req.Filters)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, filter)
}

// UpdateSavedFilter renames a saved marker filter and/or replaces its filters.
func (h *MarkerHandler) UpdateSavedFilter(c *gin.Context) {
	userID, ok := h.requireAuth(c)
	if !ok {
		return
	}
	svc, ok := h.savedFilterService(c)
	if !ok {
		return
	}
	var req updateSavedMarkerFilterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	filter, err := svc.Update(userID, c.Param("uuid"), req.Name, req.Filters)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, filter)
}

// DeleteSavedFilter deletes a saved marker filter.
func (h *MarkerHandler) DeleteSavedFilter(c *gin.Context) {
	userID, ok := h.requireAuth(c)
	if !ok {
		return
	}
	svc, ok := h.savedFilterService(c)
	if !ok {
		return
	}
	if err := svc.Delete(userID, c.Param("uuid")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}
