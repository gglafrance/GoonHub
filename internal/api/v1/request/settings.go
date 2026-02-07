package request

type UpdateSortPreferencesRequest struct {
	Actors       string `json:"actors" binding:"required"`
	Studios      string `json:"studios" binding:"required"`
	Markers      string `json:"markers" binding:"required"`
	ActorScenes  string `json:"actor_scenes"`
	StudioScenes string `json:"studio_scenes"`
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
	Type    string                 `json:"type" binding:"required,oneof=latest actor studio tag saved_search continue_watching most_viewed liked playlist"`
	Title   string                 `json:"title" binding:"required,max=100"`
	Enabled bool                   `json:"enabled"`
	Limit   int                    `json:"limit" binding:"required,min=1,max=50"`
	Order   int                    `json:"order" binding:"min=0"`
	Sort    string                 `json:"sort"`
	Config  map[string]interface{} `json:"config"`
}

type UpdateParsingRulesRequest struct {
	Presets        []ParsingPresetRequest `json:"presets" binding:"dive"`
	ActivePresetID *string                `json:"activePresetId"`
}

type ParsingPresetRequest struct {
	ID        string                `json:"id" binding:"required"`
	Name      string                `json:"name" binding:"required,max=100"`
	IsBuiltIn bool                  `json:"isBuiltIn"`
	Rules     []ParsingRuleRequest  `json:"rules" binding:"dive"`
}

type ParsingRuleRequest struct {
	ID      string                   `json:"id" binding:"required"`
	Type    string                   `json:"type" binding:"required"`
	Enabled bool                     `json:"enabled"`
	Order   int                      `json:"order" binding:"min=0"`
	Config  ParsingRuleConfigRequest `json:"config"`
}

type ParsingRuleConfigRequest struct {
	KeepContent   bool   `json:"keepContent,omitempty"`
	Pattern       string `json:"pattern,omitempty"`
	Find          string `json:"find,omitempty"`
	Replace       string `json:"replace,omitempty"`
	CaseSensitive bool   `json:"caseSensitive,omitempty"`
	MinLength     int    `json:"minLength,omitempty"`
	CaseType      string `json:"caseType,omitempty"`
}

type UpdateSceneCardConfigRequest struct {
	Badges      UpdateBadgeZonesRequest `json:"badges"`
	ContentRows []ContentRowRequest     `json:"content_rows"`
}

type UpdateBadgeZonesRequest struct {
	TopLeft     BadgeZoneRequest `json:"top_left"`
	TopRight    BadgeZoneRequest `json:"top_right"`
	BottomLeft  BadgeZoneRequest `json:"bottom_left"`
	BottomRight BadgeZoneRequest `json:"bottom_right"`
}

type BadgeZoneRequest struct {
	Items     []string `json:"items"`
	Direction string   `json:"direction"`
}

type ContentRowRequest struct {
	Type      string `json:"type"`
	Field     string `json:"field,omitempty"`
	Mode      string `json:"mode,omitempty"`
	Left      string `json:"left,omitempty"`
	Right     string `json:"right,omitempty"`
	LeftMode  string `json:"left_mode,omitempty"`
	RightMode string `json:"right_mode,omitempty"`
}

type UpdateAllSettingsRequest struct {
	Autoplay                  bool                          `json:"autoplay"`
	DefaultVolume             int                           `json:"default_volume" binding:"min=0,max=100"`
	Loop                      bool                          `json:"loop"`
	AbLoopControls            bool                          `json:"ab_loop_controls"`
	VideosPerPage             int                           `json:"videos_per_page" binding:"required,min=1"`
	DefaultSortOrder          string                        `json:"default_sort_order" binding:"required"`
	DefaultTagSort            string                        `json:"default_tag_sort" binding:"required"`
	MarkerThumbnailCycling    bool                          `json:"marker_thumbnail_cycling"`
	HomepageConfig            UpdateHomepageConfigRequest    `json:"homepage_config" binding:"required"`
	ParsingRules              UpdateParsingRulesRequest      `json:"parsing_rules"`
	SortPreferences           UpdateSortPreferencesRequest   `json:"sort_preferences" binding:"required"`
	PlaylistAutoAdvance       string                        `json:"playlist_auto_advance"`
	PlaylistCountdownSeconds  int                           `json:"playlist_countdown_seconds"`
	ShowPageSizeSelector      bool                          `json:"show_page_size_selector"`
	SceneCardConfig           UpdateSceneCardConfigRequest   `json:"scene_card_config"`
}
