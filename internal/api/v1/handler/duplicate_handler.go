package handler

import (
	"encoding/json"
	"goonhub/internal/api/v1/request"
	"goonhub/internal/api/v1/response"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"strconv"

	"github.com/gin-gonic/gin"
)

// DuplicateHandler handles duplicate detection API requests.
type DuplicateHandler struct {
	service   *core.DuplicateDetectionService
	sceneRepo data.SceneRepository
}

// NewDuplicateHandler creates a new DuplicateHandler.
func NewDuplicateHandler(service *core.DuplicateDetectionService, sceneRepo data.SceneRepository) *DuplicateHandler {
	return &DuplicateHandler{
		service:   service,
		sceneRepo: sceneRepo,
	}
}

// GetConfig returns the current duplicate detection configuration.
func (h *DuplicateHandler) GetConfig(c *gin.Context) {
	cfg, err := h.service.GetConfig()
	if err != nil {
		response.Error(c, err)
		return
	}

	resp := toDuplicateConfigResponse(cfg)
	response.OK(c, resp)
}

// UpdateConfig updates the duplicate detection configuration.
func (h *DuplicateHandler) UpdateConfig(c *gin.Context) {
	var req request.UpdateDuplicateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	cfg := &data.DuplicateConfigRecord{
		ID:              1,
		Enabled:         req.Enabled,
		CheckOnUpload:   req.CheckOnUpload,
		MatchThreshold:  req.MatchThreshold,
		HammingDistance:  req.HammingDistance,
		SampleInterval:  req.SampleInterval,
		DuplicateAction: req.DuplicateAction,
		KeepBestRules:   req.KeepBestRules,
		KeepBestEnabled: req.KeepBestEnabled,
		CodecPreference: req.CodecPreference,
	}

	if err := h.service.UpdateConfig(cfg); err != nil {
		response.Error(c, err)
		return
	}

	updated, err := h.service.GetConfig()
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, toDuplicateConfigResponse(updated))
}

// ListGroups returns paginated duplicate groups.
func (h *DuplicateHandler) ListGroups(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	status := c.DefaultQuery("status", "")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	groups, total, err := h.service.ListGroups(page, limit, status)
	if err != nil {
		response.Error(c, err)
		return
	}

	groupResponses := make([]response.DuplicateGroupResponse, len(groups))
	for i, g := range groups {
		groupResponses[i] = toDuplicateGroupResponse(g)
	}

	response.OK(c, response.NewPaginatedResponse(groupResponses, page, limit, total))
}

// GetGroup returns a duplicate group with its members and scene details.
func (h *DuplicateHandler) GetGroup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "invalid group ID")
		return
	}

	group, err := h.service.GetGroup(uint(id))
	if err != nil {
		response.Error(c, err)
		return
	}
	if group == nil {
		response.NotFound(c, "duplicate group not found")
		return
	}

	groupResp := toDuplicateGroupResponse(*group)

	// Enrich members with scene data
	sceneIDs := make([]uint, len(group.Members))
	for i, m := range group.Members {
		sceneIDs[i] = m.SceneID
	}
	scenes, err := h.sceneRepo.GetByIDs(sceneIDs)
	if err == nil {
		sceneMap := make(map[uint]*data.Scene, len(scenes))
		for i := range scenes {
			sceneMap[scenes[i].ID] = &scenes[i]
		}
		for i := range groupResp.Members {
			if scene, ok := sceneMap[groupResp.Members[i].SceneID]; ok {
				groupResp.Members[i].Scene = &response.DuplicateSceneSummary{
					ID:            scene.ID,
					Title:         scene.Title,
					Duration:      float64(scene.Duration),
					Width:         scene.Width,
					Height:        scene.Height,
					VideoCodec:    scene.VideoCodec,
					BitRate:       scene.BitRate,
					FileSize:      scene.Size,
					ThumbnailPath: scene.ThumbnailPath,
				}
			}
		}
	}

	response.OK(c, groupResp)
}

// ResolveGroup auto-resolves a duplicate group using keep-best rules.
func (h *DuplicateHandler) ResolveGroup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "invalid group ID")
		return
	}

	if err := h.service.ResolveDuplicateGroup(uint(id)); err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{"message": "group resolved"})
}

// DismissGroup marks a duplicate group as dismissed.
func (h *DuplicateHandler) DismissGroup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "invalid group ID")
		return
	}

	if err := h.service.DismissGroup(uint(id)); err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{"message": "group dismissed"})
}

// SetWinner manually sets the winner for a duplicate group.
func (h *DuplicateHandler) SetWinner(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "invalid group ID")
		return
	}

	var req request.SetWinnerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	if err := h.service.SetWinner(uint(id), req.SceneID); err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{"message": "winner set"})
}

// DeleteGroup permanently removes a duplicate group.
func (h *DuplicateHandler) DeleteGroup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "invalid group ID")
		return
	}

	if err := h.service.DeleteGroup(uint(id)); err != nil {
		response.Error(c, err)
		return
	}

	response.NoContent(c)
}

// StartRescan triggers a full library duplicate rescan.
func (h *DuplicateHandler) StartRescan(c *gin.Context) {
	if err := h.service.StartRescan(); err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{"message": "rescan started"})
}

// GetRescanStatus returns the current rescan progress.
func (h *DuplicateHandler) GetRescanStatus(c *gin.Context) {
	status := h.service.GetRescanStatus()
	response.OK(c, response.RescanStatusResponse{
		Running:   status.Running,
		Total:     status.Total,
		Completed: status.Completed,
		Matched:   status.Matched,
	})
}

func toDuplicateConfigResponse(cfg *data.DuplicateConfigRecord) response.DuplicateConfigResponse {
	var rules any
	var enabled any
	var codecs any

	if err := json.Unmarshal(cfg.KeepBestRules, &rules); err != nil {
		rules = []string{"duration", "resolution", "codec", "bitrate"}
	}
	if err := json.Unmarshal(cfg.KeepBestEnabled, &enabled); err != nil {
		enabled = map[string]bool{"duration": true, "resolution": true, "codec": true, "bitrate": true}
	}
	if err := json.Unmarshal(cfg.CodecPreference, &codecs); err != nil {
		codecs = []string{"h265", "hevc", "av1", "vp9", "h264"}
	}

	return response.DuplicateConfigResponse{
		Enabled:         cfg.Enabled,
		CheckOnUpload:   cfg.CheckOnUpload,
		MatchThreshold:  cfg.MatchThreshold,
		HammingDistance:  cfg.HammingDistance,
		SampleInterval:  cfg.SampleInterval,
		DuplicateAction: cfg.DuplicateAction,
		KeepBestRules:   rules,
		KeepBestEnabled: enabled,
		CodecPreference: codecs,
	}
}

func toDuplicateGroupResponse(g data.DuplicateGroup) response.DuplicateGroupResponse {
	members := make([]response.DuplicateGroupMemberResponse, len(g.Members))
	for i, m := range g.Members {
		members[i] = response.DuplicateGroupMemberResponse{
			ID:              m.ID,
			SceneID:         m.SceneID,
			MatchPercentage: m.MatchPercentage,
			FrameOffset:     m.FrameOffset,
			IsWinner:        m.IsWinner,
		}
	}

	return response.DuplicateGroupResponse{
		ID:            g.ID,
		Status:        g.Status,
		WinnerSceneID: g.WinnerSceneID,
		Members:       members,
		CreatedAt:     g.CreatedAt,
		UpdatedAt:     g.UpdatedAt,
	}
}
