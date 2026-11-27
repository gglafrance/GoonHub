package response

import (
	"time"

	"github.com/google/uuid"
	"goonhub/internal/data"
)

type SavedSearchResponse struct {
	UUID      uuid.UUID                `json:"uuid"`
	Name      string                   `json:"name"`
	Filters   SavedSearchFiltersOutput `json:"filters"`
	CreatedAt time.Time                `json:"created_at"`
	UpdatedAt time.Time                `json:"updated_at"`
}

type SavedSearchFiltersOutput struct {
	Query          string   `json:"query,omitempty"`
	MatchType      string   `json:"match_type,omitempty"`
	SelectedTags   []string `json:"selected_tags,omitempty"`
	SelectedActors []string `json:"selected_actors,omitempty"`
	Studio         string   `json:"studio,omitempty"`
	Resolution     string   `json:"resolution,omitempty"`
	MinDuration    *int     `json:"min_duration,omitempty"`
	MaxDuration    *int     `json:"max_duration,omitempty"`
	MinSize        *int     `json:"min_size,omitempty"` // megabytes
	MaxSize        *int     `json:"max_size,omitempty"`
	// Library status filters (nil = any)
	HasActors    *bool    `json:"has_actors,omitempty"`
	HasTags      *bool    `json:"has_tags,omitempty"`
	HasStudio    *bool    `json:"has_studio,omitempty"`
	SourceSites  []string `json:"source_sites,omitempty"` // normalized source sites (OR)
	HasSource    *bool    `json:"has_source,omitempty"`
	Watched      *bool    `json:"watched,omitempty"`
	Rated        *bool    `json:"rated,omitempty"`
	HasMarkers   *bool    `json:"has_markers,omitempty"`
	MarkerLabels []string `json:"selected_marker_labels,omitempty"`
	MinDate      string   `json:"min_date,omitempty"`
	MaxDate      string   `json:"max_date,omitempty"`
	Liked        *bool    `json:"liked,omitempty"`
	MinRating    *float64 `json:"min_rating,omitempty"`
	MaxRating    *float64 `json:"max_rating,omitempty"`
	MinJizzCount *int     `json:"min_jizz_count,omitempty"`
	MaxJizzCount *int     `json:"max_jizz_count,omitempty"`
	Sort         string   `json:"sort,omitempty"`
}

func NewSavedSearchResponse(s *data.SavedSearch) SavedSearchResponse {
	return SavedSearchResponse{
		UUID:      s.UUID,
		Name:      s.Name,
		Filters:   filtersToOutput(s.Filters),
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

func NewSavedSearchListResponse(searches []data.SavedSearch) []SavedSearchResponse {
	result := make([]SavedSearchResponse, len(searches))
	for i, s := range searches {
		result[i] = NewSavedSearchResponse(&s)
	}
	return result
}

func filtersToOutput(f data.Filters) SavedSearchFiltersOutput {
	return SavedSearchFiltersOutput{
		Query:          f.Query,
		MatchType:      f.MatchType,
		SelectedTags:   f.SelectedTags,
		SelectedActors: f.SelectedActors,
		Studio:         f.Studio,
		Resolution:     f.Resolution,
		MinDuration:    f.MinDuration,
		MaxDuration:    f.MaxDuration,
		MinSize:        f.MinSize,
		MaxSize:        f.MaxSize,
		HasActors:      f.HasActors,
		HasTags:        f.HasTags,
		HasStudio:      f.HasStudio,
		SourceSites:    f.SourceSites,
		HasSource:      f.HasSource,
		Watched:        f.Watched,
		Rated:          f.Rated,
		HasMarkers:     f.HasMarkers,
		MarkerLabels:   f.MarkerLabels,
		MinDate:        f.MinDate,
		MaxDate:        f.MaxDate,
		Liked:          f.Liked,
		MinRating:      f.MinRating,
		MaxRating:      f.MaxRating,
		MinJizzCount:   f.MinJizzCount,
		MaxJizzCount:   f.MaxJizzCount,
		Sort:           f.Sort,
	}
}
