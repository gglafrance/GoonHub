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

// ExplorerService provides folder-based video browsing and bulk editing
type ExplorerService struct {
	explorerRepo    data.ExplorerRepository
	storagePathRepo data.StoragePathRepository
	videoRepo       data.VideoRepository
	tagRepo         data.TagRepository
	actorRepo       data.ActorRepository
	eventBus        *EventBus
	logger          *zap.Logger
	indexer         VideoIndexer
	metadataPath    string
	searchService   *SearchService
}

// NewExplorerService creates a new ExplorerService
func NewExplorerService(
	explorerRepo data.ExplorerRepository,
	storagePathRepo data.StoragePathRepository,
	videoRepo data.VideoRepository,
	tagRepo data.TagRepository,
	actorRepo data.ActorRepository,
	eventBus *EventBus,
	logger *zap.Logger,
	metadataPath string,
) *ExplorerService {
	return &ExplorerService{
		explorerRepo:    explorerRepo,
		storagePathRepo: storagePathRepo,
		videoRepo:       videoRepo,
		tagRepo:         tagRepo,
		actorRepo:       actorRepo,
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

// SetIndexer sets the video indexer for search index updates
func (s *ExplorerService) SetIndexer(indexer VideoIndexer) {
	s.indexer = indexer
}

// FolderContentsResponse contains the contents of a folder
type FolderContentsResponse struct {
	StoragePath *data.StoragePath `json:"storage_path"`
	CurrentPath string            `json:"current_path"`
	Subfolders  []data.FolderInfo `json:"subfolders"`
	Videos      []data.Video      `json:"videos"`
	TotalVideos int64             `json:"total_videos"`
	Page        int               `json:"page"`
	Limit       int               `json:"limit"`
}

// GetStoragePathsWithCounts returns all storage paths with their video counts
func (s *ExplorerService) GetStoragePathsWithCounts() ([]data.StoragePathWithCount, error) {
	paths, err := s.explorerRepo.GetStoragePathsWithCounts()
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get storage paths", err)
	}
	return paths, nil
}

// GetFolderContents returns the contents of a folder (subfolders and videos)
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

	// Get videos in this folder (direct children only)
	videos, total, err := s.explorerRepo.GetVideosByFolder(storagePathID, folderPath, page, limit)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get videos", err)
	}

	return &FolderContentsResponse{
		StoragePath: storagePath,
		CurrentPath: folderPath,
		Subfolders:  subfolders,
		Videos:      videos,
		TotalVideos: total,
		Page:        page,
		Limit:       limit,
	}, nil
}

// GetFolderVideoIDs returns all video IDs in a folder
func (s *ExplorerService) GetFolderVideoIDs(storagePathID uint, folderPath string, recursive bool) ([]uint, error) {
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

	ids, err := s.explorerRepo.GetVideoIDsByFolder(storagePathID, folderPath, recursive)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get video IDs", err)
	}

	return ids, nil
}

// BulkUpdateTagsRequest represents a request to bulk update tags
type BulkUpdateTagsRequest struct {
	VideoIDs []uint `json:"video_ids"`
	TagIDs   []uint `json:"tag_ids"`
	Mode     string `json:"mode"` // "add", "remove", "replace"
}

// BulkUpdateTags updates tags for multiple videos using batch operations
func (s *ExplorerService) BulkUpdateTags(req BulkUpdateTagsRequest) (int, error) {
	if len(req.VideoIDs) == 0 {
		return 0, apperrors.NewValidationError("at least one video ID is required")
	}

	if req.Mode != "add" && req.Mode != "remove" && req.Mode != "replace" {
		return 0, apperrors.NewValidationError("mode must be 'add', 'remove', or 'replace'")
	}

	// Verify all videos exist
	videos, err := s.videoRepo.GetByIDs(req.VideoIDs)
	if err != nil {
		return 0, apperrors.NewInternalError("failed to verify videos", err)
	}
	if len(videos) != len(req.VideoIDs) {
		return 0, apperrors.NewValidationError("one or more videos not found")
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
		if err := s.tagRepo.BulkAddTagsToVideos(req.VideoIDs, req.TagIDs); err != nil {
			return 0, apperrors.NewInternalError("failed to add tags", err)
		}
	case "remove":
		if err := s.tagRepo.BulkRemoveTagsFromVideos(req.VideoIDs, req.TagIDs); err != nil {
			return 0, apperrors.NewInternalError("failed to remove tags", err)
		}
	case "replace":
		if err := s.tagRepo.BulkReplaceTagsForVideos(req.VideoIDs, req.TagIDs); err != nil {
			return 0, apperrors.NewInternalError("failed to replace tags", err)
		}
	}

	// Batch update search index
	if s.indexer != nil {
		// Refresh videos with updated associations
		updatedVideos, err := s.videoRepo.GetByIDs(req.VideoIDs)
		if err != nil {
			s.logger.Warn("Failed to fetch videos for index update", zap.Error(err))
		} else if err := s.indexer.BulkUpdateVideoIndex(updatedVideos); err != nil {
			s.logger.Warn("Failed to bulk update search index", zap.Error(err))
		}
	}

	// Emit single bulk update event
	if s.eventBus != nil {
		s.eventBus.Publish(VideoEvent{
			Type:    "videos_bulk_updated",
			VideoID: 0, // Bulk operation
		})
	}

	s.logger.Info("Bulk tag update completed",
		zap.Int("updated", len(req.VideoIDs)),
		zap.Int("total", len(req.VideoIDs)),
		zap.String("mode", req.Mode),
	)

	return len(req.VideoIDs), nil
}

// BulkUpdateActorsRequest represents a request to bulk update actors
type BulkUpdateActorsRequest struct {
	VideoIDs []uint `json:"video_ids"`
	ActorIDs []uint `json:"actor_ids"`
	Mode     string `json:"mode"` // "add", "remove", "replace"
}

// BulkUpdateActors updates actors for multiple videos using batch operations
func (s *ExplorerService) BulkUpdateActors(req BulkUpdateActorsRequest) (int, error) {
	if len(req.VideoIDs) == 0 {
		return 0, apperrors.NewValidationError("at least one video ID is required")
	}

	if req.Mode != "add" && req.Mode != "remove" && req.Mode != "replace" {
		return 0, apperrors.NewValidationError("mode must be 'add', 'remove', or 'replace'")
	}

	// Verify all videos exist
	videos, err := s.videoRepo.GetByIDs(req.VideoIDs)
	if err != nil {
		return 0, apperrors.NewInternalError("failed to verify videos", err)
	}
	if len(videos) != len(req.VideoIDs) {
		return 0, apperrors.NewValidationError("one or more videos not found")
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
		if err := s.actorRepo.BulkAddActorsToVideos(req.VideoIDs, req.ActorIDs); err != nil {
			return 0, apperrors.NewInternalError("failed to add actors", err)
		}
	case "remove":
		if err := s.actorRepo.BulkRemoveActorsFromVideos(req.VideoIDs, req.ActorIDs); err != nil {
			return 0, apperrors.NewInternalError("failed to remove actors", err)
		}
	case "replace":
		if err := s.actorRepo.BulkReplaceActorsForVideos(req.VideoIDs, req.ActorIDs); err != nil {
			return 0, apperrors.NewInternalError("failed to replace actors", err)
		}
	}

	// Batch update search index
	if s.indexer != nil {
		// Refresh videos with updated associations
		updatedVideos, err := s.videoRepo.GetByIDs(req.VideoIDs)
		if err != nil {
			s.logger.Warn("Failed to fetch videos for index update", zap.Error(err))
		} else if err := s.indexer.BulkUpdateVideoIndex(updatedVideos); err != nil {
			s.logger.Warn("Failed to bulk update search index", zap.Error(err))
		}
	}

	// Emit single bulk update event
	if s.eventBus != nil {
		s.eventBus.Publish(VideoEvent{
			Type:    "videos_bulk_updated",
			VideoID: 0, // Bulk operation
		})
	}

	s.logger.Info("Bulk actor update completed",
		zap.Int("updated", len(req.VideoIDs)),
		zap.Int("total", len(req.VideoIDs)),
		zap.String("mode", req.Mode),
	)

	return len(req.VideoIDs), nil
}


// BulkUpdateStudioRequest represents a request to bulk update studio
type BulkUpdateStudioRequest struct {
	VideoIDs []uint `json:"video_ids"`
	Studio   string `json:"studio"`
}

// BulkUpdateStudio updates studio for multiple videos using batch operations
func (s *ExplorerService) BulkUpdateStudio(req BulkUpdateStudioRequest) (int, error) {
	if len(req.VideoIDs) == 0 {
		return 0, apperrors.NewValidationError("at least one video ID is required")
	}

	// Verify all videos exist
	videos, err := s.videoRepo.GetByIDs(req.VideoIDs)
	if err != nil {
		return 0, apperrors.NewInternalError("failed to verify videos", err)
	}
	if len(videos) != len(req.VideoIDs) {
		return 0, apperrors.NewValidationError("one or more videos not found")
	}

	// Perform bulk update
	if err := s.videoRepo.BulkUpdateStudio(req.VideoIDs, req.Studio); err != nil {
		return 0, apperrors.NewInternalError("failed to update studio", err)
	}

	// Batch update search index
	if s.indexer != nil {
		// Refresh videos with updated studio
		updatedVideos, err := s.videoRepo.GetByIDs(req.VideoIDs)
		if err != nil {
			s.logger.Warn("Failed to fetch videos for index update", zap.Error(err))
		} else if err := s.indexer.BulkUpdateVideoIndex(updatedVideos); err != nil {
			s.logger.Warn("Failed to bulk update search index", zap.Error(err))
		}
	}

	// Emit single bulk update event
	if s.eventBus != nil {
		s.eventBus.Publish(VideoEvent{
			Type:    "videos_bulk_updated",
			VideoID: 0, // Bulk operation
		})
	}

	s.logger.Info("Bulk studio update completed",
		zap.Int("updated", len(req.VideoIDs)),
		zap.Int("total", len(req.VideoIDs)),
		zap.String("studio", req.Studio),
	)

	return len(req.VideoIDs), nil
}

// BulkDeleteVideos deletes multiple videos and their associated files
func (s *ExplorerService) BulkDeleteVideos(videoIDs []uint) (int, error) {
	if len(videoIDs) == 0 {
		return 0, apperrors.NewValidationError("at least one video ID is required")
	}

	// Verify videos exist
	videos, err := s.videoRepo.GetByIDs(videoIDs)
	if err != nil {
		return 0, apperrors.NewInternalError("failed to verify videos", err)
	}

	deleted := 0
	for _, video := range videos {
		// Delete from database (soft delete)
		if err := s.videoRepo.Delete(video.ID); err != nil {
			s.logger.Warn("Failed to delete video from database",
				zap.Uint("id", video.ID),
				zap.Error(err),
			)
			continue
		}

		// Remove from search index
		if s.indexer != nil {
			if err := s.indexer.DeleteVideoIndex(video.ID); err != nil {
				s.logger.Warn("Failed to delete video from search index",
					zap.Uint("id", video.ID),
					zap.Error(err),
				)
			}
		}

		// Delete physical files
		s.deleteVideoFiles(&video)
		deleted++
	}

	// Emit bulk delete event
	if s.eventBus != nil {
		s.eventBus.Publish(VideoEvent{
			Type:    "videos_bulk_deleted",
			VideoID: 0, // Bulk operation
		})
	}

	s.logger.Info("Bulk delete completed",
		zap.Int("deleted", deleted),
		zap.Int("requested", len(videoIDs)),
	)

	return deleted, nil
}

// deleteVideoFiles removes all physical files associated with a video
func (s *ExplorerService) deleteVideoFiles(video *data.Video) {
	// Remove video file
	if video.StoredPath != "" {
		if err := os.Remove(video.StoredPath); err != nil && !os.IsNotExist(err) {
			s.logger.Warn("Failed to delete video file",
				zap.Uint("id", video.ID),
				zap.String("path", video.StoredPath),
				zap.Error(err),
			)
		}
	}

	// Remove thumbnail
	if video.ThumbnailPath != "" {
		if err := os.Remove(video.ThumbnailPath); err != nil && !os.IsNotExist(err) {
			s.logger.Warn("Failed to delete thumbnail",
				zap.Uint("id", video.ID),
				zap.String("path", video.ThumbnailPath),
				zap.Error(err),
			)
		}
	}

	// Remove sprite sheets (pattern: {id}_sheet_*.jpg)
	if video.SpriteSheetPath != "" && s.metadataPath != "" {
		spriteDir := filepath.Join(s.metadataPath, "sprites")
		spritePattern := filepath.Join(spriteDir, fmt.Sprintf("%d_sheet_*.jpg", video.ID))
		files, _ := filepath.Glob(spritePattern)
		for _, file := range files {
			if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
				s.logger.Warn("Failed to delete sprite sheet",
					zap.Uint("id", video.ID),
					zap.String("path", file),
					zap.Error(err),
				)
			}
		}
	}

	// Remove VTT file
	if video.VttPath != "" {
		if err := os.Remove(video.VttPath); err != nil && !os.IsNotExist(err) {
			s.logger.Warn("Failed to delete VTT file",
				zap.Uint("id", video.ID),
				zap.String("path", video.VttPath),
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
	Page          int
	Limit         int
}

// FolderSearchResponse contains the search results
type FolderSearchResponse struct {
	Videos []data.Video `json:"videos"`
	Total  int64        `json:"total"`
	Page   int          `json:"page"`
	Limit  int          `json:"limit"`
}

// SearchInFolder searches for videos within a folder scope
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

	// Get all video IDs in the folder
	folderVideoIDs, err := s.explorerRepo.GetVideoIDsByFolder(req.StoragePathID, req.FolderPath, req.Recursive)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get folder video IDs", err)
	}

	// If folder is empty, return empty results
	if len(folderVideoIDs) == 0 {
		return &FolderSearchResponse{
			Videos: []data.Video{},
			Total:  0,
			Page:   req.Page,
			Limit:  req.Limit,
		}, nil
	}

	// Build search params with folder IDs as pre-filter
	searchParams := data.VideoSearchParams{
		Query:       req.Query,
		TagIDs:      req.TagIDs,
		Actors:      req.Actors,
		Studio:      req.Studio,
		MinDuration: req.MinDuration,
		MaxDuration: req.MaxDuration,
		Sort:        req.Sort,
		Page:        req.Page,
		Limit:       req.Limit,
		VideoIDs:    folderVideoIDs,
	}

	// Perform search
	videos, total, err := s.searchService.Search(searchParams)
	if err != nil {
		return nil, apperrors.NewInternalError("search failed", err)
	}

	return &FolderSearchResponse{
		Videos: videos,
		Total:  total,
		Page:   req.Page,
		Limit:  req.Limit,
	}, nil
}
