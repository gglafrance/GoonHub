package request

type SavedSearchFilters struct {
	Query          string   `json:"query,omitempty"`
	MatchType      string   `json:"match_type,omitempty"`
	SelectedTags   []string `json:"selected_tags,omitempty"`
	SelectedActors []string `json:"selected_actors,omitempty"`
	Studio         string   `json:"studio,omitempty"`
	Resolution     string   `json:"resolution,omitempty"`
	MinDuration    *int     `json:"min_duration,omitempty"`
	MaxDuration    *int     `json:"max_duration,omitempty"`
	MinDate        string   `json:"min_date,omitempty"`
	MaxDate        string   `json:"max_date,omitempty"`
	Liked          *bool    `json:"liked,omitempty"`
	MinRating      *float64 `json:"min_rating,omitempty"`
	MaxRating      *float64 `json:"max_rating,omitempty"`
	MinJizzCount   *int     `json:"min_jizz_count,omitempty"`
	MaxJizzCount   *int     `json:"max_jizz_count,omitempty"`
	Sort           string   `json:"sort,omitempty"`
}

type CreateSavedSearchRequest struct {
	Name    string             `json:"name" binding:"required"`
	Filters SavedSearchFilters `json:"filters"`
}

type UpdateSavedSearchRequest struct {
	Name    *string             `json:"name,omitempty"`
	Filters *SavedSearchFilters `json:"filters,omitempty"`
}
