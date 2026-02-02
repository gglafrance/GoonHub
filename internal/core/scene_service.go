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
	jobHistoryRepo    data.JobHistoryRepository
	dlqRepo           data.DLQRepository
	appSettingsRepo   data.AppSettingsRepository
}

func NewSceneService(
	repo data.SceneRepository,
	scenePath string,
	metadataPath string,
	processingService *SceneProcessingService,
	eventBus *EventBus,
	logger *zap.Logger,
	jobHistoryRepo data.JobHistoryRepository,
	dlqRepo data.DLQRepository,
	appSettingsRepo data.AppSettingsRepository,
) *SceneService {
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
		jobHistoryRepo:    jobHistoryRepo,
		dlqRepo:           dlqRepo,
		appSettingsRepo:   appSettingsRepo,
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

// MoveSceneToTrash moves a scene to trash (soft delete with retention).
// Returns the expiry date based on retention settings.
func (s *SceneService) MoveSceneToTrash(id uint) (*time.Time, error) {
	// Verify scene exists (and is not already trashed)
	scene, err := s.Repo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrSceneNotFound(id)
		}
		return nil, apperrors.NewInternalError("failed to get scene", err)
	}

	// Cancel pending jobs for this scene
	if s.jobHistoryRepo != nil {
		cancelled, err := s.jobHistoryRepo.CancelPendingJobsForScene(id)
		if err != nil {
			s.logger.Warn("Failed to cancel pending jobs for trashed scene",
				zap.Uint("scene_id", id),
				zap.Error(err),
			)
		} else if cancelled > 0 {
			s.logger.Info("Cancelled pending jobs for trashed scene",
				zap.Uint("scene_id", id),
				zap.Int64("cancelled_count", cancelled),
			)
		}
	}

	// Move to trash
	trashedAt, err := s.Repo.MoveToTrash(id)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to move scene to trash", err)
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

	// Publish SSE event
	if s.EventBus != nil {
		s.EventBus.Publish(SceneEvent{
			Type:    "scene:trashed",
			SceneID: id,
			Data: map[string]any{
				"title":      scene.Title,
				"trashed_at": trashedAt,
			},
		})
	}

	// Calculate expiry date
	retentionDays := 7 // default
	if s.appSettingsRepo != nil {
		settings, err := s.appSettingsRepo.Get()
		if err == nil && settings != nil {
			retentionDays = settings.TrashRetentionDays
		}
	}
	expiresAt := trashedAt.AddDate(0, 0, retentionDays)

	return &expiresAt, nil
}

// RestoreSceneFromTrash restores a scene from trash.
func (s *SceneService) RestoreSceneFromTrash(id uint) error {
	// Verify scene exists and is trashed
	scene, err := s.Repo.GetByIDIncludingTrashed(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrSceneNotFound(id)
		}
		return apperrors.NewInternalError("failed to get scene", err)
	}

	if scene.TrashedAt == nil {
		return apperrors.NewValidationError("scene is not in trash")
	}

	// Restore from trash
	if err := s.Repo.RestoreFromTrash(id); err != nil {
		return apperrors.NewInternalError("failed to restore scene from trash", err)
	}

	// Re-index in search engine
	restoredScene, _ := s.Repo.GetByID(id)
	if s.indexer != nil && restoredScene != nil {
		if err := s.indexer.IndexScene(restoredScene); err != nil {
			s.logger.Warn("Failed to re-index restored scene",
				zap.Uint("scene_id", id),
				zap.Error(err),
			)
		}
	}

	// Publish SSE event
	if s.EventBus != nil {
		s.EventBus.Publish(SceneEvent{
			Type:    "scene:restored",
			SceneID: id,
			Data: map[string]any{
				"title": scene.Title,
			},
		})
	}

	return nil
}

// HardDeleteScene permanently deletes a scene and all associated files.
func (s *SceneService) HardDeleteScene(id uint) error {
	// Delete DLQ entries for this scene
	if s.dlqRepo != nil {
		if _, err := s.dlqRepo.DeleteBySceneID(id); err != nil {
			s.logger.Warn("Failed to delete DLQ entries for scene",
				zap.Uint("scene_id", id),
				zap.Error(err),
			)
		}
	}

	// Cancel any pending jobs
	if s.jobHistoryRepo != nil {
		if _, err := s.jobHistoryRepo.CancelPendingJobsForScene(id); err != nil {
			s.logger.Warn("Failed to cancel pending jobs for deleted scene",
				zap.Uint("scene_id", id),
				zap.Error(err),
			)
		}
	}

	// Hard delete from DB
	deletedScene, err := s.Repo.HardDelete(id)
	if err != nil {
		return apperrors.NewInternalError("failed to hard delete scene", err)
	}

	// Delete physical files (log warnings if missing, don't fail)
	s.deleteSceneFiles(deletedScene)

	// Remove from search index (in case it wasn't removed during trash)
	if s.indexer != nil {
		if err := s.indexer.DeleteSceneIndex(id); err != nil {
			s.logger.Warn("Failed to delete scene from search index",
				zap.Uint("scene_id", id),
				zap.Error(err),
			)
		}
	}

	// Publish SSE event
	if s.EventBus != nil {
		s.EventBus.Publish(SceneEvent{
			Type:    "scene:deleted",
			SceneID: id,
			Data: map[string]any{
				"title": deletedScene.Title,
			},
		})
	}

	return nil
}

// deleteSceneFiles deletes all files associated with a scene.
func (s *SceneService) deleteSceneFiles(scene *data.Scene) {
	// Delete video file
	if scene.StoredPath != "" {
		if err := os.Remove(scene.StoredPath); err != nil && !os.IsNotExist(err) {
			s.logger.Warn("Failed to delete video file",
				zap.Uint("scene_id", scene.ID),
				zap.String("path", scene.StoredPath),
				zap.Error(err),
			)
		}
	}

	// Delete thumbnails (sm and lg)
	thumbnailDir := filepath.Join(s.MetadataPath, "thumbnails")
	smPath := filepath.Join(thumbnailDir, fmt.Sprintf("%d_thumb_sm.webp", scene.ID))
	lgPath := filepath.Join(thumbnailDir, fmt.Sprintf("%d_thumb_lg.webp", scene.ID))
	os.Remove(smPath)
	os.Remove(lgPath)

	// Also try the old thumbnail path if different
	if scene.ThumbnailPath != "" && scene.ThumbnailPath != smPath {
		os.Remove(scene.ThumbnailPath)
	}

	// Delete sprite sheets
	spriteDir := filepath.Join(s.MetadataPath, "sprites")
	spritePattern := filepath.Join(spriteDir, fmt.Sprintf("%d_sheet_*.jpg", scene.ID))
	files, _ := filepath.Glob(spritePattern)
	for _, file := range files {
		os.Remove(file)
	}

	// Delete VTT file
	if scene.VttPath != "" {
		os.Remove(scene.VttPath)
	}
}

// ListTrashedScenes returns paginated list of trashed scenes.
func (s *SceneService) ListTrashedScenes(page, limit int) ([]data.Scene, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	return s.Repo.ListTrashed(page, limit)
}

// CountTrashedScenes returns the count of trashed scenes.
func (s *SceneService) CountTrashedScenes() (int64, error) {
	return s.Repo.CountTrashed()
}

// EmptyTrash permanently deletes all trashed scenes.
func (s *SceneService) EmptyTrash() (int, error) {
	scenes, _, err := s.Repo.ListTrashed(1, 10000) // Get all trashed scenes
	if err != nil {
		return 0, apperrors.NewInternalError("failed to list trashed scenes", err)
	}

	deleted := 0
	for _, scene := range scenes {
		if err := s.HardDeleteScene(scene.ID); err != nil {
			s.logger.Warn("Failed to hard delete scene during empty trash",
				zap.Uint("scene_id", scene.ID),
				zap.Error(err),
			)
			continue
		}
		deleted++
	}

	return deleted, nil
}

// GetTrashRetentionDays returns the current trash retention setting.
func (s *SceneService) GetTrashRetentionDays() int {
	if s.appSettingsRepo == nil {
		return 7
	}
	settings, err := s.appSettingsRepo.Get()
	if err != nil || settings == nil {
		return 7
	}
	return settings.TrashRetentionDays
}
