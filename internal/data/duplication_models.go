package data

import "time"

type DuplicateGroup struct {
	ID          uint                   `gorm:"primaryKey" json:"id"`
	Status      string                 `gorm:"size:20;not null;default:'unresolved'" json:"status"`
	SceneCount  int                    `gorm:"not null;default:0" json:"scene_count"`
	BestSceneID *uint                  `gorm:"column:best_scene_id" json:"best_scene_id"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	ResolvedAt  *time.Time             `json:"resolved_at,omitempty"`
	Members     []DuplicateGroupMember `gorm:"foreignKey:GroupID" json:"members,omitempty"`
}

func (DuplicateGroup) TableName() string {
	return "duplicate_groups"
}

type DuplicateGroupMember struct {
	ID              uint    `gorm:"primaryKey" json:"id"`
	GroupID         uint    `gorm:"not null" json:"group_id"`
	SceneID         uint    `gorm:"not null" json:"scene_id"`
	IsBest          bool    `gorm:"not null;default:false" json:"is_best"`
	ConfidenceScore float64 `gorm:"not null;default:0" json:"confidence_score"`
	MatchType       string  `gorm:"size:10;not null;default:'audio'" json:"match_type"`
	// Matching segment in this scene and the corresponding segment in RefSceneID (seconds)
	MatchStart *float64 `gorm:"column:match_start" json:"match_start,omitempty"`
	MatchEnd   *float64 `gorm:"column:match_end" json:"match_end,omitempty"`
	RefSceneID *uint    `gorm:"column:ref_scene_id" json:"ref_scene_id,omitempty"`
	RefStart   *float64 `gorm:"column:ref_start" json:"ref_start,omitempty"`
	RefEnd     *float64 `gorm:"column:ref_end" json:"ref_end,omitempty"`
	// DeleteAfter schedules the permanent deletion of a scene not kept when its group was resolved
	DeleteAfter *time.Time `gorm:"column:delete_after" json:"delete_after,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

func (DuplicateGroupMember) TableName() string {
	return "duplicate_group_members"
}

type DuplicationConfigRecord struct {
	ID                      int     `gorm:"primaryKey" json:"id"`
	AudioDensityThreshold   float64 `gorm:"column:audio_density_threshold" json:"audio_density_threshold"`
	AudioMinHashes          int     `gorm:"column:audio_min_hashes" json:"audio_min_hashes"`
	AudioMaxHashOccurrences int     `gorm:"column:audio_max_hash_occurrences" json:"audio_max_hash_occurrences"`
	AudioMinSpan            int     `gorm:"column:audio_min_span" json:"audio_min_span"`
	VisualHammingMax        int     `gorm:"column:visual_hamming_max" json:"visual_hamming_max"`
	VisualMinFrames         int     `gorm:"column:visual_min_frames" json:"visual_min_frames"`
	VisualMinSpan           int     `gorm:"column:visual_min_span" json:"visual_min_span"`
	DeltaTolerance          int     `gorm:"column:delta_tolerance" json:"delta_tolerance"`
	FingerprintMode         string  `gorm:"column:fingerprint_mode;default:'audio_only'" json:"fingerprint_mode"`
	// DeleteResolvedFiles permanently deletes (scene and file) the scenes not kept when a group is
	// resolved, DeletionDelayHours after the resolution (they stay in the trash meanwhile).
	DeleteResolvedFiles bool `gorm:"column:delete_resolved_files;not null;default:false" json:"delete_resolved_files"`
	DeletionDelayHours  int  `gorm:"column:deletion_delay_hours;not null;default:72" json:"deletion_delay_hours"`
	// BestSelectionCriteria is the ordered, comma-separated list of criteria used to pick the
	// best variant of a group (see core.AllBestCriteria). Empty means the default order.
	BestSelectionCriteria string    `gorm:"column:best_selection_criteria;not null;default:''" json:"best_selection_criteria"`
	UpdatedAt             time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (DuplicationConfigRecord) TableName() string {
	return "duplication_config"
}
