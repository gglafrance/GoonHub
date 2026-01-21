package handler

import (
	"goonhub/internal/core"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type VideoHandler struct {
	Service *core.VideoService
}

func NewVideoHandler(service *core.VideoService) *VideoHandler {
	return &VideoHandler{Service: service}
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
