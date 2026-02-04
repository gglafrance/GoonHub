package handler

import (
	"goonhub/internal/api/v1/request"
	"goonhub/internal/api/v1/response"
	"goonhub/internal/core"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type ExplorerHandler struct {
	Service *core.ExplorerService
}

func NewExplorerHandler(service *core.ExplorerService) *ExplorerHandler {
	return &ExplorerHandler{
		Service: service,
	}
}

// GetStoragePaths returns all storage paths with their scene counts
func (h *ExplorerHandler) GetStoragePaths(c *gin.Context) {
	paths, err := h.Service.GetStoragePathsWithCounts()
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{"storage_paths": paths})
}

// GetFolderContents returns the contents of a folder (subfolders and videos)
func (h *ExplorerHandler) GetFolderContents(c *gin.Context) {
	storagePathID, err := strconv.ParseUint(c.Param("storagePathID"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid storage path ID")
		return
	}

	// Get the path parameter (everything after the storage path ID)
	// The route is: /folders/:storagePathID/*path
	folderPath := c.Param("path")
	// Remove leading slash if present
	folderPath = strings.TrimPrefix(folderPath, "/")

	page := 1
	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	limit := 24
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	contents, err := h.Service.GetFolderContents(uint(storagePathID), folderPath, page, limit)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, response.ToFolderContentsResponse(contents))
}

// BulkUpdateTags updates tags for multiple videos
func (h *ExplorerHandler) BulkUpdateTags(c *gin.Context) {
	var req request.BulkUpdateTagsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	updated, err := h.Service.BulkUpdateTags(core.BulkUpdateTagsRequest{
		SceneIDs: req.SceneIDs,
		TagIDs:   req.TagIDs,
		Mode:     req.Mode,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{
		"updated":   updated,
		"requested": len(req.SceneIDs),
	})
}

// BulkUpdateActors updates actors for multiple videos
func (h *ExplorerHandler) BulkUpdateActors(c *gin.Context) {
	var req request.BulkUpdateActorsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	updated, err := h.Service.BulkUpdateActors(core.BulkUpdateActorsRequest{
		SceneIDs: req.SceneIDs,
		ActorIDs: req.ActorIDs,
		Mode:     req.Mode,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{
		"updated":   updated,
		"requested": len(req.SceneIDs),
	})
}

// BulkUpdateStudio updates studio for multiple videos
func (h *ExplorerHandler) BulkUpdateStudio(c *gin.Context) {
	var req request.BulkUpdateStudioRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	updated, err := h.Service.BulkUpdateStudio(core.BulkUpdateStudioRequest{
		SceneIDs: req.SceneIDs,
		Studio:   req.Studio,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{
		"updated":   updated,
		"requested": len(req.SceneIDs),
	})
}

// GetFolderSceneIDs returns all scene IDs in a folder
func (h *ExplorerHandler) GetFolderSceneIDs(c *gin.Context) {
	var req request.FolderSceneIDsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	ids, err := h.Service.GetFolderSceneIDs(req.StoragePathID, req.FolderPath, req.Recursive)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{
		"scene_ids": ids,
		"count":     len(ids),
	})
}

// BulkDeleteScenes deletes multiple scenes
func (h *ExplorerHandler) BulkDeleteScenes(c *gin.Context) {
	var req request.BulkDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	deleted, err := h.Service.BulkDeleteScenes(req.SceneIDs, req.Permanent)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{
		"deleted":   deleted,
		"requested": len(req.SceneIDs),
	})
}

// GetScenesMatchInfo returns minimal scene data for bulk PornDB matching
func (h *ExplorerHandler) GetScenesMatchInfo(c *gin.Context) {
	var req request.ScenesMatchInfoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	scenes, err := h.Service.GetScenesMatchInfo(req.SceneIDs)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, response.ScenesMatchInfoResponse{Scenes: scenes})
}

// SearchInFolder searches for videos within a folder scope
func (h *ExplorerHandler) SearchInFolder(c *gin.Context) {
	var req request.FolderSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	result, err := h.Service.SearchInFolder(core.FolderSearchRequest{
		StoragePathID: req.StoragePathID,
		FolderPath:    req.FolderPath,
		Recursive:     req.Recursive,
		Query:         req.Query,
		TagIDs:        req.TagIDs,
		Actors:        req.Actors,
		Studio:        req.Studio,
		MinDuration:   req.MinDuration,
		MaxDuration:   req.MaxDuration,
		Sort:          req.Sort,
		HasPornDBID:   req.HasPornDBID,
		Page:          req.Page,
		Limit:         req.Limit,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, response.ToFolderSearchResponse(result))
}
