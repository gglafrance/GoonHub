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

// GetStoragePaths returns all storage paths with their video counts
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

	response.OK(c, contents)
}

// BulkUpdateTags updates tags for multiple videos
func (h *ExplorerHandler) BulkUpdateTags(c *gin.Context) {
	var req request.BulkUpdateTagsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	updated, err := h.Service.BulkUpdateTags(core.BulkUpdateTagsRequest{
		VideoIDs: req.VideoIDs,
		TagIDs:   req.TagIDs,
		Mode:     req.Mode,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{
		"updated":   updated,
		"requested": len(req.VideoIDs),
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
		VideoIDs: req.VideoIDs,
		ActorIDs: req.ActorIDs,
		Mode:     req.Mode,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{
		"updated":   updated,
		"requested": len(req.VideoIDs),
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
		VideoIDs: req.VideoIDs,
		Studio:   req.Studio,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{
		"updated":   updated,
		"requested": len(req.VideoIDs),
	})
}

// GetFolderVideoIDs returns all video IDs in a folder
func (h *ExplorerHandler) GetFolderVideoIDs(c *gin.Context) {
	var req request.FolderVideoIDsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	ids, err := h.Service.GetFolderVideoIDs(req.StoragePathID, req.FolderPath, req.Recursive)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{
		"video_ids": ids,
		"count":     len(ids),
	})
}

// BulkDeleteVideos deletes multiple videos
func (h *ExplorerHandler) BulkDeleteVideos(c *gin.Context) {
	var req request.BulkDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	deleted, err := h.Service.BulkDeleteVideos(req.VideoIDs)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{
		"deleted":   deleted,
		"requested": len(req.VideoIDs),
	})
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
		Page:          req.Page,
		Limit:         req.Limit,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, result)
}
