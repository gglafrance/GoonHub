package response

import "time"

type DuplicateConfigResponse struct {
	Enabled         bool   `json:"enabled"`
	CheckOnUpload   bool   `json:"check_on_upload"`
	MatchThreshold  int    `json:"match_threshold"`
	HammingDistance int    `json:"hamming_distance"`
	SampleInterval  int    `json:"sample_interval"`
	DuplicateAction string `json:"duplicate_action"`
	KeepBestRules   any    `json:"keep_best_rules"`
	KeepBestEnabled any    `json:"keep_best_enabled"`
	CodecPreference any    `json:"codec_preference"`
}

type DuplicateGroupResponse struct {
	ID            uint                           `json:"id"`
	Status        string                         `json:"status"`
	WinnerSceneID *uint                          `json:"winner_scene_id,omitempty"`
	Members       []DuplicateGroupMemberResponse `json:"members,omitempty"`
	CreatedAt     time.Time                      `json:"created_at"`
	UpdatedAt     time.Time                      `json:"updated_at"`
}

type DuplicateGroupMemberResponse struct {
	ID              uint    `json:"id"`
	SceneID         uint    `json:"scene_id"`
	MatchPercentage float64 `json:"match_percentage"`
	FrameOffset     int     `json:"frame_offset"`
	IsWinner        bool    `json:"is_winner"`
	Scene           *DuplicateSceneSummary `json:"scene,omitempty"`
}

type DuplicateSceneSummary struct {
	ID             uint    `json:"id"`
	Title          string  `json:"title"`
	Duration       float64 `json:"duration"`
	Width          int     `json:"width"`
	Height         int     `json:"height"`
	VideoCodec     string  `json:"video_codec"`
	BitRate        int64   `json:"bit_rate"`
	FileSize       int64   `json:"file_size"`
	ThumbnailPath  string  `json:"thumbnail_path"`
}

type RescanStatusResponse struct {
	Running   bool `json:"running"`
	Total     int  `json:"total"`
	Completed int  `json:"completed"`
	Matched   int  `json:"matched"`
}
