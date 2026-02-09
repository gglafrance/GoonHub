package request

import "encoding/json"

type UpdateDuplicateConfigRequest struct {
	Enabled         bool            `json:"enabled"`
	CheckOnUpload   bool            `json:"check_on_upload"`
	MatchThreshold  int             `json:"match_threshold" binding:"min=50,max=100"`
	HammingDistance int             `json:"hamming_distance" binding:"min=1,max=15"`
	SampleInterval  int             `json:"sample_interval" binding:"min=1,max=10"`
	DuplicateAction string          `json:"duplicate_action" binding:"oneof=flag mark trash"`
	KeepBestRules   json.RawMessage `json:"keep_best_rules"`
	KeepBestEnabled json.RawMessage `json:"keep_best_enabled"`
	CodecPreference json.RawMessage `json:"codec_preference"`
}

type SetWinnerRequest struct {
	SceneID uint `json:"scene_id" binding:"required"`
}
