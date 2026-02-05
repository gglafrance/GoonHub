package request

type UpdateSceneDetailsRequest struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	ReleaseDate *string `json:"release_date,omitempty"`
}

type SetRatingRequest struct {
	Rating float64 `json:"rating" binding:"required,min=0.5,max=5"`
}

type SearchScenesRequest struct {
	Query        string  `form:"q"`
	Tags         string  `form:"tags"`
	Actors       string  `form:"actors"`
	Studio       string  `form:"studio"`
	MinDuration  int     `form:"min_duration"`
	MaxDuration  int     `form:"max_duration"`
	MinDate      string  `form:"min_date"`
	MaxDate      string  `form:"max_date"`
	Resolution   string  `form:"resolution"`
	Sort         string  `form:"sort"`
	Page         int     `form:"page"`
	Limit        int     `form:"limit"`
	Liked        *bool   `form:"liked"`
	MinRating    float64 `form:"min_rating"`
	MaxRating    float64 `form:"max_rating"`
	MinJizzCount int     `form:"min_jizz_count"`
	MaxJizzCount int     `form:"max_jizz_count"`
	MatchType    string  `form:"match_type"`
	MarkerLabels string  `form:"marker_labels"` // Comma-separated list of marker labels
	Seed         int64   `form:"seed"`           // Random shuffle seed (0 = auto-generate)
}

type ApplySceneMetadataRequest struct {
	Title         *string  `json:"title,omitempty"`
	Description   *string  `json:"description,omitempty"`
	Studio        *string  `json:"studio,omitempty"`
	ThumbnailURL  *string  `json:"thumbnail_url,omitempty"`
	ActorIDs      []uint   `json:"actor_ids,omitempty"`
	TagNames      []string `json:"tag_names,omitempty"`
	ReleaseDate   *string  `json:"release_date,omitempty"`
	PornDBSceneID *string  `json:"porndb_scene_id,omitempty"`
}

type DeleteSceneRequest struct {
	Permanent bool `json:"permanent"`
}
