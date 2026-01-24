package handler

import (
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
		if err.Error() == "tag name is required" || err.Error() == "tag name must be 100 characters or less" {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err.Error() == "invalid color format, must be a hex color like #FF4D4D" {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": "Tag already exists"})
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
		if err.Error() == "tag not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete tag"})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

func (h *TagHandler) GetVideoTags(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"})
		return
	}

	tags, err := h.Service.GetVideoTags(uint(id))
	if err != nil {
		if err.Error() == "video not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get video tags"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": tags})
}

type setVideoTagsRequest struct {
	TagIDs []uint `json:"tag_ids"`
}

func (h *TagHandler) SetVideoTags(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"})
		return
	}

	var req setVideoTagsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	tags, err := h.Service.SetVideoTags(uint(id), req.TagIDs)
	if err != nil {
		if err.Error() == "video not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set video tags"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": tags})
}
