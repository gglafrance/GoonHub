package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"goonhub/internal/api/v1/request"
	"goonhub/internal/api/v1/response"
	"goonhub/internal/apperrors"
	"goonhub/internal/data"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ImportHandler struct {
	sceneRepo  data.SceneRepository
	markerRepo data.MarkerRepository
	logger     *zap.Logger
}

func NewImportHandler(sceneRepo data.SceneRepository, markerRepo data.MarkerRepository, logger *zap.Logger) *ImportHandler {
	return &ImportHandler{
		sceneRepo:  sceneRepo,
		markerRepo: markerRepo,
		logger:     logger,
	}
}

// ImportScene creates a scene record from pre-existing metadata without triggering processing.
func (h *ImportHandler) ImportScene(c *gin.Context) {
	var req request.ImportSceneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	// Verify the file exists at the stored path (unless explicitly skipped)
	if !req.SkipFileCheck {
		if _, err := os.Stat(req.StoredPath); err != nil {
			if os.IsNotExist(err) {
				response.Error(c, apperrors.NewValidationError(fmt.Sprintf("file not found at stored_path: %s", req.StoredPath)))
				return
			}
			h.logger.Error("failed to stat file", zap.String("path", req.StoredPath), zap.Error(err))
			response.Error(c, apperrors.NewInternalError("failed to verify file", err))
			return
		}
	}

	// Check if a scene with this path already exists
	existing, err := h.sceneRepo.GetByStoredPath(req.StoredPath)
	if err != nil && err != gorm.ErrRecordNotFound {
		h.logger.Error("failed to check existing scene", zap.String("path", req.StoredPath), zap.Error(err))
		response.Error(c, apperrors.NewInternalError("failed to check existing scene", err))
		return
	}
	if existing != nil {
		c.JSON(http.StatusConflict, gin.H{
			"id":    existing.ID,
			"title": existing.Title,
			"error": fmt.Sprintf("scene already exists at path: %s", req.StoredPath),
		})
		return
	}

	// Parse release date if provided
	var releaseDate *time.Time
	if req.ReleaseDate != nil && *req.ReleaseDate != "" {
		parsed, err := time.Parse("2006-01-02", *req.ReleaseDate)
		if err != nil {
			response.Error(c, apperrors.NewValidationError(fmt.Sprintf("invalid release_date format, expected YYYY-MM-DD: %s", *req.ReleaseDate)))
			return
		}
		releaseDate = &parsed
	}

	// Derive original filename from stored path if not provided
	originalFilename := req.OriginalFilename
	if originalFilename == "" {
		originalFilename = filepath.Base(req.StoredPath)
	}

	// Validate and default origin
	origin := req.Origin
	if origin != "" && !data.IsValidSceneOrigin(origin) {
		response.Error(c, apperrors.NewValidationError(fmt.Sprintf("invalid origin: %s", origin)))
		return
	}

	// Validate and default type
	sceneType := req.Type
	if sceneType != "" && !data.IsValidSceneType(sceneType) {
		response.Error(c, apperrors.NewValidationError(fmt.Sprintf("invalid type: %s", sceneType)))
		return
	}

	scene := &data.Scene{
		Title:            req.Title,
		StoredPath:       req.StoredPath,
		OriginalFilename: originalFilename,
		Size:             req.Size,
		Duration:         req.Duration,
		Width:            req.Width,
		Height:           req.Height,
		FrameRate:        req.FrameRate,
		BitRate:          req.BitRate,
		VideoCodec:       req.VideoCodec,
		AudioCodec:       req.AudioCodec,
		Description:      req.Description,
		ReleaseDate:      releaseDate,
		StudioID:         req.StudioID,
		StoragePathID:    req.StoragePathID,
		ProcessingStatus: "completed",
		Origin:           origin,
		Type:             sceneType,
		Tags:             pq.StringArray{},
		Actors:           pq.StringArray{},
	}

	if err := h.sceneRepo.Create(scene); err != nil {
		h.logger.Error("failed to create imported scene", zap.String("title", req.Title), zap.Error(err))
		response.Error(c, apperrors.NewInternalError("failed to create scene record", err))
		return
	}

	h.logger.Info("imported scene", zap.Uint("id", scene.ID), zap.String("title", scene.Title))
	response.Created(c, gin.H{
		"id":    scene.ID,
		"title": scene.Title,
	})
}

// ImportMarker creates a marker record without generating a thumbnail.
func (h *ImportHandler) ImportMarker(c *gin.Context) {
	var req request.ImportMarkerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	// Verify the scene exists
	_, err := h.sceneRepo.GetByID(req.SceneID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Error(c, apperrors.NewNotFoundError("scene", req.SceneID))
			return
		}
		h.logger.Error("failed to verify scene", zap.Uint("sceneID", req.SceneID), zap.Error(err))
		response.Error(c, apperrors.NewInternalError("failed to verify scene", err))
		return
	}

	// Default color
	color := req.Color
	if color == "" {
		color = "#FFFFFF"
	}

	marker := &data.UserSceneMarker{
		UserID:    req.UserID,
		SceneID:   req.SceneID,
		Timestamp: req.Timestamp,
		Label:     req.Label,
		Color:     color,
	}

	if err := h.markerRepo.Create(marker); err != nil {
		h.logger.Error("failed to create imported marker", zap.Uint("sceneID", req.SceneID), zap.Error(err))
		response.Error(c, apperrors.NewInternalError("failed to create marker record", err))
		return
	}

	// Apply label tags if the marker has a label (best effort)
	if req.Label != "" {
		if err := h.markerRepo.ApplyLabelTagsToMarker(req.UserID, marker.ID, req.Label); err != nil {
			h.logger.Warn("failed to apply label tags to imported marker",
				zap.Uint("markerID", marker.ID),
				zap.String("label", req.Label),
				zap.Error(err))
		}
	}

	h.logger.Info("imported marker", zap.Uint("id", marker.ID), zap.Uint("sceneID", req.SceneID))
	response.Created(c, gin.H{
		"id":       marker.ID,
		"scene_id": marker.SceneID,
	})
}
