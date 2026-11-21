package handler

import (
	"errors"
	"fmt"
	"net/http"

	"goonhub/internal/api/middleware"
	"goonhub/internal/api/v1/request"
	"goonhub/internal/api/v1/response"
	"goonhub/internal/core"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ImportHandler exposes the admin import API (v2) used by external importers.
type ImportHandler struct {
	service *core.ImportService
	logger  *zap.Logger
}

func NewImportHandler(service *core.ImportService, logger *zap.Logger) *ImportHandler {
	return &ImportHandler{
		service: service,
		logger:  logger,
	}
}

// Capabilities lets a client detect import API v2 support.
func (h *ImportHandler) Capabilities(c *gin.Context) {
	features := make([]string, len(core.ImportFeatures))
	copy(features, core.ImportFeatures)
	response.OK(c, response.ImportCapabilitiesResponse{
		Version:  core.ImportAPIVersion,
		Features: features,
	})
}

// ImportScene creates (or updates, with update_existing) a scene record from pre-existing
// metadata without triggering processing.
func (h *ImportHandler) ImportScene(c *gin.Context) {
	payload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req request.ImportSceneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	result, err := h.service.ImportScene(payload.UserID, toImportSceneInput(req))
	if err != nil {
		var conflict *core.ImportConflictError
		if errors.As(err, &conflict) {
			c.JSON(http.StatusConflict, response.ImportSceneConflictResponse{
				ID:    conflict.ID,
				Title: conflict.Title,
				Error: conflict.Error(),
			})
			return
		}
		response.Error(c, err)
		return
	}

	body := response.ImportSceneResponse{ID: result.ID, Title: result.Title, Created: result.Created}
	if result.Created {
		response.Created(c, body)
		return
	}
	response.OK(c, body)
}

// ImportMarker creates (or updates, with update_existing) a marker record without
// generating a thumbnail. Markers are deduplicated on (user_id, scene_id, timestamp, label).
func (h *ImportHandler) ImportMarker(c *gin.Context) {
	payload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req request.ImportMarkerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	result, err := h.service.ImportMarker(payload.UserID, core.ImportMarkerInput{
		SceneID:        req.SceneID,
		UserID:         req.UserID,
		Timestamp:      req.Timestamp,
		Label:          req.Label,
		Color:          req.Color,
		TagIDs:         req.TagIDs,
		UpdateExisting: req.UpdateExisting,
	})
	if err != nil {
		var conflict *core.ImportConflictError
		if errors.As(err, &conflict) {
			c.JSON(http.StatusConflict, response.ImportMarkerConflictResponse{
				ID:      conflict.ID,
				SceneID: conflict.SceneID,
				Error:   conflict.Error(),
			})
			return
		}
		response.Error(c, err)
		return
	}

	body := response.ImportMarkerResponse{ID: result.ID, SceneID: result.SceneID, Created: result.Created}
	if result.Created {
		response.Created(c, body)
		return
	}
	response.OK(c, body)
}

// SceneSummary returns the imported state of the given scenes for post-import verification.
func (h *ImportHandler) SceneSummary(c *gin.Context) {
	payload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req request.ImportSceneSummaryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	result, err := h.service.Summary(payload.UserID, req.IDs)
	if err != nil {
		response.Error(c, err)
		return
	}

	items := make([]response.ImportSceneSummaryItem, len(result.Scenes))
	for i, s := range result.Scenes {
		items[i] = response.ImportSceneSummaryItem{
			ID:          s.ID,
			Title:       s.Title,
			StoredPath:  s.StoredPath,
			StudioID:    s.StudioID,
			TagIDs:      s.TagIDs,
			ActorIDs:    s.ActorIDs,
			ViewCount:   s.ViewCount,
			MarkerCount: s.MarkerCount,
			Rating:      s.Rating,
			Liked:       s.Liked,
			JizzedCount: s.JizzedCount,
			WatchCount:  s.WatchCount,
			Trashed:     s.Trashed,
		}
	}
	response.OK(c, response.ImportSceneSummaryResponse{Data: items, Missing: result.Missing})
}

func toImportSceneInput(req request.ImportSceneRequest) core.ImportSceneInput {
	in := core.ImportSceneInput{
		Title:            req.Title,
		StoredPath:       req.StoredPath,
		OriginalFilename: req.OriginalFilename,
		Size:             req.Size,
		Duration:         req.Duration,
		Width:            req.Width,
		Height:           req.Height,
		FrameRate:        req.FrameRate,
		BitRate:          req.BitRate,
		VideoCodec:       req.VideoCodec,
		AudioCodec:       req.AudioCodec,
		Description:      req.Description,
		ReleaseDate:      req.ReleaseDate,
		StudioID:         req.StudioID,
		StoragePathID:    req.StoragePathID,
		Origin:           req.Origin,
		Type:             req.Type,
		SkipFileCheck:    req.SkipFileCheck,
		TagIDs:           req.TagIDs,
		ActorIDs:         req.ActorIDs,
		ViewCount:        req.ViewCount,
		CreatedAt:        req.CreatedAt,
		FileCreatedAt:    req.FileCreatedAt,
		UpdateExisting:   req.UpdateExisting,
		SkipIndex:        req.SkipIndex,
	}
	if req.Interactions != nil {
		watches := make([]core.ImportWatchInput, len(req.Interactions.Watches))
		for i, w := range req.Interactions.Watches {
			watches[i] = core.ImportWatchInput{
				WatchedAt:     w.WatchedAt,
				WatchDuration: w.WatchDuration,
				LastPosition:  w.LastPosition,
				Completed:     w.Completed,
			}
		}
		in.Interactions = &core.ImportInteractionsInput{
			Rating:      req.Interactions.Rating,
			Liked:       req.Interactions.Liked,
			JizzedCount: req.Interactions.JizzedCount,
			Watches:     watches,
		}
	}
	return in
}
