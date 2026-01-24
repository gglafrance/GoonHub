package handler

import (
	"fmt"
	"goonhub/internal/api/v1/request"
	"goonhub/internal/core"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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

func (h *VideoHandler) GetVideo(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"})
		return
	}

	video, err := h.Service.GetVideo(uint(id))
	if err != nil {
		if err.Error() == "record not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get video"})
		return
	}

	c.JSON(http.StatusOK, video)
}

func (h *VideoHandler) StreamVideo(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"})
		return
	}

	video, err := h.Service.GetVideo(uint(id))
	if err != nil {
		if err.Error() == "record not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get video"})
		return
	}

	filePath := video.StoredPath
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Video file not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to access video file"})
		return
	}

	fileSize := fileInfo.Size()
	file, err := os.Open(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open video file"})
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(filePath))
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = "video/mp4"
	}

	rangeHeader := c.GetHeader("Range")
	if rangeHeader == "" {
		c.Header("Content-Length", strconv.FormatInt(fileSize, 10))
		c.Header("Content-Type", mimeType)
		c.Header("Accept-Ranges", "bytes")
		c.Header("Cache-Control", "public, max-age=86400")

		_, err = io.Copy(c.Writer, file)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		return
	}

	if !strings.HasPrefix(rangeHeader, "bytes=") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Range header"})
		return
	}

	rangeSpec := strings.TrimPrefix(rangeHeader, "bytes=")
	ranges := strings.Split(rangeSpec, "-")
	if len(ranges) != 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Range format"})
		return
	}

	var start, end int64
	start, err = strconv.ParseInt(ranges[0], 10, 64)
	if err != nil {
		start = 0
	}

	if ranges[1] == "" {
		end = fileSize - 1
	} else {
		end, err = strconv.ParseInt(ranges[1], 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Range end value"})
			return
		}
	}

	if start < 0 || end >= fileSize || start > end {
		c.JSON(http.StatusRequestedRangeNotSatisfiable, gin.H{
			"error":     "Requested Range Not Satisfiable",
			"start":     start,
			"end":       end,
			"file_size": fileSize,
		})
		return
	}

	contentLength := end - start + 1
	c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
	c.Header("Content-Length", strconv.FormatInt(contentLength, 10))
	c.Header("Content-Type", mimeType)
	c.Header("Accept-Ranges", "bytes")
	c.Header("Cache-Control", "public, max-age=86400")
	c.Status(http.StatusPartialContent)

	_, err = file.Seek(start, 0)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	_, err = io.CopyN(c.Writer, file, contentLength)
	if err != nil {
		return
	}
}

func (h *VideoHandler) ExtractThumbnail(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"})
		return
	}

	var req request.ExtractThumbnailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: timecode is required and must be >= 0"})
		return
	}

	if err := h.Service.SetThumbnailFromTimecode(uint(id), req.Timecode); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to extract thumbnail: " + err.Error()})
		return
	}

	video, err := h.Service.GetVideo(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get updated video"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"thumbnail_path":   video.ThumbnailPath,
		"thumbnail_width":  video.ThumbnailWidth,
		"thumbnail_height": video.ThumbnailHeight,
	})
}

func (h *VideoHandler) UploadThumbnail(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"})
		return
	}

	file, err := c.FormFile("thumbnail")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Thumbnail file is required"})
		return
	}

	if file.Size > 10*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File size must be less than 10MB"})
		return
	}

	if err := h.Service.SetThumbnailFromUpload(uint(id), file); err != nil {
		if strings.Contains(err.Error(), "invalid image extension") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload thumbnail: " + err.Error()})
		return
	}

	video, err := h.Service.GetVideo(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get updated video"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"thumbnail_path":   video.ThumbnailPath,
		"thumbnail_width":  video.ThumbnailWidth,
		"thumbnail_height": video.ThumbnailHeight,
	})
}
