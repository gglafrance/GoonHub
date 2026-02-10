package data

import (
	"encoding/json"
	"time"
)

// SceneFingerprint stores a per-frame perceptual hash for duplicate detection.
type SceneFingerprint struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	SceneID    uint      `gorm:"not null;uniqueIndex:idx_scene_frame" json:"scene_id"`
	FrameIndex int       `gorm:"not null;uniqueIndex:idx_scene_frame" json:"frame_index"`
	HashValue  int64     `gorm:"not null" json:"hash_value"`
	CreatedAt  time.Time `json:"created_at"`
}

func (SceneFingerprint) TableName() string {
	return "scene_fingerprints"
}

// DuplicateGroup represents a group of scenes identified as duplicates.
type DuplicateGroup struct {
	ID            uint                   `gorm:"primarykey" json:"id"`
	Status        string                 `gorm:"not null;default:'pending'" json:"status"`
	WinnerSceneID *uint                  `json:"winner_scene_id"`
	ResolvedAt    *time.Time             `json:"resolved_at,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
	Members       []DuplicateGroupMember `gorm:"foreignKey:GroupID" json:"members,omitempty"`
}

func (DuplicateGroup) TableName() string {
	return "duplicate_groups"
}

// DuplicateGroupMember links a scene to a duplicate group with match info.
type DuplicateGroupMember struct {
	ID              uint      `gorm:"primarykey" json:"id"`
	GroupID         uint      `gorm:"not null" json:"group_id"`
	SceneID         uint      `gorm:"not null" json:"scene_id"`
	MatchPercentage float64   `gorm:"not null;default:0" json:"match_percentage"`
	FrameOffset     int       `gorm:"not null;default:0" json:"frame_offset"`
	IsWinner        bool      `gorm:"not null;default:false" json:"is_winner"`
	CreatedAt       time.Time `json:"created_at"`
}

func (DuplicateGroupMember) TableName() string {
	return "duplicate_group_members"
}

// DuplicateConfigRecord holds the singleton duplicate detection configuration.
type DuplicateConfigRecord struct {
	ID               int             `gorm:"primaryKey" json:"id"`
	Enabled          bool            `gorm:"not null;default:false" json:"enabled"`
	CheckOnUpload    bool            `gorm:"not null;default:true" json:"check_on_upload"`
	MatchThreshold   int             `gorm:"not null;default:80" json:"match_threshold"`
	HammingDistance  int             `gorm:"not null;default:8" json:"hamming_distance"`
	SampleInterval  int             `gorm:"not null;default:2" json:"sample_interval"`
	DuplicateAction  string          `gorm:"not null;default:'flag'" json:"duplicate_action"`
	KeepBestRules    json.RawMessage `gorm:"type:jsonb;not null;default:'[\"duration\",\"resolution\",\"codec\",\"bitrate\"]'" json:"keep_best_rules"`
	KeepBestEnabled  json.RawMessage `gorm:"type:jsonb;not null;default:'{\"duration\":true,\"resolution\":true,\"codec\":true,\"bitrate\":true}'" json:"keep_best_enabled"`
	CodecPreference  json.RawMessage `gorm:"type:jsonb;not null;default:'[\"h265\",\"hevc\",\"av1\",\"vp9\",\"h264\"]'" json:"codec_preference"`
	UpdatedAt        time.Time       `gorm:"column:updated_at" json:"updated_at"`
}

func (DuplicateConfigRecord) TableName() string {
	return "duplicate_config"
}
