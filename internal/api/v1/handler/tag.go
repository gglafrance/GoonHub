package handler

import (
	"goonhub/internal/api/v1/response"
	"goonhub/internal/apperrors"
	"goonhub/internal/core"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type TagHandler struct {
	Service *core.TagService
}

func NewTagHandler(service *core.TagService) *TagHandler {
	return &TagHandler{Service: service}
}

func (h *TagHandler) ListTags(c *gin.Context) {
	tags, err := h.Service.ListTags()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list tags"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": tags})
}

type createTagRequest struct {
	Name  string `json:"name" binding:"required"`
	Color string `json:"color"`
}

func (h *TagHandler) CreateTag(c *gin.Context) {
	var req createTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name is required"})
		return
	}

	tag, err := h.Service.CreateTag(req.Name, req.Color)
	if err != nil {
		if apperrors.IsValidation(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if apperrors.IsConflict(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "Tag already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create tag"})
		return
	}

	c.JSON(http.StatusCreated, tag)
}

func (h *TagHandler) DeleteTag(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tag ID"})
		return
	}

	if err := h.Service.DeleteTag(uint(id)); err != nil {
		if apperrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete tag"})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

func (h *TagHandler) GetSceneTags(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scene ID"})
		return
	}

	tags, err := h.Service.GetSceneTags(uint(id))
	if err != nil {
		if apperrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Scene not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get scene tags"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": tags})
}

type setSceneTagsRequest struct {
	TagIDs []uint `json:"tag_ids"`
}

func (h *TagHandler) SetSceneTags(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scene ID"})
		return
	}

	var req setSceneTagsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	tags, err := h.Service.SetSceneTags(uint(id), req.TagIDs)
	if err != nil {
		if apperrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Scene not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set scene tags"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": tags})
}

type updateTagRequest struct {
	Color string `json:"color" binding:"required"`
}

// UpdateTag changes a tag's color (PUT /tags/:id, body {color}).
func (h *TagHandler) UpdateTag(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tag ID"})
		return
	}
	var req updateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Color is required"})
		return
	}
	tag, err := h.Service.UpdateTagColor(uint(id), req.Color)
	if err != nil {
		response.Error(c, err)
		return
	}
	c.JSON(http.StatusOK, tag)
}

// ListDuplicateTags returns the groups of tags whose names are equal ignoring case and extra
// spaces (GET /tags/duplicates).
func (h *TagHandler) ListDuplicateTags(c *gin.Context) {
	groups, err := h.Service.FindDuplicates()
	if err != nil {
		response.Error(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": groups})
}

type mergeTagsRequest struct {
	TargetID  uint   `json:"target_id" binding:"required"`
	SourceIDs []uint `json:"source_ids" binding:"required"`
	// DryRun only counts what the merge would change.
	DryRun bool `json:"dry_run"`
}

// MergeTags merges source tags into a target tag (POST /tags/merge, body
// {target_id, source_ids, dry_run}).
func (h *TagHandler) MergeTags(c *gin.Context) {
	var req mergeTagsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target_id and source_ids are required"})
		return
	}
	res, err := h.Service.MergeTags(req.TargetID, req.SourceIDs, req.DryRun)
	if err != nil {
		response.Error(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}
