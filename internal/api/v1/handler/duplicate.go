package handler

import (
	"goonhub/internal/api/v1/response"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// DuplicateHandler handles duplicate group management endpoints
type DuplicateHandler struct {
	duplicateService *core.DuplicateService
	dupConfigRepo    data.DuplicationConfigRepository
}

// NewDuplicateHandler creates a new DuplicateHandler
func NewDuplicateHandler(
	duplicateService *core.DuplicateService,
	dupConfigRepo data.DuplicationConfigRepository,
) *DuplicateHandler {
	return &DuplicateHandler{
		duplicateService: duplicateService,
		dupConfigRepo:    dupConfigRepo,
	}
}

// ListGroups returns paginated duplicate groups
func (h *DuplicateHandler) ListGroups(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	page, limit = clampPagination(page, limit, 20, 100)

	status := c.Query("status")
	sortBy := c.DefaultQuery("sort_by", "newest")

	groups, total, err := h.duplicateService.ListGroups(page, limit, status, sortBy)
	if err != nil {
		response.InternalError(c, "Failed to list duplicate groups")
		return
	}

	respGroups := make([]response.DuplicateGroupResponse, len(groups))
	for i, g := range groups {
		respGroups[i] = response.ToDuplicateGroupResponse(g)
	}

	response.OK(c, response.NewPaginatedResponse(respGroups, page, limit, total))
}

// GetGroup returns a single duplicate group with full details
func (h *DuplicateHandler) GetGroup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid group ID")
		return
	}

	group, err := h.duplicateService.GetGroup(uint(id))
	if err != nil {
		response.InternalError(c, "Failed to get duplicate group")
		return
	}

	response.OK(c, response.ToDuplicateGroupResponse(*group))
}

// GetStats returns duplicate group statistics
func (h *DuplicateHandler) GetStats(c *gin.Context) {
	stats, err := h.duplicateService.GetStats()
	if err != nil {
		response.InternalError(c, "Failed to get duplicate stats")
		return
	}

	response.OK(c, response.DuplicateStatsResponse{
		Unresolved: stats.Unresolved,
		Resolved:   stats.Resolved,
		Dismissed:  stats.Dismissed,
		Total:      stats.Total,
	})
}

// GetConfig returns the duplication detection configuration
func (h *DuplicateHandler) GetConfig(c *gin.Context) {
	cfg, err := h.dupConfigRepo.Get()
	if err != nil {
		response.InternalError(c, "Failed to get duplication config")
		return
	}

	if cfg == nil {
		// Return defaults
		response.OK(c, response.DuplicationConfigResponse{
			AudioDensityThreshold:   0.50,
			AudioMinHashes:          80,
			AudioMaxHashOccurrences: 10,
			AudioMinSpan:            160,
			VisualHammingMax:        5,
			VisualMinFrames:         20,
			VisualMinSpan:           30,
			DeltaTolerance:          2,
			FingerprintMode:         "audio_only",
		})
		return
	}

	fpMode := cfg.FingerprintMode
	if fpMode == "" {
		fpMode = "audio_only"
	}

	response.OK(c, response.DuplicationConfigResponse{
		AudioDensityThreshold:   cfg.AudioDensityThreshold,
		AudioMinHashes:          cfg.AudioMinHashes,
		AudioMaxHashOccurrences: cfg.AudioMaxHashOccurrences,
		AudioMinSpan:            cfg.AudioMinSpan,
		VisualHammingMax:        cfg.VisualHammingMax,
		VisualMinFrames:         cfg.VisualMinFrames,
		VisualMinSpan:           cfg.VisualMinSpan,
		DeltaTolerance:          cfg.DeltaTolerance,
		FingerprintMode:         fpMode,
	})
}

// UpdateConfig updates the duplication detection configuration
func (h *DuplicateHandler) UpdateConfig(c *gin.Context) {
	var req response.DuplicationConfigResponse
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	// Validate fingerprint_mode
	fpMode := req.FingerprintMode
	if fpMode == "" {
		fpMode = "audio_only"
	}
	if fpMode != "audio_only" && fpMode != "dual" {
		response.BadRequest(c, "fingerprint_mode must be 'audio_only' or 'dual'")
		return
	}

	record := &data.DuplicationConfigRecord{
		AudioDensityThreshold:   req.AudioDensityThreshold,
		AudioMinHashes:          req.AudioMinHashes,
		AudioMaxHashOccurrences: req.AudioMaxHashOccurrences,
		AudioMinSpan:            req.AudioMinSpan,
		VisualHammingMax:        req.VisualHammingMax,
		VisualMinFrames:         req.VisualMinFrames,
		VisualMinSpan:           req.VisualMinSpan,
		DeltaTolerance:          req.DeltaTolerance,
		FingerprintMode:         fpMode,
	}

	if err := h.dupConfigRepo.Upsert(record); err != nil {
		response.InternalError(c, "Failed to update duplication config")
		return
	}

	req.FingerprintMode = fpMode
	response.OK(c, req)
}

type resolveGroupRequest struct {
	BestSceneID   uint `json:"best_scene_id" binding:"required"`
	MergeMetadata bool `json:"merge_metadata"`
}

// ResolveGroup resolves a duplicate group
func (h *DuplicateHandler) ResolveGroup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid group ID")
		return
	}

	var req resolveGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	if err := h.duplicateService.ResolveGroup(uint(id), req.BestSceneID, req.MergeMetadata); err != nil {
		response.InternalError(c, "Failed to resolve group: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Group resolved"})
}

// DismissGroup dismisses a duplicate group
func (h *DuplicateHandler) DismissGroup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid group ID")
		return
	}

	if err := h.duplicateService.DismissGroup(uint(id)); err != nil {
		response.InternalError(c, "Failed to dismiss group")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Group dismissed"})
}

type setBestRequest struct {
	SceneID uint `json:"scene_id" binding:"required"`
}

// SetBest updates the best variant for a duplicate group
func (h *DuplicateHandler) SetBest(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid group ID")
		return
	}

	var req setBestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	if err := h.duplicateService.SetBest(uint(id), req.SceneID); err != nil {
		response.InternalError(c, "Failed to set best scene: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Best scene updated"})
}
