package handler

import (
	"goonhub/internal/api/v1/response"
	"goonhub/internal/apperrors"

	"github.com/gin-gonic/gin"
)

// maxMarkerDedupScenes caps the scene_ids of one deduplication request.
const maxMarkerDedupScenes = 1000

type deduplicateMarkersRequest struct {
	SceneIDs []uint `json:"scene_ids"`
	All      bool   `json:"all"`
	// DryRun defaults to true: nothing changes unless it is explicitly false
	DryRun *bool `json:"dry_run"`
}

// DeduplicateMarkers folds duplicate markers (same user, same or no label, starts at most a
// couple of seconds apart), e.g. left by earlier duplicate merges, in the given scenes or in
// all of them. Dry run by default: the response tells what would be merged and deleted.
func (h *MarkerHandler) DeduplicateMarkers(c *gin.Context) {
	var req deduplicateMarkersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	if !req.All && len(req.SceneIDs) == 0 {
		response.BadRequest(c, "scene_ids or all is required")
		return
	}
	if len(req.SceneIDs) > maxMarkerDedupScenes {
		response.BadRequest(c, "too many scenes (max 1000)")
		return
	}
	dryRun := req.DryRun == nil || *req.DryRun
	var sceneIDs []uint
	if !req.All {
		sceneIDs = req.SceneIDs
	}
	result, err := h.service.DeduplicateMarkers(sceneIDs, dryRun)
	if err != nil {
		response.Error(c, apperrors.NewInternalError("failed to deduplicate markers", err))
		return
	}
	response.OK(c, result)
}
