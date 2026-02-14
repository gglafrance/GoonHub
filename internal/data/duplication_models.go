package data

import "time"

type DuplicateGroup struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Status      string     `gorm:"size:20;not null;default:'unresolved'" json:"status"`
	SceneCount  int        `gorm:"not null;default:0" json:"scene_count"`
	BestSceneID *uint      `gorm:"column:best_scene_id" json:"best_scene_id"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
	Members     []DuplicateGroupMember `gorm:"foreignKey:GroupID" json:"members,omitempty"`
}

func (DuplicateGroup) TableName() string {
	return "duplicate_groups"
}

type DuplicateGroupMember struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	GroupID         uint      `gorm:"not null" json:"group_id"`
	SceneID         uint      `gorm:"not null" json:"scene_id"`
	IsBest          bool      `gorm:"not null;default:false" json:"is_best"`
	ConfidenceScore float64   `gorm:"not null;default:0" json:"confidence_score"`
	MatchType       string    `gorm:"size:10;not null;default:'audio'" json:"match_type"`
	CreatedAt       time.Time `json:"created_at"`
}

func (DuplicateGroupMember) TableName() string {
	return "duplicate_group_members"
}

type DuplicationConfigRecord struct {
	ID                      int       `gorm:"primaryKey" json:"id"`
	AudioDensityThreshold   float64   `gorm:"column:audio_density_threshold" json:"audio_density_threshold"`
	AudioMinHashes          int       `gorm:"column:audio_min_hashes" json:"audio_min_hashes"`
	AudioMaxHashOccurrences int       `gorm:"column:audio_max_hash_occurrences" json:"audio_max_hash_occurrences"`
	AudioMinSpan            int       `gorm:"column:audio_min_span" json:"audio_min_span"`
	VisualHammingMax        int       `gorm:"column:visual_hamming_max" json:"visual_hamming_max"`
	VisualMinFrames         int       `gorm:"column:visual_min_frames" json:"visual_min_frames"`
	VisualMinSpan           int       `gorm:"column:visual_min_span" json:"visual_min_span"`
	DeltaTolerance          int       `gorm:"column:delta_tolerance" json:"delta_tolerance"`
	FingerprintMode         string    `gorm:"column:fingerprint_mode;default:'audio_only'" json:"fingerprint_mode"`
	UpdatedAt               time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (DuplicationConfigRecord) TableName() string {
	return "duplication_config"
}
