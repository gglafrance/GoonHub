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

type PlaylistHandler struct {
	Service         *core.PlaylistService
	MaxItemsPerPage int
}

func NewPlaylistHandler(service *core.PlaylistService, maxItemsPerPage int) *PlaylistHandler {
	return &PlaylistHandler{Service: service, MaxItemsPerPage: maxItemsPerPage}
}

func (h *PlaylistHandler) getUserID(c *gin.Context) (uint, bool) {
	user, exists := c.Get("user")
	if !exists {
		return 0, false
	}
	userPayload, ok := user.(*core.UserPayload)
	if !ok {
		return 0, false
	}
	return userPayload.UserID, true
}

func (h *PlaylistHandler) List(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		response.BadRequest(c, "User not authenticated")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	page, limit = clampPagination(page, limit, 20, h.MaxItemsPerPage)

	// Parse tag_ids
	var tagIDs []uint
	if tagIDsStr := c.Query("tag_ids"); tagIDsStr != "" {
		for _, idStr := range strings.Split(tagIDsStr, ",") {
			id, err := strconv.ParseUint(strings.TrimSpace(idStr), 10, 64)
			if err == nil {
				tagIDs = append(tagIDs, uint(id))
			}
		}
	}

	params := data.PlaylistListParams{
		Owner:      c.DefaultQuery("owner", "all"),
		Visibility: c.Query("visibility"),
		TagIDs:     tagIDs,
		Search:     c.Query("search"),
		Sort:       c.DefaultQuery("sort", "created_at_desc"),
		Page:       page,
		Limit:      limit,
	}

	items, total, err := h.Service.List(userID, params)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, response.NewPaginatedResponse(
		response.NewPlaylistListResponse(items),
		page, limit, total,
	))
}

func (h *PlaylistHandler) GetByUUID(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		response.BadRequest(c, "User not authenticated")
		return
	}

	uuidStr := c.Param("uuid")
	if _, err := uuid.Parse(uuidStr); err != nil {
		response.BadRequest(c, "Invalid playlist UUID")
		return
	}

	detail, err := h.Service.GetByUUID(userID, uuidStr)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, response.NewPlaylistDetailResponse(detail))
}

func (h *PlaylistHandler) Create(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		response.BadRequest(c, "User not authenticated")
		return
	}

	var req request.CreatePlaylistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Name is required")
		return
	}

	input := core.CreatePlaylistInput{
		Name:        req.Name,
		Description: req.Description,
		Visibility:  req.Visibility,
		TagIDs:      req.TagIDs,
		SceneIDs:    req.SceneIDs,
	}

	playlist, err := h.Service.Create(userID, input)
	if err != nil {
		response.Error(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"uuid": playlist.UUID.String(),
		"name": playlist.Name,
	})
}

func (h *PlaylistHandler) Update(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		response.BadRequest(c, "User not authenticated")
		return
	}

	uuidStr := c.Param("uuid")
	if _, err := uuid.Parse(uuidStr); err != nil {
		response.BadRequest(c, "Invalid playlist UUID")
		return
	}

	var req request.UpdatePlaylistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	input := core.UpdatePlaylistInput{
		Name:        req.Name,
		Description: req.Description,
		Visibility:  req.Visibility,
	}

	playlist, err := h.Service.Update(userID, uuidStr, input)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{
		"uuid":       playlist.UUID.String(),
		"name":       playlist.Name,
		"visibility": playlist.Visibility,
	})
}

func (h *PlaylistHandler) Delete(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		response.BadRequest(c, "User not authenticated")
		return
	}

	uuidStr := c.Param("uuid")
	if _, err := uuid.Parse(uuidStr); err != nil {
		response.BadRequest(c, "Invalid playlist UUID")
		return
	}

	if err := h.Service.Delete(userID, uuidStr); err != nil {
		response.Error(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *PlaylistHandler) AddScenes(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		response.BadRequest(c, "User not authenticated")
		return
	}

	uuidStr := c.Param("uuid")
	if _, err := uuid.Parse(uuidStr); err != nil {
		response.BadRequest(c, "Invalid playlist UUID")
		return
	}

	var req request.AddPlaylistScenesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "scene_ids is required")
		return
	}

	if err := h.Service.AddScenes(userID, uuidStr, req.SceneIDs); err != nil {
		response.Error(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *PlaylistHandler) RemoveScene(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		response.BadRequest(c, "User not authenticated")
		return
	}

	uuidStr := c.Param("uuid")
	if _, err := uuid.Parse(uuidStr); err != nil {
		response.BadRequest(c, "Invalid playlist UUID")
		return
	}

	sceneIDStr := c.Param("sceneId")
	sceneID, err := strconv.ParseUint(sceneIDStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid scene ID")
		return
	}

	if err := h.Service.RemoveScene(userID, uuidStr, uint(sceneID)); err != nil {
		response.Error(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *PlaylistHandler) RemoveScenes(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		response.BadRequest(c, "User not authenticated")
		return
	}

	uuidStr := c.Param("uuid")
	if _, err := uuid.Parse(uuidStr); err != nil {
		response.BadRequest(c, "Invalid playlist UUID")
		return
	}

	var req request.RemovePlaylistScenesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "scene_ids is required")
		return
	}

	if err := h.Service.RemoveScenes(userID, uuidStr, req.SceneIDs); err != nil {
		response.Error(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *PlaylistHandler) ReorderScenes(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		response.BadRequest(c, "User not authenticated")
		return
	}

	uuidStr := c.Param("uuid")
	if _, err := uuid.Parse(uuidStr); err != nil {
		response.BadRequest(c, "Invalid playlist UUID")
		return
	}

	var req request.ReorderPlaylistScenesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "scene_ids is required")
		return
	}

	if err := h.Service.ReorderScenes(userID, uuidStr, req.SceneIDs); err != nil {
		response.Error(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *PlaylistHandler) GetTags(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		response.BadRequest(c, "User not authenticated")
		return
	}

	uuidStr := c.Param("uuid")
	if _, err := uuid.Parse(uuidStr); err != nil {
		response.BadRequest(c, "Invalid playlist UUID")
		return
	}

	tags, err := h.Service.GetTags(userID, uuidStr)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{"data": tags})
}

func (h *PlaylistHandler) SetTags(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		response.BadRequest(c, "User not authenticated")
		return
	}

	uuidStr := c.Param("uuid")
	if _, err := uuid.Parse(uuidStr); err != nil {
		response.BadRequest(c, "Invalid playlist UUID")
		return
	}

	var req request.SetPlaylistTagsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	tags, err := h.Service.SetTags(userID, uuidStr, req.TagIDs)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{"data": tags})
}

func (h *PlaylistHandler) ToggleLike(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		response.BadRequest(c, "User not authenticated")
		return
	}

	uuidStr := c.Param("uuid")
	if _, err := uuid.Parse(uuidStr); err != nil {
		response.BadRequest(c, "Invalid playlist UUID")
		return
	}

	liked, err := h.Service.ToggleLike(userID, uuidStr)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{"liked": liked})
}

func (h *PlaylistHandler) GetLikeStatus(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		response.BadRequest(c, "User not authenticated")
		return
	}

	uuidStr := c.Param("uuid")
	if _, err := uuid.Parse(uuidStr); err != nil {
		response.BadRequest(c, "Invalid playlist UUID")
		return
	}

	liked, count, err := h.Service.GetLikeStatus(userID, uuidStr)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{"liked": liked, "like_count": count})
}

func (h *PlaylistHandler) GetProgress(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		response.BadRequest(c, "User not authenticated")
		return
	}

	uuidStr := c.Param("uuid")
	if _, err := uuid.Parse(uuidStr); err != nil {
		response.BadRequest(c, "Invalid playlist UUID")
		return
	}

	resume, err := h.Service.GetProgress(userID, uuidStr)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, resume)
}

func (h *PlaylistHandler) UpdateProgress(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		response.BadRequest(c, "User not authenticated")
		return
	}

	uuidStr := c.Param("uuid")
	if _, err := uuid.Parse(uuidStr); err != nil {
		response.BadRequest(c, "Invalid playlist UUID")
		return
	}

	var req request.UpdatePlaylistProgressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "scene_id is required")
		return
	}

	if err := h.Service.UpdateProgress(userID, uuidStr, req.SceneID, req.PositionS); err != nil {
		response.Error(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}
