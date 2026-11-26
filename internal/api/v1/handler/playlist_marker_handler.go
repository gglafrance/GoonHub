package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"goonhub/internal/api/v1/request"
	"goonhub/internal/api/v1/response"
	"goonhub/internal/core"
	"goonhub/internal/data"
)

// Marker playlist routes (docs/MARKER_PLAYLISTS.md): /playlists/:uuid/markers...

// playlistMarkerTarget reads the user and the playlist UUID, answering 400 when missing.
func (h *PlaylistHandler) playlistMarkerTarget(c *gin.Context) (uint, string, bool) {
	userID, ok := h.getUserID(c)
	if !ok {
		response.BadRequest(c, "User not authenticated")
		return 0, "", false
	}
	uuidStr := c.Param("uuid")
	if _, err := uuid.Parse(uuidStr); err != nil {
		response.BadRequest(c, "Invalid playlist UUID")
		return 0, "", false
	}
	return userID, uuidStr, true
}

// AddMarkers adds markers by ID: {marker_ids} -> {added, skipped}.
func (h *PlaylistHandler) AddMarkers(c *gin.Context) {
	userID, uuidStr, ok := h.playlistMarkerTarget(c)
	if !ok {
		return
	}
	var req request.PlaylistMarkerIDsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "marker_ids is required")
		return
	}
	result, err := h.Service.AddMarkers(userID, uuidStr, req.MarkerIDs)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}

// AddMarkersFromFilter adds every marker of a marker listing (a label, or the filtered list of
// all markers) -> {added, skipped}.
func (h *PlaylistHandler) AddMarkersFromFilter(c *gin.Context) {
	userID, uuidStr, ok := h.playlistMarkerTarget(c)
	if !ok {
		return
	}
	var req request.AddPlaylistMarkersFromFilterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	filter := core.PlaylistMarkerFilter{
		Label: strings.TrimSpace(req.Label),
		Options: data.MarkerListOptions{
			SortBy:     req.Sort,
			Seed:       req.Seed,
			RangesOnly: req.RangesOnly,
			Labels:     splitQueryList(req.Labels),
			Tags:       splitQueryList(req.Tags),
			Actors:     splitQueryList(req.Actors),
			Studio:     strings.TrimSpace(req.Studio),
			MarkerTags: splitQueryList(req.MarkerTags),
		},
	}
	result, err := h.Service.AddMarkersFromFilter(userID, uuidStr, filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}

// RemoveMarker removes one marker (:markerId).
func (h *PlaylistHandler) RemoveMarker(c *gin.Context) {
	userID, uuidStr, ok := h.playlistMarkerTarget(c)
	if !ok {
		return
	}
	markerID, err := strconv.ParseUint(c.Param("markerId"), 10, 64)
	if err != nil || markerID == 0 {
		response.BadRequest(c, "Invalid marker ID")
		return
	}
	if err := h.Service.RemoveMarker(userID, uuidStr, uint(markerID)); err != nil {
		response.Error(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// RemoveMarkers removes several markers: {marker_ids}.
func (h *PlaylistHandler) RemoveMarkers(c *gin.Context) {
	userID, uuidStr, ok := h.playlistMarkerTarget(c)
	if !ok {
		return
	}
	var req request.PlaylistMarkerIDsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "marker_ids is required")
		return
	}
	if err := h.Service.RemoveMarkers(userID, uuidStr, req.MarkerIDs); err != nil {
		response.Error(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ReorderMarkers sets the order of the markers: {marker_ids} in the new order.
func (h *PlaylistHandler) ReorderMarkers(c *gin.Context) {
	userID, uuidStr, ok := h.playlistMarkerTarget(c)
	if !ok {
		return
	}
	var req request.PlaylistMarkerIDsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "marker_ids is required")
		return
	}
	if err := h.Service.ReorderMarkers(userID, uuidStr, req.MarkerIDs); err != nil {
		response.Error(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
