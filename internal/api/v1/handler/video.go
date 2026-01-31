package handler

import (
	"errors"
	"fmt"
	"goonhub/internal/api/middleware"
	"goonhub/internal/api/v1/request"
	"goonhub/internal/api/v1/response"
	"goonhub/internal/apperrors"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type VideoHandler struct {
	Service              *core.VideoService
	ProcessingService    *core.VideoProcessingService
	TagService           *core.TagService
	SearchService        *core.SearchService
	RelatedVideosService *core.RelatedVideosService
	MarkerService        *core.MarkerService
}

func NewVideoHandler(service *core.VideoService, processingService *core.VideoProcessingService, tagService *core.TagService, searchService *core.SearchService, relatedVideosService *core.RelatedVideosService, markerService *core.MarkerService) *VideoHandler {
	return &VideoHandler{
		Service:              service,
		ProcessingService:    processingService,
		TagService:           tagService,
		SearchService:        searchService,
		RelatedVideosService: relatedVideosService,
		MarkerService:        markerService,
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
		if errors.Is(err, apperrors.ErrInvalidFileExtension) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload video: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, video)
}

var resolutionToHeight = map[string][2]int{
	"4k":    {2160, 0},
	"1440p": {1440, 2159},
	"1080p": {1080, 1439},
	"720p":  {720, 1079},
	"480p":  {480, 719},
	"360p":  {0, 479},
}

func (h *VideoHandler) ListVideos(c *gin.Context) {
	var req request.SearchVideosRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid query parameters"})
		return
	}

	if req.Page < 1 {
		req.Page = 1
	}
	if req.Limit < 1 {
		req.Limit = 20
	}

	var userID uint
	if payload, err := middleware.GetUserFromContext(c); err == nil {
		userID = payload.UserID
	}

	// Map frontend match_type to Meilisearch matching strategy
	var matchingStrategy string
	switch req.MatchType {
	case "strict":
		matchingStrategy = "all"
	case "frequency":
		matchingStrategy = "frequency"
	default:
		matchingStrategy = "last"
	}

	params := data.VideoSearchParams{
		Page:             req.Page,
		Limit:            req.Limit,
		Query:            req.Query,
		Studio:           req.Studio,
		MinDuration:      req.MinDuration,
		MaxDuration:      req.MaxDuration,
		Sort:             req.Sort,
		UserID:           userID,
		Liked:            req.Liked,
		MinRating:        req.MinRating,
		MaxRating:        req.MaxRating,
		MinJizzCount:     req.MinJizzCount,
		MaxJizzCount:     req.MaxJizzCount,
		MatchingStrategy: matchingStrategy,
	}

	if req.Tags != "" {
		tagNames := strings.Split(req.Tags, ",")
		tags, err := h.TagService.GetTagsByNames(tagNames)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resolve tags"})
			return
		}
		for _, tag := range tags {
			params.TagIDs = append(params.TagIDs, tag.ID)
		}
	}

	if req.Actors != "" {
		params.Actors = strings.Split(req.Actors, ",")
	}

	if req.MarkerLabels != "" {
		params.MarkerLabels = strings.Split(req.MarkerLabels, ",")
	}

	if req.MinDate != "" {
		t, err := time.Parse("2006-01-02", req.MinDate)
		if err == nil {
			params.MinDate = &t
		}
	}
	if req.MaxDate != "" {
		t, err := time.Parse("2006-01-02", req.MaxDate)
		if err == nil {
			endOfDay := t.Add(24*time.Hour - time.Second)
			params.MaxDate = &endOfDay
		}
	}

	if req.Resolution != "" {
		if heights, ok := resolutionToHeight[req.Resolution]; ok {
			params.MinHeight = heights[0]
			params.MaxHeight = heights[1]
		}
	}

	videos, total, err := h.SearchService.Search(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search videos"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  response.ToVideoListItems(videos),
		"total": total,
		"page":  req.Page,
		"limit": req.Limit,
	})
}

func (h *VideoHandler) GetFilterOptions(c *gin.Context) {
	studios, err := h.Service.GetDistinctStudios()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get studios"})
		return
	}

	actors, err := h.Service.GetDistinctActors()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get actors"})
		return
	}

	tags, err := h.TagService.ListTags()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get tags"})
		return
	}

	// Get user-specific marker labels (if authenticated)
	var markerLabels []gin.H
	if payload, err := middleware.GetUserFromContext(c); err == nil {
		labels, err := h.MarkerService.GetLabelSuggestions(payload.UserID, 100)
		if err == nil {
			for _, label := range labels {
				markerLabels = append(markerLabels, gin.H{
					"label": label.Label,
					"count": label.Count,
				})
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"studios":       studios,
		"actors":        actors,
		"tags":          tags,
		"marker_labels": markerLabels,
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
		if apperrors.IsNotFound(err) {
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
		if apperrors.IsNotFound(err) {
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
		if apperrors.IsNotFound(err) {
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

func (h *VideoHandler) UpdateVideoDetails(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"})
		return
	}

	var req request.UpdateVideoDetailsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	var releaseDate *time.Time
	if req.ReleaseDate != nil {
		if *req.ReleaseDate == "" {
			// Empty string means clear the date
			zero := time.Time{}
			releaseDate = &zero
		} else {
			parsed, err := time.Parse("2006-01-02", *req.ReleaseDate)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid release_date format, expected YYYY-MM-DD"})
				return
			}
			releaseDate = &parsed
		}
	}

	video, err := h.Service.UpdateVideoDetails(uint(id), req.Title, req.Description, releaseDate)
	if err != nil {
		if apperrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update video details"})
		return
	}

	c.JSON(http.StatusOK, video)
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
		if errors.Is(err, apperrors.ErrInvalidImageExtension) {
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

func (h *VideoHandler) ApplySceneMetadata(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"})
		return
	}

	var req request.ApplySceneMetadataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	video, err := h.Service.GetVideo(uint(id))
	if err != nil {
		if apperrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get video"})
		return
	}

	// Build the update values, using existing values if not provided
	title := video.Title
	description := video.Description
	studio := video.Studio

	if req.Title != nil {
		title = *req.Title
	}
	if req.Description != nil {
		description = *req.Description
	}
	if req.Studio != nil {
		studio = *req.Studio
	}

	var releaseDate *time.Time
	if req.ReleaseDate != nil && *req.ReleaseDate != "" {
		parsed, err := time.Parse("2006-01-02", *req.ReleaseDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid release_date format, expected YYYY-MM-DD"})
			return
		}
		releaseDate = &parsed
	}

	porndbSceneID := ""
	if req.PornDBSceneID != nil {
		porndbSceneID = *req.PornDBSceneID
	}

	updatedVideo, err := h.Service.UpdateSceneMetadata(uint(id), title, description, studio, releaseDate, porndbSceneID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update scene metadata"})
		return
	}

	// Import thumbnail from URL if provided
	if req.ThumbnailURL != nil && *req.ThumbnailURL != "" {
		if err := h.Service.SetThumbnailFromURL(uint(id), *req.ThumbnailURL); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to import thumbnail: %v", err)})
			return
		}
		// Re-fetch to include updated thumbnail
		updatedVideo, _ = h.Service.GetVideo(uint(id))
	}

	// Import tags if provided (best-effort, skip on errors)
	if len(req.TagNames) > 0 {
		if existingTags, err := h.TagService.GetTagsByNames(req.TagNames); err == nil {
			// Build set of existing tag names for fast lookup
			existingNames := make(map[string]struct{}, len(existingTags))
			allTagIDs := make([]uint, 0, len(req.TagNames))
			for _, t := range existingTags {
				existingNames[t.Name] = struct{}{}
				allTagIDs = append(allTagIDs, t.ID)
			}

			// Create missing tags
			for _, name := range req.TagNames {
				if _, found := existingNames[name]; !found {
					newTag, err := h.TagService.CreateTag(name, "")
					if err != nil {
						continue
					}
					allTagIDs = append(allTagIDs, newTag.ID)
				}
			}

			// Merge with current video tags to avoid overwriting manually-assigned tags
			if currentTags, err := h.TagService.GetVideoTags(uint(id)); err == nil {
				seen := make(map[uint]struct{}, len(allTagIDs))
				for _, tid := range allTagIDs {
					seen[tid] = struct{}{}
				}
				for _, ct := range currentTags {
					if _, exists := seen[ct.ID]; !exists {
						allTagIDs = append(allTagIDs, ct.ID)
					}
				}
			}

			if _, err := h.TagService.SetVideoTags(uint(id), allTagIDs); err == nil {
				// Re-fetch to include updated tags
				updatedVideo, _ = h.Service.GetVideo(uint(id))
			}
		}
	}

	c.JSON(http.StatusOK, updatedVideo)
}

func (h *VideoHandler) GetRelatedVideos(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"})
		return
	}

	// Parse optional limit parameter
	limit := 15
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 50 {
		limit = 50
	}

	// Verify the video exists
	_, err = h.Service.GetVideo(uint(id))
	if err != nil {
		if apperrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get video"})
		return
	}

	videos, err := h.RelatedVideosService.GetRelatedVideos(uint(id), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get related videos"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  response.ToVideoListItems(videos),
		"total": len(videos),
	})
}
