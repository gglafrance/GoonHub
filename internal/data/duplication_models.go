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
	// Relation to RefSceneID derived from the segments and the durations (DuplicateRelation*)
	Relation string `gorm:"column:relation;size:10;not null;default:'unknown'" json:"relation"`
	// DeleteAfter schedules the permanent deletion of a scene not kept when its group was resolved
	DeleteAfter *time.Time `gorm:"column:delete_after" json:"delete_after,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

func (DuplicateGroupMember) TableName() string {
	return "duplicate_group_members"
}

// Relation of a group member to the scene it was matched against (its reference, RefSceneID).
// See core.ClassifyDuplicateRelation for how it is derived.
const (
	// DuplicateRelationIdentical: the match covers (nearly) all of both scenes.
	DuplicateRelationIdentical = "identical"
	// DuplicateRelationContained: this scene is a part of the reference (a clip of it).
	DuplicateRelationContained = "contained"
	// DuplicateRelationContains: the reference is a part of this scene (e.g. a compilation).
	DuplicateRelationContains = "contains"
	// DuplicateRelationPartial: the match covers a minor part of both scenes.
	DuplicateRelationPartial = "partial"
	// DuplicateRelationUnknown: no usable segment or duration yet.
	DuplicateRelationUnknown = "unknown"
)

// Kind of a duplicate group, summarizing the relations of its members (core.DuplicateGroupKind).
const (
	// DuplicateKindIdentical: every member is identical to its reference.
	DuplicateKindIdentical = "identical"
	// DuplicateKindContainment: at least one member is contained in or contains its reference.
	DuplicateKindContainment = "containment"
	// DuplicateKindPartial: no containment, but at least one partial match.
	DuplicateKindPartial = "partial"
	// DuplicateKindUnverified: the other members are identical, but some have no relation yet.
	DuplicateKindUnverified = "unverified"
)

// DuplicateMemberRelationInput is a group member with what its relation is derived from.
type DuplicateMemberRelationInput struct {
	ID          uint
	MatchStart  *float64
	MatchEnd    *float64
	RefStart    *float64
	RefEnd      *float64
	Duration    int // duration of the member scene (seconds)
	RefDuration int // duration of the reference scene (seconds), 0 when unknown
}

// DuplicateMemberScene is a group member with the status of its group and the card data of its
// scene (ListSceneGroupMembers).
type DuplicateMemberScene struct {
	GroupID         uint
	GroupStatus     string
	SceneID         uint
	IsBest          bool
	ConfidenceScore float64
	MatchType       string
	MatchStart      *float64
	MatchEnd        *float64
	RefSceneID      *uint
	RefStart        *float64
	RefEnd          *float64
	Relation        string
	Title           string
	Duration        int
	Width           int
	Height          int
	Size            int64
	ThumbnailPath   string
	TrashedAt       *time.Time
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
