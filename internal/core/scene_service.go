package core

import (
	"errors"
	"fmt"
	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"goonhub/pkg/ffmpeg"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type SceneService struct {
	Repo              data.SceneRepository
	ScenePath         string
	MetadataPath      string
	ProcessingService *SceneProcessingService
	EventBus          *EventBus
	logger            *zap.Logger
	indexer           SceneIndexer
}

func NewSceneService(repo data.SceneRepository, scenePath string, metadataPath string, processingService *SceneProcessingService, eventBus *EventBus, logger *zap.Logger) *SceneService {
	// Ensure scene directory exists
	if err := os.MkdirAll(scenePath, 0755); err != nil {
		logger.Warn("Failed to create scene directory",
			zap.String("directory", scenePath),
			zap.Error(err),
		)
	}
	// Ensure metadata directory exists
	if err := os.MkdirAll(metadataPath, 0755); err != nil {
		logger.Warn("Failed to create metadata directory",
			zap.String("directory", metadataPath),
			zap.Error(err),
		)
	}
	return &SceneService{
		Repo:              repo,
		ScenePath:         scenePath,
		MetadataPath:      metadataPath,
		ProcessingService: processingService,
		EventBus:          eventBus,
		logger:            logger,
	}
}

// SetIndexer sets the scene indexer for search index updates.
// This is called after service initialization to avoid circular dependencies.
func (s *SceneService) SetIndexer(indexer SceneIndexer) {
	s.indexer = indexer
}

var AllowedExtensions = map[string]bool{
	".mp4":  true,
	".mkv":  true,
	".avi":  true,
	".mov":  true,
	".webm": true,
	".wmv":  true,
	".m4v":  true,
}

func (s *SceneService) ValidateExtension(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return AllowedExtensions[ext]
}

func (s *SceneService) UploadScene(file *multipart.FileHeader, title string) (*data.Scene, error) {
	if !s.ValidateExtension(file.Filename) {
		return nil, apperrors.ErrInvalidFileExtension
	}

	src, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	// Generate unique filename
	uniqueName := fmt.Sprintf("%s_%s", uuid.New().String(), file.Filename)
	storedPath := filepath.Join(s.ScenePath, uniqueName)

	// Save file
	dst, err := os.Create(storedPath)
	if err != nil {
		return nil, err
	}
	defer dst.Close()

	if _, err = io.Copy(dst, src); err != nil {
		return nil, err
	}

	if title == "" {
		title = file.Filename
	}

	scene := &data.Scene{
		Title:            title,
		OriginalFilename: file.Filename,
		StoredPath:       storedPath,
		Size:             file.Size,
		ProcessingStatus: "pending",
		Tags:             pq.StringArray{},
		Actors:           pq.StringArray{},
	}

	if stat, err := os.Stat(storedPath); err == nil {
		modTime := stat.ModTime()
		scene.FileCreatedAt = &modTime
	}

	if err := s.Repo.Create(scene); err != nil {
		// Cleanup file if DB insert fails
		os.Remove(storedPath)
		return nil, err
	}

	if s.ProcessingService != nil {
		// Submit scene for processing synchronously - this is just a queue operation,
		// not the actual processing work, so it's safe to block briefly
		if err := s.ProcessingService.SubmitScene(scene.ID, storedPath); err != nil {
			s.logger.Error("Failed to submit scene for processing",
				zap.Uint("scene_id", scene.ID),
				zap.String("scene_path", storedPath),
				zap.Error(err),
			)
			// Don't fail the upload - scene is saved but processing won't start automatically
			// Users can manually trigger processing via the admin API
		}
	}

	// Index scene in search engine
	if s.indexer != nil {
		if err := s.indexer.IndexScene(scene); err != nil {
			s.logger.Warn("Failed to index scene for search",
				zap.Uint("scene_id", scene.ID),
				zap.Error(err),
			)
		}
	}

	return scene, nil
}

func (s *SceneService) ListScenes(page, limit int) ([]data.Scene, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	return s.Repo.List(page, limit)
}

func (s *SceneService) GetDistinctStudios() ([]string, error) {
	return s.Repo.GetDistinctStudios()
}

func (s *SceneService) GetDistinctActors() ([]string, error) {
	return s.Repo.GetDistinctActors()
}

func (s *SceneService) GetScene(id uint) (*data.Scene, error) {
	scene, err := s.Repo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrSceneNotFound(id)
		}
		return nil, apperrors.NewInternalError("failed to get scene", err)
	}
	return scene, nil
}

func (s *SceneService) UpdateSceneDetails(id uint, title, description string, releaseDate *time.Time) (*data.Scene, error) {
	if err := s.Repo.UpdateDetails(id, title, description, releaseDate); err != nil {
		return nil, fmt.Errorf("failed to update scene details: %w", err)
	}

	scene, err := s.Repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	// Update search index
	if s.indexer != nil {
		if err := s.indexer.UpdateSceneIndex(scene); err != nil {
			s.logger.Warn("Failed to update scene in search index",
				zap.Uint("scene_id", id),
				zap.Error(err),
			)
		}
	}

	return scene, nil
}

func (s *SceneService) UpdateSceneMetadata(id uint, title, description, studio string, releaseDate *time.Time, porndbSceneID string) (*data.Scene, error) {
	if err := s.Repo.UpdateSceneMetadata(id, title, description, studio, releaseDate, porndbSceneID); err != nil {
		return nil, fmt.Errorf("failed to update scene metadata: %w", err)
	}

	scene, err := s.Repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	// Update search index
	if s.indexer != nil {
		if err := s.indexer.UpdateSceneIndex(scene); err != nil {
			s.logger.Warn("Failed to update scene in search index",
				zap.Uint("scene_id", id),
				zap.Error(err),
			)
		}
	}

	return scene, nil
}

func (s *SceneService) DeleteScene(id uint) error {
	scene, err := s.Repo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrSceneNotFound(id)
		}
		return apperrors.NewInternalError("failed to get scene", err)
	}

	if err := s.Repo.Delete(id); err != nil {
		return apperrors.NewInternalError("failed to delete scene", err)
	}

	// Remove from search index
	if s.indexer != nil {
		if err := s.indexer.DeleteSceneIndex(id); err != nil {
			s.logger.Warn("Failed to delete scene from search index",
				zap.Uint("scene_id", id),
				zap.Error(err),
			)
		}
	}

	os.Remove(scene.StoredPath)

	if scene.ThumbnailPath != "" {
		os.Remove(scene.ThumbnailPath)
	}

	if scene.SpriteSheetPath != "" {
		spriteDir := filepath.Join(s.MetadataPath, "sprites")
		spritePattern := filepath.Join(spriteDir, fmt.Sprintf("%d_sheet_*.jpg", id))
		files, _ := filepath.Glob(spritePattern)
		for _, file := range files {
			os.Remove(file)
		}
	}

	if scene.VttPath != "" {
		os.Remove(scene.VttPath)
	}

	return nil
}

var allowedImageExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
}

func (s *SceneService) SetThumbnailFromTimecode(sceneID uint, timecode float64) error {
	scene, err := s.Repo.GetByID(sceneID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrSceneNotFound(sceneID)
		}
		return apperrors.NewInternalError("failed to get scene", err)
	}

	if scene.Width == 0 || scene.Height == 0 {
		return apperrors.ErrSceneDimensionsNotAvailable
	}

	qualityConfig := s.ProcessingService.GetProcessingQualityConfig()

	tileWidthSm, tileHeightSm := ffmpeg.CalculateTileDimensions(scene.Width, scene.Height, qualityConfig.MaxFrameDimensionSm)
	tileWidthLg, tileHeightLg := ffmpeg.CalculateTileDimensions(scene.Width, scene.Height, qualityConfig.MaxFrameDimensionLg)

	thumbnailDir := filepath.Join(s.MetadataPath, "thumbnails")
	if err := os.MkdirAll(thumbnailDir, 0755); err != nil {
		return fmt.Errorf("failed to create thumbnail directory: %w", err)
	}

	seekPos := strconv.FormatFloat(timecode, 'f', 3, 64)
	smPath := filepath.Join(thumbnailDir, fmt.Sprintf("%d_thumb_sm.webp", sceneID))
	lgPath := filepath.Join(thumbnailDir, fmt.Sprintf("%d_thumb_lg.webp", sceneID))

	if err := ffmpeg.ExtractThumbnail(scene.StoredPath, smPath, seekPos, tileWidthSm, tileHeightSm, qualityConfig.FrameQualitySm); err != nil {
		return fmt.Errorf("failed to extract small thumbnail: %w", err)
	}

	if err := ffmpeg.ExtractThumbnail(scene.StoredPath, lgPath, seekPos, tileWidthLg, tileHeightLg, qualityConfig.FrameQualityLg); err != nil {
		return fmt.Errorf("failed to extract large thumbnail: %w", err)
	}

	if err := s.Repo.UpdateThumbnail(sceneID, smPath, tileWidthSm, tileHeightSm); err != nil {
		return fmt.Errorf("failed to update thumbnail in database: %w", err)
	}

	if s.EventBus != nil {
		s.EventBus.Publish(SceneEvent{
			Type:    "scene:thumbnail_complete",
			SceneID: sceneID,
			Data: map[string]any{
				"thumbnail_path": smPath,
			},
		})
	}

	return nil
}

func (s *SceneService) SetThumbnailFromUpload(sceneID uint, file *multipart.FileHeader) error {
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedImageExtensions[ext] {
		return apperrors.ErrInvalidImageExtension
	}

	scene, err := s.Repo.GetByID(sceneID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrSceneNotFound(sceneID)
		}
		return apperrors.NewInternalError("failed to get scene", err)
	}

	if scene.Width == 0 || scene.Height == 0 {
		return apperrors.ErrSceneDimensionsNotAvailable
	}

	// Save uploaded file to temp location
	src, err := file.Open()
	if err != nil {
		return fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer src.Close()

	tmpFile, err := os.CreateTemp("", "goonhub-thumb-*"+ext)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpFile, src); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to save uploaded file: %w", err)
	}
	tmpFile.Close()

	return s.processAndSaveThumbnail(sceneID, scene, tmpPath)
}

// SetThumbnailFromURL downloads an image from a URL and sets it as the scene thumbnail.
func (s *SceneService) SetThumbnailFromURL(sceneID uint, imageURL string) error {
	scene, err := s.Repo.GetByID(sceneID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrSceneNotFound(sceneID)
		}
		return apperrors.NewInternalError("failed to get scene", err)
	}

	if scene.Width == 0 || scene.Height == 0 {
		return apperrors.ErrSceneDimensionsNotAvailable
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(imageURL)
	if err != nil {
		return fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download image: HTTP %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "goonhub-thumb-url-*.jpg")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to save downloaded image: %w", err)
	}
	tmpFile.Close()

	return s.processAndSaveThumbnail(sceneID, scene, tmpPath)
}

// processAndSaveThumbnail resizes an image file to sm/lg WebP thumbnails and updates the database.
func (s *SceneService) processAndSaveThumbnail(sceneID uint, scene *data.Scene, srcPath string) error {
	qualityConfig := s.ProcessingService.GetProcessingQualityConfig()

	tileWidthSm, tileHeightSm := ffmpeg.CalculateTileDimensions(scene.Width, scene.Height, qualityConfig.MaxFrameDimensionSm)
	tileWidthLg, tileHeightLg := ffmpeg.CalculateTileDimensions(scene.Width, scene.Height, qualityConfig.MaxFrameDimensionLg)

	thumbnailDir := filepath.Join(s.MetadataPath, "thumbnails")
	if err := os.MkdirAll(thumbnailDir, 0755); err != nil {
		return fmt.Errorf("failed to create thumbnail directory: %w", err)
	}

	smPath := filepath.Join(thumbnailDir, fmt.Sprintf("%d_thumb_sm.webp", sceneID))
	lgPath := filepath.Join(thumbnailDir, fmt.Sprintf("%d_thumb_lg.webp", sceneID))

	if err := ffmpeg.ResizeImageToWebp(srcPath, smPath, tileWidthSm, tileHeightSm, qualityConfig.FrameQualitySm); err != nil {
		return fmt.Errorf("failed to resize to small thumbnail: %w", err)
	}

	if err := ffmpeg.ResizeImageToWebp(srcPath, lgPath, tileWidthLg, tileHeightLg, qualityConfig.FrameQualityLg); err != nil {
		return fmt.Errorf("failed to resize to large thumbnail: %w", err)
	}

	if err := s.Repo.UpdateThumbnail(sceneID, smPath, tileWidthSm, tileHeightSm); err != nil {
		return fmt.Errorf("failed to update thumbnail in database: %w", err)
	}

	if s.EventBus != nil {
		s.EventBus.Publish(SceneEvent{
			Type:    "scene:thumbnail_complete",
			SceneID: sceneID,
			Data: map[string]any{
				"thumbnail_path": smPath,
			},
		})
	}

	return nil
}
