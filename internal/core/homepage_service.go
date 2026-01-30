package core

import (
	"fmt"
	"sort"

	"go.uber.org/zap"

	"goonhub/internal/data"
)

// WatchProgress represents the watch progress for a video
type WatchProgress struct {
	LastPosition int `json:"last_position"`
	Duration     int `json:"duration"`
}

// HomepageSectionData represents a section with its fetched video data
type HomepageSectionData struct {
	Section       data.HomepageSection   `json:"section"`
	Videos        []data.Video           `json:"videos"`
	Total         int64                  `json:"total"`
	WatchProgress map[uint]WatchProgress `json:"watch_progress,omitempty"`
	Ratings       map[uint]float64       `json:"ratings,omitempty"`
}

// HomepageResponse represents the full homepage data
type HomepageResponse struct {
	Config   data.HomepageConfig   `json:"config"`
	Sections []HomepageSectionData `json:"sections"`
}

// HomepageService handles fetching homepage section data
type HomepageService struct {
	settingsService    *SettingsService
	searchService      *SearchService
	savedSearchService *SavedSearchService
	watchHistoryRepo   data.WatchHistoryRepository
	interactionRepo    data.InteractionRepository
	videoRepo          data.VideoRepository
	tagRepo            data.TagRepository
	actorRepo          data.ActorRepository
	studioRepo         data.StudioRepository
	logger             *zap.Logger
}

// NewHomepageService creates a new HomepageService
func NewHomepageService(
	settingsService *SettingsService,
	searchService *SearchService,
	savedSearchService *SavedSearchService,
	watchHistoryRepo data.WatchHistoryRepository,
	interactionRepo data.InteractionRepository,
	videoRepo data.VideoRepository,
	tagRepo data.TagRepository,
	actorRepo data.ActorRepository,
	studioRepo data.StudioRepository,
	logger *zap.Logger,
) *HomepageService {
	return &HomepageService{
		settingsService:    settingsService,
		searchService:      searchService,
		savedSearchService: savedSearchService,
		watchHistoryRepo:   watchHistoryRepo,
		interactionRepo:    interactionRepo,
		videoRepo:          videoRepo,
		tagRepo:            tagRepo,
		actorRepo:          actorRepo,
		studioRepo:         studioRepo,
		logger:             logger,
	}
}

// GetHomepageData fetches the full homepage data for a user
func (s *HomepageService) GetHomepageData(userID uint) (*HomepageResponse, error) {
	config, err := s.settingsService.GetHomepageConfig(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get homepage config: %w", err)
	}

	// Sort sections by order
	sections := make([]data.HomepageSection, len(config.Sections))
	copy(sections, config.Sections)
	sort.Slice(sections, func(i, j int) bool {
		return sections[i].Order < sections[j].Order
	})

	// Fetch data for each enabled section
	response := &HomepageResponse{
		Config:   *config,
		Sections: make([]HomepageSectionData, 0, len(sections)),
	}

	for _, section := range sections {
		if !section.Enabled {
			continue
		}

		sectionData, err := s.fetchSectionData(userID, section)
		if err != nil {
			s.logger.Warn("failed to fetch section data",
				zap.String("section_id", section.ID),
				zap.String("section_type", section.Type),
				zap.Error(err),
			)
			// Continue with empty section rather than failing entire request
			sectionData = &HomepageSectionData{
				Section: section,
				Videos:  []data.Video{},
				Total:   0,
			}
		}
		response.Sections = append(response.Sections, *sectionData)
	}

	return response, nil
}

// GetSectionData fetches data for a single section
func (s *HomepageService) GetSectionData(userID uint, sectionID string) (*HomepageSectionData, error) {
	config, err := s.settingsService.GetHomepageConfig(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get homepage config: %w", err)
	}

	var section *data.HomepageSection
	for i := range config.Sections {
		if config.Sections[i].ID == sectionID {
			section = &config.Sections[i]
			break
		}
	}

	if section == nil {
		return nil, fmt.Errorf("section not found: %s", sectionID)
	}

	return s.fetchSectionData(userID, *section)
}

func (s *HomepageService) fetchSectionData(userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	var sectionData *HomepageSectionData
	var err error

	switch section.Type {
	case "latest":
		sectionData, err = s.fetchLatestSection(userID, section)
	case "actor":
		sectionData, err = s.fetchActorSection(userID, section)
	case "studio":
		sectionData, err = s.fetchStudioSection(userID, section)
	case "tag":
		sectionData, err = s.fetchTagSection(userID, section)
	case "saved_search":
		sectionData, err = s.fetchSavedSearchSection(userID, section)
	case "continue_watching":
		sectionData, err = s.fetchContinueWatchingSection(userID, section)
	case "most_viewed":
		sectionData, err = s.fetchMostViewedSection(userID, section)
	case "liked":
		sectionData, err = s.fetchLikedSection(userID, section)
	default:
		return nil, fmt.Errorf("unknown section type: %s", section.Type)
	}

	if err != nil {
		return nil, err
	}

	// Enrich with ratings
	if len(sectionData.Videos) > 0 {
		sectionData.Ratings = s.fetchRatingsForVideos(userID, sectionData.Videos)
	}

	return sectionData, nil
}

// fetchRatingsForVideos fetches user ratings for a list of videos
func (s *HomepageService) fetchRatingsForVideos(userID uint, videos []data.Video) map[uint]float64 {
	if s.interactionRepo == nil {
		return nil
	}

	videoIDs := make([]uint, len(videos))
	for i, v := range videos {
		videoIDs[i] = v.ID
	}

	ratings, err := s.interactionRepo.GetRatingsByVideoIDs(userID, videoIDs)
	if err != nil {
		s.logger.Warn("failed to fetch ratings for videos", zap.Error(err))
		return nil
	}

	return ratings
}

func (s *HomepageService) fetchLatestSection(userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	sortOrder := section.Sort
	if sortOrder == "" {
		sortOrder = "created_at_desc"
	}

	params := data.VideoSearchParams{
		Page:   1,
		Limit:  section.Limit,
		Sort:   sortOrder,
		UserID: userID,
	}

	videos, total, err := s.searchService.Search(params)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return &HomepageSectionData{
		Section: section,
		Videos:  videos,
		Total:   total,
	}, nil
}

func (s *HomepageService) fetchActorSection(userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	actorUUID, ok := section.Config["actor_uuid"].(string)
	if !ok || actorUUID == "" {
		return nil, fmt.Errorf("actor_uuid not found in config")
	}

	// Get actor name from UUID
	actor, err := s.actorRepo.GetByUUID(actorUUID)
	if err != nil {
		return nil, fmt.Errorf("actor not found: %w", err)
	}

	sortOrder := section.Sort
	if sortOrder == "" {
		sortOrder = "created_at_desc"
	}

	params := data.VideoSearchParams{
		Page:   1,
		Limit:  section.Limit,
		Sort:   sortOrder,
		Actors: []string{actor.Name},
		UserID: userID,
	}

	videos, total, err := s.searchService.Search(params)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return &HomepageSectionData{
		Section: section,
		Videos:  videos,
		Total:   total,
	}, nil
}

func (s *HomepageService) fetchStudioSection(userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	studioUUID, ok := section.Config["studio_uuid"].(string)
	if !ok || studioUUID == "" {
		return nil, fmt.Errorf("studio_uuid not found in config")
	}

	// Get studio name from UUID
	studio, err := s.studioRepo.GetByUUID(studioUUID)
	if err != nil {
		return nil, fmt.Errorf("studio not found: %w", err)
	}

	sortOrder := section.Sort
	if sortOrder == "" {
		sortOrder = "created_at_desc"
	}

	params := data.VideoSearchParams{
		Page:   1,
		Limit:  section.Limit,
		Sort:   sortOrder,
		Studio: studio.Name,
		UserID: userID,
	}

	videos, total, err := s.searchService.Search(params)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return &HomepageSectionData{
		Section: section,
		Videos:  videos,
		Total:   total,
	}, nil
}

func (s *HomepageService) fetchTagSection(userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	var tagID uint
	switch v := section.Config["tag_id"].(type) {
	case float64:
		tagID = uint(v)
	case string:
		var parsed uint64
		if _, err := fmt.Sscanf(v, "%d", &parsed); err != nil {
			return nil, fmt.Errorf("invalid tag_id format: %s", v)
		}
		tagID = uint(parsed)
	default:
		return nil, fmt.Errorf("tag_id not found in config")
	}

	sortOrder := section.Sort
	if sortOrder == "" {
		sortOrder = "created_at_desc"
	}

	params := data.VideoSearchParams{
		Page:   1,
		Limit:  section.Limit,
		Sort:   sortOrder,
		TagIDs: []uint{tagID},
		UserID: userID,
	}

	videos, total, err := s.searchService.Search(params)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return &HomepageSectionData{
		Section: section,
		Videos:  videos,
		Total:   total,
	}, nil
}

func (s *HomepageService) fetchSavedSearchSection(userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	savedSearchUUID, ok := section.Config["saved_search_uuid"].(string)
	if !ok || savedSearchUUID == "" {
		return nil, fmt.Errorf("saved_search_uuid not found in config")
	}

	savedSearch, err := s.savedSearchService.GetByUUID(userID, savedSearchUUID)
	if err != nil {
		return nil, fmt.Errorf("saved search not found: %w", err)
	}

	sortOrder := section.Sort
	if sortOrder == "" && savedSearch.Filters.Sort != "" {
		sortOrder = savedSearch.Filters.Sort
	}
	if sortOrder == "" {
		sortOrder = "created_at_desc"
	}

	// Convert saved search filters to VideoSearchParams
	params := data.VideoSearchParams{
		Page:   1,
		Limit:  section.Limit,
		Sort:   sortOrder,
		Query:  savedSearch.Filters.Query,
		Studio: savedSearch.Filters.Studio,
		UserID: userID,
	}

	if savedSearch.Filters.MatchType != "" {
		params.MatchingStrategy = savedSearch.Filters.MatchType
	}

	// Convert tag names to tag IDs
	if len(savedSearch.Filters.SelectedTags) > 0 {
		tagIDs, err := s.tagRepo.GetIDsByNames(savedSearch.Filters.SelectedTags)
		if err != nil {
			s.logger.Warn("failed to get tag IDs", zap.Error(err))
		} else {
			params.TagIDs = tagIDs
		}
	}

	if len(savedSearch.Filters.SelectedActors) > 0 {
		params.Actors = savedSearch.Filters.SelectedActors
	}

	if savedSearch.Filters.MinDuration != nil {
		params.MinDuration = *savedSearch.Filters.MinDuration
	}
	if savedSearch.Filters.MaxDuration != nil {
		params.MaxDuration = *savedSearch.Filters.MaxDuration
	}

	if savedSearch.Filters.Liked != nil {
		params.Liked = savedSearch.Filters.Liked
	}
	if savedSearch.Filters.MinRating != nil {
		params.MinRating = *savedSearch.Filters.MinRating
	}
	if savedSearch.Filters.MaxRating != nil {
		params.MaxRating = *savedSearch.Filters.MaxRating
	}
	if savedSearch.Filters.MinJizzCount != nil {
		params.MinJizzCount = *savedSearch.Filters.MinJizzCount
	}
	if savedSearch.Filters.MaxJizzCount != nil {
		params.MaxJizzCount = *savedSearch.Filters.MaxJizzCount
	}

	videos, total, err := s.searchService.Search(params)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return &HomepageSectionData{
		Section: section,
		Videos:  videos,
		Total:   total,
	}, nil
}

func (s *HomepageService) fetchContinueWatchingSection(userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	// Get videos with resume positions (not completed)
	// Fetch more than needed to filter for incomplete watches
	watches, _, err := s.watchHistoryRepo.ListUserHistory(userID, 1, section.Limit*3)
	if err != nil {
		return nil, fmt.Errorf("failed to get watch history: %w", err)
	}

	// Filter to only incomplete watches with position > 0
	// Also build a map of video ID -> last position
	var videoIDs []uint
	watchPositions := make(map[uint]int)
	for _, watch := range watches {
		if !watch.Completed && watch.LastPosition > 0 {
			videoIDs = append(videoIDs, watch.VideoID)
			watchPositions[watch.VideoID] = watch.LastPosition
			if len(videoIDs) >= section.Limit {
				break
			}
		}
	}

	if len(videoIDs) == 0 {
		return &HomepageSectionData{
			Section: section,
			Videos:  []data.Video{},
			Total:   0,
		}, nil
	}

	videos, err := s.videoRepo.GetByIDs(videoIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get videos: %w", err)
	}

	// Build watch progress map with position and duration
	watchProgress := make(map[uint]WatchProgress)
	for _, video := range videos {
		if pos, ok := watchPositions[video.ID]; ok {
			watchProgress[video.ID] = WatchProgress{
				LastPosition: pos,
				Duration:     video.Duration,
			}
		}
	}

	// Return the actual count of videos we found, not the unfiltered total
	return &HomepageSectionData{
		Section:       section,
		Videos:        videos,
		Total:         int64(len(videos)),
		WatchProgress: watchProgress,
	}, nil
}

func (s *HomepageService) fetchMostViewedSection(userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	sortOrder := section.Sort
	if sortOrder == "" {
		sortOrder = "view_count_desc"
	}

	params := data.VideoSearchParams{
		Page:   1,
		Limit:  section.Limit,
		Sort:   sortOrder,
		UserID: userID,
	}

	videos, total, err := s.searchService.Search(params)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return &HomepageSectionData{
		Section: section,
		Videos:  videos,
		Total:   total,
	}, nil
}

func (s *HomepageService) fetchLikedSection(userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	sortOrder := section.Sort
	if sortOrder == "" {
		sortOrder = "created_at_desc"
	}

	liked := true
	params := data.VideoSearchParams{
		Page:   1,
		Limit:  section.Limit,
		Sort:   sortOrder,
		UserID: userID,
		Liked:  &liked,
	}

	videos, total, err := s.searchService.Search(params)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return &HomepageSectionData{
		Section: section,
		Videos:  videos,
		Total:   total,
	}, nil
}

