package request

// ImportSceneRequest represents a request to import a scene with pre-existing metadata.
// v2 fields are optional: nil slices/pointers mean "don't touch".
type ImportSceneRequest struct {
	Title            string  `json:"title" binding:"required"`
	StoredPath       string  `json:"stored_path" binding:"required"`
	OriginalFilename string  `json:"original_filename"`
	Size             int64   `json:"size"`
	Duration         int     `json:"duration"`
	Width            int     `json:"width"`
	Height           int     `json:"height"`
	FrameRate        float64 `json:"frame_rate"`
	BitRate          int64   `json:"bit_rate"`
	VideoCodec       string  `json:"video_codec"`
	AudioCodec       string  `json:"audio_codec"`
	Description      *string `json:"description,omitempty"`
	SourceURL        *string `json:"source_url,omitempty"`
	ReleaseDate      *string `json:"release_date,omitempty"`
	StudioID         *uint   `json:"studio_id,omitempty"`
	StoragePathID    *uint   `json:"storage_path_id,omitempty"`
	Origin           string  `json:"origin,omitempty"`
	Type             string  `json:"type,omitempty"`
	SkipFileCheck    bool    `json:"skip_file_check,omitempty"`

	// v2
	TagIDs         []uint                     `json:"tag_ids"`
	ActorIDs       []uint                     `json:"actor_ids"`
	ViewCount      *int64                     `json:"view_count,omitempty"`
	CreatedAt      *string                    `json:"created_at,omitempty"`
	FileCreatedAt  *string                    `json:"file_created_at,omitempty"`
	UpdateExisting bool                       `json:"update_existing,omitempty"`
	SkipIndex      bool                       `json:"skip_index,omitempty"`
	Interactions   *ImportInteractionsRequest `json:"interactions,omitempty"`
}

// ImportInteractionsRequest holds the interactions applied for the current user.
type ImportInteractionsRequest struct {
	Rating      *float64 `json:"rating,omitempty"`
	Liked       *bool    `json:"liked,omitempty"`
	JizzedCount *int     `json:"jizzed_count,omitempty"`
	// JizzHistory holds the dates (RFC 3339) of the "o", imported as dated events
	JizzHistory []string             `json:"jizz_history,omitempty"`
	Watches     []ImportWatchRequest `json:"watches,omitempty"`
}

// ImportWatchRequest is a watch history entry to import.
type ImportWatchRequest struct {
	WatchedAt     string `json:"watched_at"`
	WatchDuration int    `json:"watch_duration"`
	LastPosition  int    `json:"last_position"`
	Completed     bool   `json:"completed"`
}

// ImportMarkerRequest represents a request to import a marker with pre-existing data.
type ImportMarkerRequest struct {
	SceneID        uint   `json:"scene_id" binding:"required"`
	UserID         *uint  `json:"user_id,omitempty"`
	Timestamp      int    `json:"timestamp" binding:"min=0"`
	EndTimestamp   *int   `json:"end_timestamp,omitempty"`
	Label          string `json:"label"`
	Color          string `json:"color"`
	TagIDs         []uint `json:"tag_ids"`
	UpdateExisting bool   `json:"update_existing,omitempty"`
}

// ImportSceneSummaryRequest lists the scene IDs to verify after an import.
type ImportSceneSummaryRequest struct {
	IDs []uint `json:"ids"`
}
