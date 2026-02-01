package core

import (
	"fmt"

	"go.uber.org/zap"

	"goonhub/internal/data"
	"goonhub/internal/infrastructure/meilisearch"
)

// SceneIndexer defines the interface for scene search indexing operations.
// This interface allows services to update the search index without depending
// directly on SearchService, enabling better testability.
type SceneIndexer interface {
	IndexScene(scene *data.Scene) error
	UpdateSceneIndex(scene *data.Scene) error
	BulkUpdateSceneIndex(scenes []data.Scene) error
	DeleteSceneIndex(id uint) error
}

// SearchService orchestrates search operations using Meilisearch.
// User-specific filters (liked, rating, jizz_count, marker_labels) are handled by pre-querying
// PostgreSQL for matching scene IDs, then passing those as filters to Meilisearch.
type SearchService struct {
	meiliClient     *meilisearch.Client
	sceneRepo       data.SceneRepository
	interactionRepo data.InteractionRepository
	tagRepo         data.TagRepository
	actorRepo       data.ActorRepository
	markerRepo      data.MarkerRepository
	logger          *zap.Logger
}

// NewSearchService creates a new SearchService.
func NewSearchService(
	meiliClient *meilisearch.Client,
	sceneRepo data.SceneRepository,
	interactionRepo data.InteractionRepository,
	tagRepo data.TagRepository,
	actorRepo data.ActorRepository,
	markerRepo data.MarkerRepository,
	logger *zap.Logger,
) *SearchService {
	return &SearchService{
		meiliClient:     meiliClient,
		sceneRepo:       sceneRepo,
		interactionRepo: interactionRepo,
		tagRepo:         tagRepo,
		actorRepo:       actorRepo,
		markerRepo:      markerRepo,
		logger:          logger,
	}
}

// Search performs a search for scenes using Meilisearch.
func (s *SearchService) Search(params data.SceneSearchParams) ([]data.Scene, int64, error) {
	if s.meiliClient == nil {
		return nil, 0, fmt.Errorf("meilisearch is not configured")
	}

	// Start with SceneIDs pre-filter if provided (e.g., folder search)
	var preFilteredIDs []uint
	if len(params.SceneIDs) > 0 {
		preFilteredIDs = params.SceneIDs
	}

	// Handle user-specific filters by pre-querying PostgreSQL for scene IDs
	if s.hasUserFilters(params) {
		ids, err := s.getUserFilteredIDs(params)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to get user-filtered IDs: %w", err)
		}
		// If user filters are active but no scenes match, return empty result
		if len(ids) == 0 {
			return []data.Scene{}, 0, nil
		}
		// Intersect with folder pre-filter if present
		if len(preFilteredIDs) > 0 {
			preFilteredIDs = intersect(preFilteredIDs, ids)
			if len(preFilteredIDs) == 0 {
				return []data.Scene{}, 0, nil
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
		return []data.Scene{}, result.TotalCount, nil
	}

	// Fetch full scene records from PostgreSQL
	scenes, err := s.sceneRepo.GetByIDs(result.IDs)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch scenes by IDs: %w", err)
	}

	return scenes, result.TotalCount, nil
}

// hasUserFilters returns true if the params include user-specific filters.
func (s *SearchService) hasUserFilters(params data.SceneSearchParams) bool {
	if params.UserID == 0 {
		return false
	}
	return (params.Liked != nil && *params.Liked) ||
		params.MinRating > 0 || params.MaxRating > 0 ||
		params.MinJizzCount > 0 || params.MaxJizzCount > 0 ||
		len(params.MarkerLabels) > 0
}

// getUserFilteredIDs queries PostgreSQL for scene IDs matching user-specific filters.
func (s *SearchService) getUserFilteredIDs(params data.SceneSearchParams) ([]uint, error) {
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
	if len(params.MarkerLabels) > 0 {
		filterCount++
	}
	needsIntersection := filterCount > 1

	// Get liked scene IDs
	if params.Liked != nil && *params.Liked {
		ids, err := s.interactionRepo.GetLikedSceneIDs(params.UserID)
		if err != nil {
			return nil, fmt.Errorf("failed to get liked scene IDs: %w", err)
		}
		if needsIntersection && result == nil {
			result = ids
		} else if needsIntersection {
			result = intersect(result, ids)
		} else {
			return ids, nil
		}
	}

	// Get rated scene IDs
	if params.MinRating > 0 || params.MaxRating > 0 {
		ids, err := s.interactionRepo.GetRatedSceneIDs(params.UserID, params.MinRating, params.MaxRating)
		if err != nil {
			return nil, fmt.Errorf("failed to get rated scene IDs: %w", err)
		}
		if needsIntersection && result == nil {
			result = ids
		} else if needsIntersection {
			result = intersect(result, ids)
		} else {
			return ids, nil
		}
	}

	// Get jizzed scene IDs
	if params.MinJizzCount > 0 || params.MaxJizzCount > 0 {
		ids, err := s.interactionRepo.GetJizzedSceneIDs(params.UserID, params.MinJizzCount, params.MaxJizzCount)
		if err != nil {
			return nil, fmt.Errorf("failed to get jizzed scene IDs: %w", err)
		}
		if needsIntersection && result == nil {
			result = ids
		} else if needsIntersection {
			result = intersect(result, ids)
		} else {
			return ids, nil
		}
	}

	// Get scene IDs with markers matching specified labels
	if len(params.MarkerLabels) > 0 {
		ids, err := s.markerRepo.GetSceneIDsByLabels(params.UserID, params.MarkerLabels)
		if err != nil {
			return nil, fmt.Errorf("failed to get marker scene IDs: %w", err)
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

// buildMeiliParams converts SceneSearchParams to Meilisearch SearchParams.
func (s *SearchService) buildMeiliParams(params data.SceneSearchParams, preFilteredIDs []uint) meilisearch.SearchParams {
	meiliParams := meilisearch.SearchParams{
		Query:            params.Query,
		TagIDs:           params.TagIDs,
		Actors:           params.Actors,
		Studio:           params.Studio,
		SceneIDs:         preFilteredIDs,
		Offset:           (params.Page - 1) * params.Limit,
		Limit:            params.Limit,
		MatchingStrategy: params.MatchingStrategy,
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
	case "view_count_desc":
		meiliParams.Sort = "view_count"
		meiliParams.SortDir = "desc"
	case "view_count_asc":
		meiliParams.Sort = "view_count"
		meiliParams.SortDir = "asc"
	default:
		meiliParams.Sort = "created_at"
		meiliParams.SortDir = "desc"
	}

	return meiliParams
}

// buildSceneDocument creates a Meilisearch document from a scene with its tags and actors.
func buildSceneDocument(scene *data.Scene, tags []data.Tag, actors []data.Actor) meilisearch.SceneDocument {
	tagIDs := make([]uint, len(tags))
	tagNames := make([]string, len(tags))
	for i, tag := range tags {
		tagIDs[i] = tag.ID
		tagNames[i] = tag.Name
	}

	actorNames := make([]string, len(actors))
	for i, actor := range actors {
		actorNames[i] = actor.Name
	}

	return meilisearch.SceneDocument{
		ID:               scene.ID,
		Title:            scene.Title,
		OriginalFilename: scene.OriginalFilename,
		Description:      scene.Description,
		Studio:           scene.Studio,
		Actors:           actorNames,
		TagIDs:           tagIDs,
		TagNames:         tagNames,
		Duration:         float64(scene.Duration),
		Height:           scene.Height,
		CreatedAt:        scene.CreatedAt.Unix(),
		ProcessingStatus: scene.ProcessingStatus,
		ViewCount:        int(scene.ViewCount),
	}
}

// IndexScene adds or updates a scene in the Meilisearch index.
func (s *SearchService) IndexScene(scene *data.Scene) error {
	if s.meiliClient == nil {
		return nil
	}

	tags, err := s.tagRepo.GetSceneTags(scene.ID)
	if err != nil {
		s.logger.Warn("failed to get scene tags for indexing", zap.Uint("scene_id", scene.ID), zap.Error(err))
	}

	actors, err := s.actorRepo.GetSceneActors(scene.ID)
	if err != nil {
		s.logger.Warn("failed to get scene actors for indexing", zap.Uint("scene_id", scene.ID), zap.Error(err))
	}

	return s.meiliClient.IndexScene(buildSceneDocument(scene, tags, actors))
}

// UpdateSceneIndex updates a scene in the Meilisearch index.
func (s *SearchService) UpdateSceneIndex(scene *data.Scene) error {
	return s.IndexScene(scene)
}

// BulkUpdateSceneIndex updates multiple scenes in the Meilisearch index efficiently.
func (s *SearchService) BulkUpdateSceneIndex(scenes []data.Scene) error {
	if s.meiliClient == nil || len(scenes) == 0 {
		return nil
	}

	// Get scene IDs
	sceneIDs := make([]uint, len(scenes))
	for i, v := range scenes {
		sceneIDs[i] = v.ID
	}

	// Fetch all tags for all scenes in a single query
	tagsByScene, err := s.tagRepo.GetSceneTagsMultiple(sceneIDs)
	if err != nil {
		s.logger.Warn("failed to get scene tags for bulk indexing", zap.Error(err))
		tagsByScene = make(map[uint][]data.Tag)
	}

	// Fetch all actors for all scenes in a single query
	actorsByScene, err := s.actorRepo.GetSceneActorsMultiple(sceneIDs)
	if err != nil {
		s.logger.Warn("failed to get scene actors for bulk indexing", zap.Error(err))
		actorsByScene = make(map[uint][]data.Actor)
	}

	// Build documents
	docs := make([]meilisearch.SceneDocument, len(scenes))
	for i, scene := range scenes {
		docs[i] = buildSceneDocument(&scene, tagsByScene[scene.ID], actorsByScene[scene.ID])
	}

	// Bulk index
	return s.meiliClient.BulkIndex(docs)
}

// DeleteSceneIndex removes a scene from the Meilisearch index.
func (s *SearchService) DeleteSceneIndex(id uint) error {
	if s.meiliClient == nil {
		return nil
	}
	return s.meiliClient.DeleteScene(id)
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

	scenes, err := s.sceneRepo.GetAll()
	if err != nil {
		return fmt.Errorf("failed to get all scenes: %w", err)
	}

	batchSize := 100
	for i := 0; i < len(scenes); i += batchSize {
		end := i + batchSize
		if end > len(scenes) {
			end = len(scenes)
		}
		batch := scenes[i:end]

		// Get scene IDs for this batch
		batchIDs := make([]uint, len(batch))
		for j, v := range batch {
			batchIDs[j] = v.ID
		}

		// Fetch all tags for this batch in a single query
		tagsByScene, err := s.tagRepo.GetSceneTagsMultiple(batchIDs)
		if err != nil {
			s.logger.Warn("failed to get scene tags for reindexing batch", zap.Error(err))
			tagsByScene = make(map[uint][]data.Tag)
		}

		// Fetch all actors for this batch in a single query
		actorsByScene, err := s.actorRepo.GetSceneActorsMultiple(batchIDs)
		if err != nil {
			s.logger.Warn("failed to get scene actors for reindexing batch", zap.Error(err))
			actorsByScene = make(map[uint][]data.Actor)
		}

		docs := make([]meilisearch.SceneDocument, len(batch))
		for i, scene := range batch {
			docs[i] = buildSceneDocument(&scene, tagsByScene[scene.ID], actorsByScene[scene.ID])
		}

		if err := s.meiliClient.BulkIndex(docs); err != nil {
			return fmt.Errorf("failed to bulk index batch: %w", err)
		}

		s.logger.Info("reindexed batch", zap.Int("start", i), zap.Int("end", end), zap.Int("total", len(scenes)))
	}

	s.logger.Info("full reindex completed", zap.Int("total_scenes", len(scenes)))
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
