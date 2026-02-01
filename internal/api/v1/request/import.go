package request

// ImportSceneRequest represents a request to import a scene with pre-existing metadata.
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
	Description      string  `json:"description"`
	ReleaseDate      *string `json:"release_date,omitempty"`
	StudioID         *uint   `json:"studio_id,omitempty"`
	StoragePathID    *uint   `json:"storage_path_id,omitempty"`
	Origin           string  `json:"origin,omitempty"`
	Type             string  `json:"type,omitempty"`
	SkipFileCheck    bool    `json:"skip_file_check,omitempty"`
}

// ImportMarkerRequest represents a request to import a marker with pre-existing data.
type ImportMarkerRequest struct {
	SceneID   uint   `json:"scene_id" binding:"required"`
	UserID    uint   `json:"user_id" binding:"required"`
	Timestamp int    `json:"timestamp" binding:"min=0"`
	Label     string `json:"label"`
	Color     string `json:"color"`
}
