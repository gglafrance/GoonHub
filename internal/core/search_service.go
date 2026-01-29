package core

import (
	"fmt"

	"go.uber.org/zap"

	"goonhub/internal/data"
	"goonhub/internal/infrastructure/meilisearch"
)

// VideoIndexer defines the interface for video search indexing operations.
// This interface allows services to update the search index without depending
// directly on SearchService, enabling better testability.
type VideoIndexer interface {
	IndexVideo(video *data.Video) error
	UpdateVideoIndex(video *data.Video) error
	BulkUpdateVideoIndex(videos []data.Video) error
	DeleteVideoIndex(id uint) error
}

// SearchService orchestrates search operations using Meilisearch.
// User-specific filters (liked, rating, jizz_count) are handled by pre-querying
// PostgreSQL for matching video IDs, then passing those as filters to Meilisearch.
type SearchService struct {
	meiliClient     *meilisearch.Client
	videoRepo       data.VideoRepository
	interactionRepo data.InteractionRepository
	tagRepo         data.TagRepository
	logger          *zap.Logger
}

// NewSearchService creates a new SearchService.
func NewSearchService(
	meiliClient *meilisearch.Client,
	videoRepo data.VideoRepository,
	interactionRepo data.InteractionRepository,
	tagRepo data.TagRepository,
	logger *zap.Logger,
) *SearchService {
	return &SearchService{
		meiliClient:     meiliClient,
		videoRepo:       videoRepo,
		interactionRepo: interactionRepo,
		tagRepo:         tagRepo,
		logger:          logger,
	}
}

// Search performs a search for videos using Meilisearch.
func (s *SearchService) Search(params data.VideoSearchParams) ([]data.Video, int64, error) {
	if s.meiliClient == nil {
		return nil, 0, fmt.Errorf("meilisearch is not configured")
	}

	// Start with VideoIDs pre-filter if provided (e.g., folder search)
	var preFilteredIDs []uint
	if len(params.VideoIDs) > 0 {
		preFilteredIDs = params.VideoIDs
	}

	// Handle user-specific filters by pre-querying PostgreSQL for video IDs
	if s.hasUserFilters(params) {
		ids, err := s.getUserFilteredIDs(params)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to get user-filtered IDs: %w", err)
		}
		// If user filters are active but no videos match, return empty result
		if len(ids) == 0 {
			return []data.Video{}, 0, nil
		}
		// Intersect with folder pre-filter if present
		if len(preFilteredIDs) > 0 {
			preFilteredIDs = intersect(preFilteredIDs, ids)
			if len(preFilteredIDs) == 0 {
				return []data.Video{}, 0, nil
			}
		} else {
			preFilteredIDs = ids
		}
	}

	// Build Meilisearch search params
	meiliParams := s.buildMeiliParams(params, preFilteredIDs)

	// Perform Meilisearch search
	result, err := s.meiliClient.Search(meiliParams)
	if err != nil {
		return nil, 0, fmt.Errorf("meilisearch search failed: %w", err)
	}

	// If no results, return empty
	if len(result.IDs) == 0 {
		return []data.Video{}, result.TotalCount, nil
	}

	// Fetch full video records from PostgreSQL
	videos, err := s.videoRepo.GetByIDs(result.IDs)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch videos by IDs: %w", err)
	}

	return videos, result.TotalCount, nil
}

// hasUserFilters returns true if the params include user-specific filters.
func (s *SearchService) hasUserFilters(params data.VideoSearchParams) bool {
	if params.UserID == 0 {
		return false
	}
	return (params.Liked != nil && *params.Liked) ||
		params.MinRating > 0 || params.MaxRating > 0 ||
		params.MinJizzCount > 0 || params.MaxJizzCount > 0
}

// getUserFilteredIDs queries PostgreSQL for video IDs matching user-specific filters.
func (s *SearchService) getUserFilteredIDs(params data.VideoSearchParams) ([]uint, error) {
	var result []uint
	filterCount := 0

	if params.Liked != nil && *params.Liked {
		filterCount++
	}
	if params.MinRating > 0 || params.MaxRating > 0 {
		filterCount++
	}
	if params.MinJizzCount > 0 || params.MaxJizzCount > 0 {
		filterCount++
	}
	needsIntersection := filterCount > 1

	// Get liked video IDs
	if params.Liked != nil && *params.Liked {
		ids, err := s.interactionRepo.GetLikedVideoIDs(params.UserID)
		if err != nil {
			return nil, fmt.Errorf("failed to get liked video IDs: %w", err)
		}
		if needsIntersection && result == nil {
			result = ids
		} else if needsIntersection {
			result = intersect(result, ids)
		} else {
			return ids, nil
		}
	}

	// Get rated video IDs
	if params.MinRating > 0 || params.MaxRating > 0 {
		ids, err := s.interactionRepo.GetRatedVideoIDs(params.UserID, params.MinRating, params.MaxRating)
		if err != nil {
			return nil, fmt.Errorf("failed to get rated video IDs: %w", err)
		}
		if needsIntersection && result == nil {
			result = ids
		} else if needsIntersection {
			result = intersect(result, ids)
		} else {
			return ids, nil
		}
	}

	// Get jizzed video IDs
	if params.MinJizzCount > 0 || params.MaxJizzCount > 0 {
		ids, err := s.interactionRepo.GetJizzedVideoIDs(params.UserID, params.MinJizzCount, params.MaxJizzCount)
		if err != nil {
			return nil, fmt.Errorf("failed to get jizzed video IDs: %w", err)
		}
		if needsIntersection && result == nil {
			result = ids
		} else if needsIntersection {
			result = intersect(result, ids)
		} else {
			return ids, nil
		}
	}

	return result, nil
}

// buildMeiliParams converts VideoSearchParams to Meilisearch SearchParams.
func (s *SearchService) buildMeiliParams(params data.VideoSearchParams, preFilteredIDs []uint) meilisearch.SearchParams {
	meiliParams := meilisearch.SearchParams{
		Query:    params.Query,
		TagIDs:   params.TagIDs,
		Actors:   params.Actors,
		Studio:   params.Studio,
		VideoIDs: preFilteredIDs,
		Offset:   (params.Page - 1) * params.Limit,
		Limit:    params.Limit,
	}

	if params.MinDuration > 0 {
		minDur := float64(params.MinDuration)
		meiliParams.MinDuration = &minDur
	}
	if params.MaxDuration > 0 {
		maxDur := float64(params.MaxDuration)
		meiliParams.MaxDuration = &maxDur
	}
	if params.MinHeight > 0 {
		meiliParams.MinHeight = &params.MinHeight
	}
	if params.MaxHeight > 0 {
		meiliParams.MaxHeight = &params.MaxHeight
	}
	if params.MinDate != nil {
		ts := params.MinDate.Unix()
		meiliParams.DateAfter = &ts
	}
	if params.MaxDate != nil {
		ts := params.MaxDate.Unix()
		meiliParams.DateBefore = &ts
	}

	// Sort mapping
	switch params.Sort {
	case "relevance":
		meiliParams.Sort = ""
	case "title_asc":
		meiliParams.Sort = "title"
		meiliParams.SortDir = "asc"
	case "title_desc":
		meiliParams.Sort = "title"
		meiliParams.SortDir = "desc"
	case "duration_asc":
		meiliParams.Sort = "duration"
		meiliParams.SortDir = "asc"
	case "duration_desc":
		meiliParams.Sort = "duration"
		meiliParams.SortDir = "desc"
	case "created_at_asc":
		meiliParams.Sort = "created_at"
		meiliParams.SortDir = "asc"
	default:
		meiliParams.Sort = "created_at"
		meiliParams.SortDir = "desc"
	}

	return meiliParams
}

// IndexVideo adds or updates a video in the Meilisearch index.
func (s *SearchService) IndexVideo(video *data.Video) error {
	if s.meiliClient == nil {
		return nil
	}

	tags, err := s.tagRepo.GetVideoTags(video.ID)
	if err != nil {
		s.logger.Warn("failed to get video tags for indexing", zap.Uint("video_id", video.ID), zap.Error(err))
	}

	tagIDs := make([]uint, len(tags))
	tagNames := make([]string, len(tags))
	for i, tag := range tags {
		tagIDs[i] = tag.ID
		tagNames[i] = tag.Name
	}

	doc := meilisearch.VideoDocument{
		ID:               video.ID,
		Title:            video.Title,
		OriginalFilename: video.OriginalFilename,
		Description:      video.Description,
		Studio:           video.Studio,
		Actors:           video.Actors,
		TagIDs:           tagIDs,
		TagNames:         tagNames,
		Duration:         float64(video.Duration),
		Height:           video.Height,
		CreatedAt:        video.CreatedAt.Unix(),
		ProcessingStatus: video.ProcessingStatus,
	}

	return s.meiliClient.IndexVideo(doc)
}

// UpdateVideoIndex updates a video in the Meilisearch index.
func (s *SearchService) UpdateVideoIndex(video *data.Video) error {
	return s.IndexVideo(video)
}

// BulkUpdateVideoIndex updates multiple videos in the Meilisearch index efficiently.
func (s *SearchService) BulkUpdateVideoIndex(videos []data.Video) error {
	if s.meiliClient == nil || len(videos) == 0 {
		return nil
	}

	// Get video IDs
	videoIDs := make([]uint, len(videos))
	for i, v := range videos {
		videoIDs[i] = v.ID
	}

	// Fetch all tags for all videos in a single query
	tagsByVideo, err := s.tagRepo.GetVideoTagsMultiple(videoIDs)
	if err != nil {
		s.logger.Warn("failed to get video tags for bulk indexing", zap.Error(err))
		tagsByVideo = make(map[uint][]data.Tag)
	}

	// Build documents
	docs := make([]meilisearch.VideoDocument, len(videos))
	for i, video := range videos {
		tags := tagsByVideo[video.ID]
		tagIDs := make([]uint, len(tags))
		tagNames := make([]string, len(tags))
		for j, tag := range tags {
			tagIDs[j] = tag.ID
			tagNames[j] = tag.Name
		}

		docs[i] = meilisearch.VideoDocument{
			ID:               video.ID,
			Title:            video.Title,
			OriginalFilename: video.OriginalFilename,
			Description:      video.Description,
			Studio:           video.Studio,
			Actors:           video.Actors,
			TagIDs:           tagIDs,
			TagNames:         tagNames,
			Duration:         float64(video.Duration),
			Height:           video.Height,
			CreatedAt:        video.CreatedAt.Unix(),
			ProcessingStatus: video.ProcessingStatus,
		}
	}

	// Bulk index
	return s.meiliClient.BulkIndex(docs)
}

// DeleteVideoIndex removes a video from the Meilisearch index.
func (s *SearchService) DeleteVideoIndex(id uint) error {
	if s.meiliClient == nil {
		return nil
	}
	return s.meiliClient.DeleteVideo(id)
}

// ReindexAll rebuilds the entire Meilisearch index from PostgreSQL.
func (s *SearchService) ReindexAll() error {
	if s.meiliClient == nil {
		return fmt.Errorf("meilisearch is not configured")
	}

	s.logger.Info("starting full reindex")

	if err := s.meiliClient.ClearIndex(); err != nil {
		return fmt.Errorf("failed to clear index: %w", err)
	}

	videos, err := s.videoRepo.GetAll()
	if err != nil {
		return fmt.Errorf("failed to get all videos: %w", err)
	}

	batchSize := 100
	for i := 0; i < len(videos); i += batchSize {
		end := i + batchSize
		if end > len(videos) {
			end = len(videos)
		}
		batch := videos[i:end]

		docs := make([]meilisearch.VideoDocument, 0, len(batch))
		for _, video := range batch {
			tags, err := s.tagRepo.GetVideoTags(video.ID)
			if err != nil {
				s.logger.Warn("failed to get video tags for reindexing", zap.Uint("video_id", video.ID), zap.Error(err))
			}

			tagIDs := make([]uint, len(tags))
			tagNames := make([]string, len(tags))
			for j, tag := range tags {
				tagIDs[j] = tag.ID
				tagNames[j] = tag.Name
			}

			docs = append(docs, meilisearch.VideoDocument{
				ID:               video.ID,
				Title:            video.Title,
				OriginalFilename: video.OriginalFilename,
				Description:      video.Description,
				Studio:           video.Studio,
				Actors:           video.Actors,
				TagIDs:           tagIDs,
				TagNames:         tagNames,
				Duration:         float64(video.Duration),
				Height:           video.Height,
				CreatedAt:        video.CreatedAt.Unix(),
				ProcessingStatus: video.ProcessingStatus,
			})
		}

		if err := s.meiliClient.BulkIndex(docs); err != nil {
			return fmt.Errorf("failed to bulk index batch: %w", err)
		}

		s.logger.Info("reindexed batch", zap.Int("start", i), zap.Int("end", end), zap.Int("total", len(videos)))
	}

	s.logger.Info("full reindex completed", zap.Int("total_videos", len(videos)))
	return nil
}

// IsAvailable returns true if Meilisearch is configured and healthy.
func (s *SearchService) IsAvailable() bool {
	if s.meiliClient == nil {
		return false
	}
	return s.meiliClient.Health() == nil
}

// intersect returns the intersection of two slices of uint.
func intersect(a, b []uint) []uint {
	if len(a) == 0 || len(b) == 0 {
		return []uint{}
	}

	set := make(map[uint]struct{}, len(b))
	for _, id := range b {
		set[id] = struct{}{}
	}

	result := make([]uint, 0)
	for _, id := range a {
		if _, ok := set[id]; ok {
			result = append(result, id)
		}
	}

	return result
}
