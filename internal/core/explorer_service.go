package core

import (
	"fmt"
	"os"
	"path/filepath"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ExplorerService provides folder-based scene browsing and bulk editing
type ExplorerService struct {
	explorerRepo    data.ExplorerRepository
	storagePathRepo data.StoragePathRepository
	sceneRepo       data.SceneRepository
	tagRepo         data.TagRepository
	actorRepo       data.ActorRepository
	jobHistoryRepo  data.JobHistoryRepository
	eventBus        *EventBus
	logger          *zap.Logger
	indexer         SceneIndexer
	metadataPath    string
	searchService   *SearchService
}

// NewExplorerService creates a new ExplorerService
func NewExplorerService(
	explorerRepo data.ExplorerRepository,
	storagePathRepo data.StoragePathRepository,
	sceneRepo data.SceneRepository,
	tagRepo data.TagRepository,
	actorRepo data.ActorRepository,
	jobHistoryRepo data.JobHistoryRepository,
	eventBus *EventBus,
	logger *zap.Logger,
	metadataPath string,
) *ExplorerService {
	return &ExplorerService{
		explorerRepo:    explorerRepo,
		storagePathRepo: storagePathRepo,
		sceneRepo:       sceneRepo,
		tagRepo:         tagRepo,
		actorRepo:       actorRepo,
		jobHistoryRepo:  jobHistoryRepo,
		eventBus:        eventBus,
		logger:          logger,
		metadataPath:    metadataPath,
	}
}

// SetSearchService sets the search service for folder search operations.
// This is called after service initialization to avoid circular dependencies.
func (s *ExplorerService) SetSearchService(searchService *SearchService) {
	s.searchService = searchService
}

// SetIndexer sets the scene indexer for search index updates
func (s *ExplorerService) SetIndexer(indexer SceneIndexer) {
	s.indexer = indexer
}

// FolderContentsResponse contains the contents of a folder
type FolderContentsResponse struct {
	StoragePath *data.StoragePath `json:"storage_path"`
	CurrentPath string            `json:"current_path"`
	Subfolders  []data.FolderInfo `json:"subfolders"`
	Scenes      []data.Scene      `json:"scenes"`
	TotalScenes int64             `json:"total_scenes"`
	Page        int               `json:"page"`
	Limit       int               `json:"limit"`
}

// GetStoragePathsWithCounts returns all storage paths with their scene counts
func (s *ExplorerService) GetStoragePathsWithCounts() ([]data.StoragePathWithCount, error) {
	paths, err := s.explorerRepo.GetStoragePathsWithCounts()
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get storage paths", err)
	}
	return paths, nil
}

// GetFolderContents returns the contents of a folder (subfolders and scenes)
func (s *ExplorerService) GetFolderContents(storagePathID uint, folderPath string, page, limit int) (*FolderContentsResponse, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 24
	}

	// Verify storage path exists
	storagePath, err := s.storagePathRepo.GetByID(storagePathID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperrors.NewNotFoundError("storage path", storagePathID)
		}
		return nil, apperrors.NewInternalError("failed to get storage path", err)
	}
	if storagePath == nil {
		return nil, apperrors.NewNotFoundError("storage path", storagePathID)
	}

	// Get subfolders
	subfolders, err := s.explorerRepo.GetSubfolders(storagePathID, folderPath)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get subfolders", err)
	}

	// Get scenes in this folder (direct children only)
	scenes, total, err := s.explorerRepo.GetScenesByFolder(storagePathID, folderPath, page, limit)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get scenes", err)
	}

	return &FolderContentsResponse{
		StoragePath: storagePath,
		CurrentPath: folderPath,
		Subfolders:  subfolders,
		Scenes:      scenes,
		TotalScenes: total,
		Page:        page,
		Limit:       limit,
	}, nil
}

// GetFolderSceneIDs returns all scene IDs in a folder
func (s *ExplorerService) GetFolderSceneIDs(storagePathID uint, folderPath string, recursive bool) ([]uint, error) {
	// Verify storage path exists
	storagePath, err := s.storagePathRepo.GetByID(storagePathID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperrors.NewNotFoundError("storage path", storagePathID)
		}
		return nil, apperrors.NewInternalError("failed to get storage path", err)
	}
	if storagePath == nil {
		return nil, apperrors.NewNotFoundError("storage path", storagePathID)
	}

	ids, err := s.explorerRepo.GetSceneIDsByFolder(storagePathID, folderPath, recursive)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get scene IDs", err)
	}

	return ids, nil
}

// BulkUpdateTagsRequest represents a request to bulk update tags
type BulkUpdateTagsRequest struct {
	SceneIDs []uint `json:"scene_ids"`
	TagIDs   []uint `json:"tag_ids"`
	Mode     string `json:"mode"` // "add", "remove", "replace"
}

// BulkUpdateTags updates tags for multiple scenes using batch operations
func (s *ExplorerService) BulkUpdateTags(req BulkUpdateTagsRequest) (int, error) {
	if len(req.SceneIDs) == 0 {
		return 0, apperrors.NewValidationError("at least one scene ID is required")
	}

	if req.Mode != "add" && req.Mode != "remove" && req.Mode != "replace" {
		return 0, apperrors.NewValidationError("mode must be 'add', 'remove', or 'replace'")
	}

	// Verify all scenes exist
	scenes, err := s.sceneRepo.GetByIDs(req.SceneIDs)
	if err != nil {
		return 0, apperrors.NewInternalError("failed to verify scenes", err)
	}
	if len(scenes) != len(req.SceneIDs) {
		return 0, apperrors.NewValidationError("one or more scenes not found")
	}

	// Verify tags exist for add/replace modes
	if (req.Mode == "add" || req.Mode == "replace") && len(req.TagIDs) > 0 {
		tags, err := s.tagRepo.GetByIDs(req.TagIDs)
		if err != nil {
			return 0, apperrors.NewInternalError("failed to verify tags", err)
		}
		if len(tags) != len(req.TagIDs) {
			return 0, apperrors.NewValidationError("one or more tags not found")
		}
	}

	// Perform bulk operation based on mode
	switch req.Mode {
	case "add":
		if err := s.tagRepo.BulkAddTagsToScenes(req.SceneIDs, req.TagIDs); err != nil {
			return 0, apperrors.NewInternalError("failed to add tags", err)
		}
	case "remove":
		if err := s.tagRepo.BulkRemoveTagsFromScenes(req.SceneIDs, req.TagIDs); err != nil {
			return 0, apperrors.NewInternalError("failed to remove tags", err)
		}
	case "replace":
		if err := s.tagRepo.BulkReplaceTagsForScenes(req.SceneIDs, req.TagIDs); err != nil {
			return 0, apperrors.NewInternalError("failed to replace tags", err)
		}
	}

	// Batch update search index
	if s.indexer != nil {
		// Refresh scenes with updated associations
		updatedScenes, err := s.sceneRepo.GetByIDs(req.SceneIDs)
		if err != nil {
			s.logger.Warn("Failed to fetch scenes for index update", zap.Error(err))
		} else if err := s.indexer.BulkUpdateSceneIndex(updatedScenes); err != nil {
			s.logger.Warn("Failed to bulk update search index", zap.Error(err))
		}
	}

	// Emit single bulk update event
	if s.eventBus != nil {
		s.eventBus.Publish(SceneEvent{
			Type:    "scenes_bulk_updated",
			SceneID: 0, // Bulk operation
		})
	}

	s.logger.Info("Bulk tag update completed",
		zap.Int("updated", len(req.SceneIDs)),
		zap.Int("total", len(req.SceneIDs)),
		zap.String("mode", req.Mode),
	)

	return len(req.SceneIDs), nil
}

// BulkUpdateActorsRequest represents a request to bulk update actors
type BulkUpdateActorsRequest struct {
	SceneIDs []uint `json:"scene_ids"`
	ActorIDs []uint `json:"actor_ids"`
	Mode     string `json:"mode"` // "add", "remove", "replace"
}

// BulkUpdateActors updates actors for multiple scenes using batch operations
func (s *ExplorerService) BulkUpdateActors(req BulkUpdateActorsRequest) (int, error) {
	if len(req.SceneIDs) == 0 {
		return 0, apperrors.NewValidationError("at least one scene ID is required")
	}

	if req.Mode != "add" && req.Mode != "remove" && req.Mode != "replace" {
		return 0, apperrors.NewValidationError("mode must be 'add', 'remove', or 'replace'")
	}

	// Verify all scenes exist
	scenes, err := s.sceneRepo.GetByIDs(req.SceneIDs)
	if err != nil {
		return 0, apperrors.NewInternalError("failed to verify scenes", err)
	}
	if len(scenes) != len(req.SceneIDs) {
		return 0, apperrors.NewValidationError("one or more scenes not found")
	}

	// Verify actors exist for add/replace modes
	if (req.Mode == "add" || req.Mode == "replace") && len(req.ActorIDs) > 0 {
		actors, err := s.actorRepo.GetByIDs(req.ActorIDs)
		if err != nil {
			return 0, apperrors.NewInternalError("failed to verify actors", err)
		}
		if len(actors) != len(req.ActorIDs) {
			return 0, apperrors.NewValidationError("one or more actors not found")
		}
	}

	// Perform bulk operation based on mode
	switch req.Mode {
	case "add":
		if err := s.actorRepo.BulkAddActorsToScenes(req.SceneIDs, req.ActorIDs); err != nil {
			return 0, apperrors.NewInternalError("failed to add actors", err)
		}
	case "remove":
		if err := s.actorRepo.BulkRemoveActorsFromScenes(req.SceneIDs, req.ActorIDs); err != nil {
			return 0, apperrors.NewInternalError("failed to remove actors", err)
		}
	case "replace":
		if err := s.actorRepo.BulkReplaceActorsForScenes(req.SceneIDs, req.ActorIDs); err != nil {
			return 0, apperrors.NewInternalError("failed to replace actors", err)
		}
	}

	// Batch update search index
	if s.indexer != nil {
		// Refresh scenes with updated associations
		updatedScenes, err := s.sceneRepo.GetByIDs(req.SceneIDs)
		if err != nil {
			s.logger.Warn("Failed to fetch scenes for index update", zap.Error(err))
		} else if err := s.indexer.BulkUpdateSceneIndex(updatedScenes); err != nil {
			s.logger.Warn("Failed to bulk update search index", zap.Error(err))
		}
	}

	// Emit single bulk update event
	if s.eventBus != nil {
		s.eventBus.Publish(SceneEvent{
			Type:    "scenes_bulk_updated",
			SceneID: 0, // Bulk operation
		})
	}

	s.logger.Info("Bulk actor update completed",
		zap.Int("updated", len(req.SceneIDs)),
		zap.Int("total", len(req.SceneIDs)),
		zap.String("mode", req.Mode),
	)

	return len(req.SceneIDs), nil
}


// BulkUpdateStudioRequest represents a request to bulk update studio
type BulkUpdateStudioRequest struct {
	SceneIDs []uint `json:"scene_ids"`
	Studio   string `json:"studio"`
}

// BulkUpdateStudio updates studio for multiple scenes using batch operations
func (s *ExplorerService) BulkUpdateStudio(req BulkUpdateStudioRequest) (int, error) {
	if len(req.SceneIDs) == 0 {
		return 0, apperrors.NewValidationError("at least one scene ID is required")
	}

	// Verify all scenes exist
	scenes, err := s.sceneRepo.GetByIDs(req.SceneIDs)
	if err != nil {
		return 0, apperrors.NewInternalError("failed to verify scenes", err)
	}
	if len(scenes) != len(req.SceneIDs) {
		return 0, apperrors.NewValidationError("one or more scenes not found")
	}

	// Perform bulk update
	if err := s.sceneRepo.BulkUpdateStudio(req.SceneIDs, req.Studio); err != nil {
		return 0, apperrors.NewInternalError("failed to update studio", err)
	}

	// Batch update search index
	if s.indexer != nil {
		// Refresh scenes with updated studio
		updatedScenes, err := s.sceneRepo.GetByIDs(req.SceneIDs)
		if err != nil {
			s.logger.Warn("Failed to fetch scenes for index update", zap.Error(err))
		} else if err := s.indexer.BulkUpdateSceneIndex(updatedScenes); err != nil {
			s.logger.Warn("Failed to bulk update search index", zap.Error(err))
		}
	}

	// Emit single bulk update event
	if s.eventBus != nil {
		s.eventBus.Publish(SceneEvent{
			Type:    "scenes_bulk_updated",
			SceneID: 0, // Bulk operation
		})
	}

	s.logger.Info("Bulk studio update completed",
		zap.Int("updated", len(req.SceneIDs)),
		zap.Int("total", len(req.SceneIDs)),
		zap.String("studio", req.Studio),
	)

	return len(req.SceneIDs), nil
}

// BulkDeleteScenes deletes multiple scenes.
// If permanent is false, scenes are moved to trash (files preserved).
// If permanent is true, scenes are hard deleted (files removed).
func (s *ExplorerService) BulkDeleteScenes(sceneIDs []uint, permanent bool) (int, error) {
	if len(sceneIDs) == 0 {
		return 0, apperrors.NewValidationError("at least one scene ID is required")
	}

	// Verify scenes exist
	scenes, err := s.sceneRepo.GetByIDs(sceneIDs)
	if err != nil {
		return 0, apperrors.NewInternalError("failed to verify scenes", err)
	}

	deleted := 0
	for _, scene := range scenes {
		// Cancel pending jobs for this scene
		if s.jobHistoryRepo != nil {
			if _, err := s.jobHistoryRepo.CancelPendingJobsForScene(scene.ID); err != nil {
				s.logger.Warn("Failed to cancel pending jobs for scene",
					zap.Uint("id", scene.ID),
					zap.Error(err),
				)
			}
		}

		if permanent {
			// Hard delete: remove from DB and delete files
			if _, err := s.sceneRepo.HardDelete(scene.ID); err != nil {
				s.logger.Warn("Failed to hard delete scene",
					zap.Uint("id", scene.ID),
					zap.Error(err),
				)
				continue
			}
			s.deleteSceneFiles(&scene)
		} else {
			// Soft delete: move to trash (files preserved)
			if _, err := s.sceneRepo.MoveToTrash(scene.ID); err != nil {
				s.logger.Warn("Failed to move scene to trash",
					zap.Uint("id", scene.ID),
					zap.Error(err),
				)
				continue
			}
		}

		// Remove from search index
		if s.indexer != nil {
			if err := s.indexer.DeleteSceneIndex(scene.ID); err != nil {
				s.logger.Warn("Failed to delete scene from search index",
					zap.Uint("id", scene.ID),
					zap.Error(err),
				)
			}
		}

		deleted++
	}

	// Emit appropriate event
	eventType := "scenes_bulk_trashed"
	if permanent {
		eventType = "scenes_bulk_deleted"
	}
	if s.eventBus != nil {
		s.eventBus.Publish(SceneEvent{
			Type:    eventType,
			SceneID: 0, // Bulk operation
		})
	}

	action := "trashed"
	if permanent {
		action = "deleted"
	}
	s.logger.Info("Bulk delete completed",
		zap.String("action", action),
		zap.Int("affected", deleted),
		zap.Int("requested", len(sceneIDs)),
	)

	return deleted, nil
}

// deleteSceneFiles removes all physical files associated with a scene
func (s *ExplorerService) deleteSceneFiles(scene *data.Scene) {
	// Remove scene file
	if scene.StoredPath != "" {
		if err := os.Remove(scene.StoredPath); err != nil && !os.IsNotExist(err) {
			s.logger.Warn("Failed to delete scene file",
				zap.Uint("id", scene.ID),
				zap.String("path", scene.StoredPath),
				zap.Error(err),
			)
		}
	}

	// Remove thumbnail
	if scene.ThumbnailPath != "" {
		if err := os.Remove(scene.ThumbnailPath); err != nil && !os.IsNotExist(err) {
			s.logger.Warn("Failed to delete thumbnail",
				zap.Uint("id", scene.ID),
				zap.String("path", scene.ThumbnailPath),
				zap.Error(err),
			)
		}
	}

	// Remove sprite sheets (pattern: {id}_sheet_*.jpg)
	if scene.SpriteSheetPath != "" && s.metadataPath != "" {
		spriteDir := filepath.Join(s.metadataPath, "sprites")
		spritePattern := filepath.Join(spriteDir, fmt.Sprintf("%d_sheet_*.jpg", scene.ID))
		files, _ := filepath.Glob(spritePattern)
		for _, file := range files {
			if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
				s.logger.Warn("Failed to delete sprite sheet",
					zap.Uint("id", scene.ID),
					zap.String("path", file),
					zap.Error(err),
				)
			}
		}
	}

	// Remove VTT file
	if scene.VttPath != "" {
		if err := os.Remove(scene.VttPath); err != nil && !os.IsNotExist(err) {
			s.logger.Warn("Failed to delete VTT file",
				zap.Uint("id", scene.ID),
				zap.String("path", scene.VttPath),
				zap.Error(err),
			)
		}
	}
}

// FolderSearchRequest represents a request to search within a folder
type FolderSearchRequest struct {
	StoragePathID uint
	FolderPath    string
	Recursive     bool
	Query         string
	TagIDs        []uint
	Actors        []string
	Studio        string
	MinDuration   int
	MaxDuration   int
	Sort          string
	HasPornDBID   *bool // nil = no filter, true = has, false = missing
	Page          int
	Limit         int
}

// FolderSearchResponse contains the search results
type FolderSearchResponse struct {
	Scenes []data.Scene `json:"scenes"`
	Total  int64        `json:"total"`
	Page   int          `json:"page"`
	Limit  int          `json:"limit"`
}

// SceneMatchInfo contains minimal data for bulk PornDB matching
type SceneMatchInfo struct {
	ID               uint     `json:"id"`
	Title            string   `json:"title"`
	OriginalFilename string   `json:"original_filename"`
	PornDBSceneID    *string  `json:"porndb_scene_id"`
	Actors           []string `json:"actors"`
	Studio           *string  `json:"studio"`
	ThumbnailPath    string   `json:"thumbnail_path"`
	Duration         int      `json:"duration"`
}

// GetScenesMatchInfo returns minimal scene data needed for bulk PornDB matching
func (s *ExplorerService) GetScenesMatchInfo(sceneIDs []uint) ([]SceneMatchInfo, error) {
	if len(sceneIDs) == 0 {
		return []SceneMatchInfo{}, nil
	}

	scenes, err := s.sceneRepo.GetByIDs(sceneIDs)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to fetch scenes", err)
	}

	// Build a map for fast lookup of actors by scene ID
	actorsByScene, err := s.actorRepo.GetSceneActorsMultiple(sceneIDs)
	if err != nil {
		s.logger.Warn("Failed to fetch actors for scenes", zap.Error(err))
		// Continue without actors
		actorsByScene = make(map[uint][]data.Actor)
	}

	result := make([]SceneMatchInfo, 0, len(scenes))
	for _, scene := range scenes {
		// Get actor names for this scene
		var actorNames []string
		if actors, ok := actorsByScene[scene.ID]; ok {
			actorNames = make([]string, len(actors))
			for i, actor := range actors {
				actorNames[i] = actor.Name
			}
		}

		// Handle nullable PornDBSceneID and Studio
		var porndbID *string
		if scene.PornDBSceneID != "" {
			porndbID = &scene.PornDBSceneID
		}

		var studio *string
		if scene.Studio != "" {
			studio = &scene.Studio
		}

		result = append(result, SceneMatchInfo{
			ID:               scene.ID,
			Title:            scene.Title,
			OriginalFilename: scene.OriginalFilename,
			PornDBSceneID:    porndbID,
			Actors:           actorNames,
			Studio:           studio,
			ThumbnailPath:    scene.ThumbnailPath,
			Duration:         scene.Duration,
		})
	}

	return result, nil
}

// SearchInFolder searches for scenes within a folder scope
func (s *ExplorerService) SearchInFolder(req FolderSearchRequest) (*FolderSearchResponse, error) {
	if s.searchService == nil {
		return nil, apperrors.NewInternalError("search service not available", nil)
	}

	// Validate pagination
	if req.Page < 1 {
		req.Page = 1
	}
	if req.Limit < 1 || req.Limit > 100 {
		req.Limit = 24
	}

	// Verify storage path exists
	storagePath, err := s.storagePathRepo.GetByID(req.StoragePathID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperrors.NewNotFoundError("storage path", req.StoragePathID)
		}
		return nil, apperrors.NewInternalError("failed to get storage path", err)
	}
	if storagePath == nil {
		return nil, apperrors.NewNotFoundError("storage path", req.StoragePathID)
	}

	// Get all scene IDs in the folder
	folderSceneIDs, err := s.explorerRepo.GetSceneIDsByFolder(req.StoragePathID, req.FolderPath, req.Recursive)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get folder scene IDs", err)
	}

	// If folder is empty, return empty results
	if len(folderSceneIDs) == 0 {
		return &FolderSearchResponse{
			Scenes: []data.Scene{},
			Total:  0,
			Page:   req.Page,
			Limit:  req.Limit,
		}, nil
	}

	// Build search params with folder IDs as pre-filter
	searchParams := data.SceneSearchParams{
		Query:       req.Query,
		TagIDs:      req.TagIDs,
		Actors:      req.Actors,
		Studio:      req.Studio,
		MinDuration: req.MinDuration,
		MaxDuration: req.MaxDuration,
		Sort:        req.Sort,
		HasPornDBID: req.HasPornDBID,
		Page:        req.Page,
		Limit:       req.Limit,
		SceneIDs:    folderSceneIDs,
	}

	// Perform search
	scenes, total, err := s.searchService.Search(searchParams)
	if err != nil {
		return nil, apperrors.NewInternalError("search failed", err)
	}

	return &FolderSearchResponse{
		Scenes: scenes,
		Total:  total,
		Page:   req.Page,
		Limit:  req.Limit,
	}, nil
}
