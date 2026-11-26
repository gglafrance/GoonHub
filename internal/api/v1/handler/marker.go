package handler

import (
	"strconv"
	"strings"

	"goonhub/internal/api/middleware"
	"goonhub/internal/api/v1/request"
	"goonhub/internal/api/v1/response"
	"goonhub/internal/apperrors"
	"goonhub/internal/core"
	"goonhub/internal/data"

	"github.com/gin-gonic/gin"
)

type MarkerHandler struct {
	service         *core.MarkerService
	maxItemsPerPage int
	savedFilters    *core.SavedMarkerFilterService
	labels          *core.MarkerLabelService
}

func NewMarkerHandler(service *core.MarkerService, maxItemsPerPage int) *MarkerHandler {
	return &MarkerHandler{service: service, maxItemsPerPage: maxItemsPerPage}
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

	marker, err := h.service.CreateMarker(userID, uint(sceneID), req.Timestamp, req.EndTimestamp, req.Label, req.Color,
		middleware.HasPermission(c, core.PermMarkerLabelsManage))
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

	marker, err := h.service.UpdateMarker(userID, uint(markerID), req.Label, req.Color, req.Timestamp, req.EndTimestamp, req.ClearEndTimestamp,
		middleware.HasPermission(c, core.PermMarkerLabelsManage))
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

	suggestions, err := h.service.GetLabelSuggestions(userID, maxMarkerLabelSuggestions)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{"labels": suggestions})
}

// markerListOptions reads the sort, seed (random sort) and filter query parameters
// (ranges_only, and comma-separated labels, tags (scene), marker_tags, actors, plus studio).
func markerListOptions(c *gin.Context, defaultSort string) data.MarkerListOptions {
	seed, _ := strconv.ParseInt(c.Query("seed"), 10, 64)
	return data.MarkerListOptions{
		SortBy:     c.DefaultQuery("sort", defaultSort),
		Seed:       seed,
		RangesOnly: c.Query("ranges_only") == "true",
		Labels:     splitQueryList(c.Query("labels")),
		Tags:       splitQueryList(c.Query("tags")),
		Actors:     splitQueryList(c.Query("actors")),
		Studio:     strings.TrimSpace(c.Query("studio")),
		MarkerTags: splitQueryList(c.Query("marker_tags")),
	}
}

// splitQueryList splits a comma-separated query value, dropping empty entries.
func splitQueryList(raw string) []string {
	if raw == "" {
		return nil
	}
	var out []string
	for _, v := range strings.Split(raw, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func (h *MarkerHandler) ListLabelGroups(c *gin.Context) {
	userID, ok := h.requireAuth(c)
	if !ok {
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	page, limit = clampPagination(page, limit, 20, h.maxItemsPerPage)
	opts := markerListOptions(c, "count_desc")

	groups, total, err := h.service.GetLabelGroups(userID, page, limit, opts)
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
	page, limit = clampPagination(page, limit, 20, h.maxItemsPerPage)

	markers, total, err := h.service.GetMarkersByLabel(userID, label, page, limit, markerListOptions(c, ""))
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
	page, limit = clampPagination(page, limit, 20, h.maxItemsPerPage)
	opts := markerListOptions(c, "label_asc")

	markers, total, err := h.service.GetAllMarkers(userID, page, limit, opts)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, response.NewPaginatedResponse(markers, page, limit, total))
}

// GetLabelTags returns the default tags of a label, by name (GET /markers/label-tags?label=,
// ignoring case). Unknown label: 404.
func (h *MarkerHandler) GetLabelTags(c *gin.Context) {
	label := c.Query("label")
	if strings.TrimSpace(label) == "" {
		response.BadRequest(c, "label query parameter is required")
		return
	}
	found, err := h.labels.FindByName(label)
	if err != nil {
		response.Error(c, err)
		return
	}
	tags, err := h.labels.GetDefaultTags(found.ID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"tags": tags})
}

// SetLabelTags sets the default tags of a label, by name (PUT /markers/label-tags?label=,
// marker_labels:manage). The tags are the same for every user.
func (h *MarkerHandler) SetLabelTags(c *gin.Context) {
	label := c.Query("label")
	if strings.TrimSpace(label) == "" {
		response.BadRequest(c, "label query parameter is required")
		return
	}
	var req request.SetLabelTagsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	found, err := h.labels.FindByName(label)
	if err != nil {
		response.Error(c, err)
		return
	}
	tags, err := h.labels.SetDefaultTags(found.ID, req.TagIDs)
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
