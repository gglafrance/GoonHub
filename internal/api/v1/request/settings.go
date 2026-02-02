package request

type UpdatePlayerSettingsRequest struct {
	Autoplay      bool `json:"autoplay"`
	DefaultVolume int  `json:"default_volume" binding:"min=0,max=100"`
	Loop          bool `json:"loop"`
}

type UpdateAppSettingsRequest struct {
	VideosPerPage          int    `json:"videos_per_page" binding:"required,min=1,max=100"`
	DefaultSortOrder       string `json:"default_sort_order" binding:"required"`
	MarkerThumbnailCycling bool   `json:"marker_thumbnail_cycling"`
}

type UpdateTagSettingsRequest struct {
	DefaultTagSort string `json:"default_tag_sort" binding:"required"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=6"`
}

type ChangeUsernameRequest struct {
	Username string `json:"username" binding:"required,min=3"`
}

type UpdateHomepageConfigRequest struct {
	ShowUpload bool                       `json:"show_upload"`
	Sections   []HomepageSectionRequest   `json:"sections" binding:"required,dive"`
}

type HomepageSectionRequest struct {
	ID      string                 `json:"id" binding:"required"`
	Type    string                 `json:"type" binding:"required,oneof=latest actor studio tag saved_search continue_watching most_viewed liked"`
	Title   string                 `json:"title" binding:"required,max=100"`
	Enabled bool                   `json:"enabled"`
	Limit   int                    `json:"limit" binding:"required,min=1,max=50"`
	Order   int                    `json:"order" binding:"min=0"`
	Sort    string                 `json:"sort"`
	Config  map[string]interface{} `json:"config"`
}
