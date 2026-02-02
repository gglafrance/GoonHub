package handler

import (
	"strconv"

	"goonhub/internal/api/middleware"
	"goonhub/internal/api/v1/request"
	"goonhub/internal/api/v1/response"
	"goonhub/internal/apperrors"
	"goonhub/internal/core"

	"github.com/gin-gonic/gin"
)

type MarkerHandler struct {
	service *core.MarkerService
}

func NewMarkerHandler(service *core.MarkerService) *MarkerHandler {
	return &MarkerHandler{service: service}
}

// requireAuth extracts the authenticated user from context.
// Returns the user ID and true if successful, or sends an error response and returns false.
func (h *MarkerHandler) requireAuth(c *gin.Context) (uint, bool) {
	payload, err := middleware.GetUserFromContext(c)
	if err != nil {
		response.Error(c, apperrors.NewUnauthorizedError("authentication required"))
		return 0, false
	}
	return payload.UserID, true
}

func (h *MarkerHandler) ListMarkers(c *gin.Context) {
	userID, ok := h.requireAuth(c)
	if !ok {
		return
	}

	sceneID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid scene ID")
		return
	}

	markers, err := h.service.ListMarkers(userID, uint(sceneID))
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{"markers": markers})
}

func (h *MarkerHandler) CreateMarker(c *gin.Context) {
	userID, ok := h.requireAuth(c)
	if !ok {
		return
	}

	sceneID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid scene ID")
		return
	}

	var req request.CreateMarkerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	marker, err := h.service.CreateMarker(userID, uint(sceneID), req.Timestamp, req.Label, req.Color)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Created(c, marker)
}

func (h *MarkerHandler) UpdateMarker(c *gin.Context) {
	userID, ok := h.requireAuth(c)
	if !ok {
		return
	}

	markerID, err := strconv.ParseUint(c.Param("markerID"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid marker ID")
		return
	}

	var req request.UpdateMarkerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	marker, err := h.service.UpdateMarker(userID, uint(markerID), req.Label, req.Color, req.Timestamp)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, marker)
}

func (h *MarkerHandler) DeleteMarker(c *gin.Context) {
	userID, ok := h.requireAuth(c)
	if !ok {
		return
	}

	markerID, err := strconv.ParseUint(c.Param("markerID"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid marker ID")
		return
	}

	if err := h.service.DeleteMarker(userID, uint(markerID)); err != nil {
		response.Error(c, err)
		return
	}

	response.NoContent(c)
}

func (h *MarkerHandler) ListLabelSuggestions(c *gin.Context) {
	userID, ok := h.requireAuth(c)
	if !ok {
		return
	}

	suggestions, err := h.service.GetLabelSuggestions(userID, 50)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{"labels": suggestions})
}

func (h *MarkerHandler) ListLabelGroups(c *gin.Context) {
	userID, ok := h.requireAuth(c)
	if !ok {
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	sortBy := c.DefaultQuery("sort", "count_desc")

	// Validate pagination bounds
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 1
	} else if limit > 100 {
		limit = 100
	}

	groups, total, err := h.service.GetLabelGroups(userID, page, limit, sortBy)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, response.NewPaginatedResponse(groups, page, limit, total))
}

func (h *MarkerHandler) ListMarkersByLabel(c *gin.Context) {
	userID, ok := h.requireAuth(c)
	if !ok {
		return
	}

	label := c.Query("label")
	if label == "" {
		response.BadRequest(c, "label query parameter is required")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	// Validate pagination bounds
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 1
	} else if limit > 100 {
		limit = 100
	}

	markers, total, err := h.service.GetMarkersByLabel(userID, label, page, limit)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, response.NewPaginatedResponse(markers, page, limit, total))
}

// ListAllMarkers returns all individual markers for the authenticated user
func (h *MarkerHandler) ListAllMarkers(c *gin.Context) {
	userID, ok := h.requireAuth(c)
	if !ok {
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	sortBy := c.DefaultQuery("sort", "label_asc")

	// Validate pagination bounds
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 1
	} else if limit > 100 {
		limit = 100
	}

	markers, total, err := h.service.GetAllMarkers(userID, page, limit, sortBy)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, response.NewPaginatedResponse(markers, page, limit, total))
}

// GetLabelTags returns the default tags for a label
func (h *MarkerHandler) GetLabelTags(c *gin.Context) {
	userID, ok := h.requireAuth(c)
	if !ok {
		return
	}

	label := c.Query("label")
	if label == "" {
		response.BadRequest(c, "label query parameter is required")
		return
	}

	tags, err := h.service.GetLabelTags(userID, label)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{"tags": tags})
}

// SetLabelTags sets the default tags for a label
func (h *MarkerHandler) SetLabelTags(c *gin.Context) {
	userID, ok := h.requireAuth(c)
	if !ok {
		return
	}

	label := c.Query("label")
	if label == "" {
		response.BadRequest(c, "label query parameter is required")
		return
	}

	var req request.SetLabelTagsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	if err := h.service.SetLabelTags(userID, label, req.TagIDs); err != nil {
		response.Error(c, err)
		return
	}

	// Return the updated tags
	tags, err := h.service.GetLabelTags(userID, label)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{"tags": tags})
}

// GetMarkerTags returns tags for a specific marker
func (h *MarkerHandler) GetMarkerTags(c *gin.Context) {
	userID, ok := h.requireAuth(c)
	if !ok {
		return
	}

	markerID, err := strconv.ParseUint(c.Param("markerID"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid marker ID")
		return
	}

	tags, err := h.service.GetMarkerTags(userID, uint(markerID))
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{"tags": tags})
}

// SetMarkerTags sets individual tags on a marker
func (h *MarkerHandler) SetMarkerTags(c *gin.Context) {
	userID, ok := h.requireAuth(c)
	if !ok {
		return
	}

	markerID, err := strconv.ParseUint(c.Param("markerID"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid marker ID")
		return
	}

	var req request.SetMarkerTagsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	if err := h.service.SetMarkerTags(userID, uint(markerID), req.TagIDs); err != nil {
		response.Error(c, err)
		return
	}

	// Return the updated tags
	tags, err := h.service.GetMarkerTags(userID, uint(markerID))
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{"tags": tags})
}

// AddMarkerTags adds tags to a marker
func (h *MarkerHandler) AddMarkerTags(c *gin.Context) {
	userID, ok := h.requireAuth(c)
	if !ok {
		return
	}

	markerID, err := strconv.ParseUint(c.Param("markerID"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid marker ID")
		return
	}

	var req request.SetMarkerTagsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	if err := h.service.AddMarkerTags(userID, uint(markerID), req.TagIDs); err != nil {
		response.Error(c, err)
		return
	}

	// Return the updated tags
	tags, err := h.service.GetMarkerTags(userID, uint(markerID))
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{"tags": tags})
}
