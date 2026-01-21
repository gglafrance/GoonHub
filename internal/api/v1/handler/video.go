package handler

import (
	"goonhub/internal/core"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type VideoHandler struct {
	Service           *core.VideoService
	ProcessingService *core.VideoProcessingService
}

func NewVideoHandler(service *core.VideoService, processingService *core.VideoProcessingService) *VideoHandler {
	return &VideoHandler{
		Service:           service,
		ProcessingService: processingService,
	}
}

func (h *VideoHandler) UploadVideo(c *gin.Context) {
	file, err := c.FormFile("video")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Video file is required"})
		return
	}

	title := c.PostForm("title")

	video, err := h.Service.UploadVideo(file, title)
	if err != nil {
		if err.Error() == "invalid file extension" {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload video: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, video)
}

func (h *VideoHandler) ListVideos(c *gin.Context) {
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "20")

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)

	videos, total, err := h.Service.ListVideos(page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list videos"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  videos,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func (h *VideoHandler) ReprocessVideo(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"})
		return
	}

	video, err := h.Service.GetVideo(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
		return
	}

	if err := h.ProcessingService.SubmitVideo(uint(id), video.StoredPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to submit video for processing"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"message": "Video submitted for processing"})
}

func (h *VideoHandler) DeleteVideo(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"})
		return
	}

	if err := h.Service.DeleteVideo(uint(id)); err != nil {
		if err.Error() == "record not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete video"})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}
