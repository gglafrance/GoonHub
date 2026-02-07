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
	"goonhub/internal/streaming"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type SceneHandler struct {
	Service              *core.SceneService
	ProcessingService    *core.SceneProcessingService
	TagService           *core.TagService
	SearchService        *core.SearchService
	RelatedScenesService *core.RelatedScenesService
	MarkerService        *core.MarkerService
	StreamManager        *streaming.Manager
	MaxItemsPerPage      int
}

func NewSceneHandler(service *core.SceneService, processingService *core.SceneProcessingService, tagService *core.TagService, searchService *core.SearchService, relatedScenesService *core.RelatedScenesService, markerService *core.MarkerService, streamManager *streaming.Manager, maxItemsPerPage int) *SceneHandler {
	return &SceneHandler{
		Service:              service,
		ProcessingService:    processingService,
		TagService:           tagService,
		SearchService:        searchService,
		RelatedScenesService: relatedScenesService,
		MarkerService:        markerService,
		StreamManager:        streamManager,
		MaxItemsPerPage:      maxItemsPerPage,
	}
}

func (h *SceneHandler) UploadScene(c *gin.Context) {
	file, err := c.FormFile("scene")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Scene file is required"})
		return
	}

	title := c.PostForm("title")

	scene, err := h.Service.UploadScene(file, title)
	if err != nil {
		if errors.Is(err, apperrors.ErrInvalidFileExtension) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload scene: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, scene)
}

var resolutionToHeight = map[string][2]int{
	"4k":    {2160, 0},
	"1440p": {1440, 2159},
	"1080p": {1080, 1439},
	"720p":  {720, 1079},
	"480p":  {480, 719},
	"360p":  {0, 479},
}

func (h *SceneHandler) ListScenes(c *gin.Context) {
	var req request.SearchScenesRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid query parameters"})
		return
	}

	req.Page, req.Limit = clampPagination(req.Page, req.Limit, 20, h.MaxItemsPerPage)

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

	params := data.SceneSearchParams{
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
		Seed:             req.Seed,
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

	result, err := h.SearchService.Search(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search scenes"})
		return
	}

	resp := gin.H{
		"data":  response.ToSceneListItems(result.Scenes),
		"total": result.Total,
		"page":  req.Page,
		"limit": req.Limit,
	}
	if result.Seed != 0 {
		resp["seed"] = result.Seed
	}
	c.JSON(http.StatusOK, resp)
}

func (h *SceneHandler) GetFilterOptions(c *gin.Context) {
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

func (h *SceneHandler) ReprocessScene(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scene ID"})
		return
	}

	scene, err := h.Service.GetScene(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Scene not found"})
		return
	}

	if err := h.ProcessingService.SubmitScene(uint(id), scene.StoredPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to submit scene for processing"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"message": "Scene submitted for processing"})
}

func (h *SceneHandler) DeleteScene(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scene ID"})
		return
	}

	var req request.DeleteSceneRequest
	// Ignore binding errors - body is optional
	_ = c.ShouldBindJSON(&req)

	if req.Permanent {
		// Permanent delete
		if err := h.Service.HardDeleteScene(uint(id)); err != nil {
			if apperrors.IsNotFound(err) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Scene not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete scene"})
			return
		}
		c.Status(http.StatusNoContent)
		return
	}

	// Move to trash
	expiresAt, err := h.Service.MoveSceneToTrash(uint(id))
	if err != nil {
		if apperrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Scene not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to move scene to trash"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Scene moved to trash",
		"expires_at": expiresAt,
	})
}

func (h *SceneHandler) GetScene(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scene ID"})
		return
	}

	scene, err := h.Service.GetScene(uint(id))
	if err != nil {
		if apperrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Scene not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get scene"})
		return
	}

	c.JSON(http.StatusOK, scene)
}

func (h *SceneHandler) StreamScene(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scene ID"})
		return
	}

	sceneID := uint(id)
	clientIP := c.ClientIP()

	// Acquire stream slot (global + per-IP limits).
	// The limiter tracks by IP+SceneID so concurrent range requests for the
	// same video share a single slot instead of exhausting per-IP limits.
	if !h.StreamManager.Limiter().Acquire(clientIP, sceneID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Too many concurrent streams",
			"code":  "STREAM_LIMIT_EXCEEDED",
		})
		return
	}
	defer h.StreamManager.Limiter().Release(clientIP, sceneID)

	// Get cached path (avoids DB query on repeated range requests)
	filePath, err := h.StreamManager.GetScenePath(sceneID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Scene not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get scene"})
		return
	}

	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Scene file not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open scene file"})
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to access scene file"})
		return
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = "video/mp4"
	}

	c.Header("Content-Type", mimeType)
	c.Header("Cache-Control", "public, max-age=86400")

	// Use the buffer pool for efficient I/O (256KB vs Go's default 32KB)
	buf := h.StreamManager.BufferPool().Get()
	defer h.StreamManager.BufferPool().Put(buf)

	streaming.ServeVideo(c.Writer, c.Request, filepath.Base(filePath), fileInfo.ModTime(), file, buf)
}

func (h *SceneHandler) ExtractThumbnail(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scene ID"})
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

	scene, err := h.Service.GetScene(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get updated scene"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"thumbnail_path":   scene.ThumbnailPath,
		"thumbnail_width":  scene.ThumbnailWidth,
		"thumbnail_height": scene.ThumbnailHeight,
	})
}

func (h *SceneHandler) UpdateSceneDetails(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scene ID"})
		return
	}

	var req request.UpdateSceneDetailsRequest
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

	scene, err := h.Service.UpdateSceneDetails(uint(id), req.Title, req.Description, releaseDate)
	if err != nil {
		if apperrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Scene not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update scene details"})
		return
	}

	c.JSON(http.StatusOK, scene)
}

func (h *SceneHandler) UploadThumbnail(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scene ID"})
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

	scene, err := h.Service.GetScene(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get updated scene"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"thumbnail_path":   scene.ThumbnailPath,
		"thumbnail_width":  scene.ThumbnailWidth,
		"thumbnail_height": scene.ThumbnailHeight,
	})
}

func (h *SceneHandler) ApplySceneMetadata(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scene ID"})
		return
	}

	var req request.ApplySceneMetadataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	scene, err := h.Service.GetScene(uint(id))
	if err != nil {
		if apperrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Scene not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get scene"})
		return
	}

	// Build the update values, using existing values if not provided
	title := scene.Title
	description := scene.Description
	studio := scene.Studio

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

	updatedScene, err := h.Service.UpdateSceneMetadata(uint(id), title, description, studio, releaseDate, porndbSceneID)
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
		updatedScene, _ = h.Service.GetScene(uint(id))
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

			// Merge with current scene tags to avoid overwriting manually-assigned tags
			if currentTags, err := h.TagService.GetSceneTags(uint(id)); err == nil {
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

			if _, err := h.TagService.SetSceneTags(uint(id), allTagIDs); err == nil {
				// Re-fetch to include updated tags
				updatedScene, _ = h.Service.GetScene(uint(id))
			}
		}
	}

	c.JSON(http.StatusOK, updatedScene)
}

func (h *SceneHandler) GetRelatedScenes(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scene ID"})
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

	// Verify the scene exists
	_, err = h.Service.GetScene(uint(id))
	if err != nil {
		if apperrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Scene not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get scene"})
		return
	}

	scenes, err := h.RelatedScenesService.GetRelatedScenes(uint(id), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get related scenes"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  response.ToSceneListItems(scenes),
		"total": len(scenes),
	})
}
