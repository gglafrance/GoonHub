package data

import (
	"time"
)

type UserSceneMarker struct {
	ID        uint `gorm:"primarykey" json:"id"`
	UserID    uint `gorm:"not null" json:"user_id"`
	SceneID   uint `gorm:"not null;column:scene_id" json:"scene_id"`
	Timestamp int  `gorm:"not null" json:"timestamp"` // seconds
	// EndTimestamp is the optional end of the marked range, in seconds (> Timestamp).
	EndTimestamp          *int      `gorm:"column:end_timestamp" json:"end_timestamp"`
	Label                 string    `gorm:"size:100" json:"label"`
	Color                 string    `gorm:"size:7;default:'#FFFFFF'" json:"color"`
	ThumbnailPath         string    `gorm:"size:255" json:"thumbnail_path"`
	AnimatedThumbnailPath string    `gorm:"size:255" json:"animated_thumbnail_path"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

func (UserSceneMarker) TableName() string {
	return "user_scene_markers"
}

type MarkerLabelSuggestion struct {
	Label string `json:"label"`
	Count int64  `json:"count"`
}

// MarkerLabelGroup represents a group of markers with the same label
type MarkerLabelGroup struct {
	Label              string `json:"label"`
	Count              int64  `json:"count"`
	ThumbnailMarkerID  uint   `json:"thumbnail_marker_id"`
	ThumbnailMarkerIDs []uint `json:"thumbnail_marker_ids" gorm:"-"`
	// ThumbnailVersions maps the IDs of ThumbnailMarkerIDs to their thumbnail version
	// (MarkerThumbnailVersion), appended to the thumbnail URLs so that browsers refetch a
	// regenerated thumbnail.
	ThumbnailVersions map[uint]int64 `json:"thumbnail_versions" gorm:"-"`
}

// MarkerThumbnailRef is a marker picked as a thumbnail, with its last update time.
type MarkerThumbnailRef struct {
	ID        uint
	UpdatedAt time.Time
}

// MarkerThumbnailVersion is the version of a marker's thumbnail files: its updated_at in Unix
// milliseconds. updated_at moves each time a thumbnail path is written (UpdateThumbnailPath,
// UpdateAnimatedThumbnailPath), so a regenerated thumbnail gets a new version. The frontend
// computes the same value from the marker's updated_at.
func MarkerThumbnailVersion(updatedAt time.Time) int64 {
	return updatedAt.UnixMilli()
}

// MarkerWithScene extends UserSceneMarker with scene information
type MarkerWithScene struct {
	UserSceneMarker
	SceneTitle string          `json:"scene_title"`
	Tags       []MarkerTagInfo `json:"tags,omitempty" gorm:"-"`
}

// MarkerWithTags extends UserSceneMarker with tags
type MarkerWithTags struct {
	UserSceneMarker
	Tags []MarkerTagInfo `json:"tags" gorm:"-"`
}

// MarkerLabelTag represents the default tags for a marker label (per user)
type MarkerLabelTag struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	UserID    uint      `gorm:"not null" json:"user_id"`
	Label     string    `gorm:"size:100;not null" json:"label"`
	TagID     uint      `gorm:"not null" json:"tag_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (MarkerLabelTag) TableName() string {
	return "marker_label_tags"
}

// MarkerTag represents a tag on an individual marker
type MarkerTag struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	MarkerID    uint      `gorm:"not null" json:"marker_id"`
	TagID       uint      `gorm:"not null" json:"tag_id"`
	IsFromLabel bool      `gorm:"not null;default:false" json:"is_from_label"`
	CreatedAt   time.Time `json:"created_at"`
}

func (MarkerTag) TableName() string {
	return "marker_tags"
}

// MarkerTagInfo represents a tag with metadata about its source
type MarkerTagInfo struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	IsFromLabel bool   `json:"is_from_label"`
}

// MarkerMove sets the scene and times of a marker: a marker of a duplicate moved to the kept
// scene of its group, its times shifted to where the same moment is in that scene.
type MarkerMove struct {
	ID           uint
	SceneID      uint
	Timestamp    int
	EndTimestamp *int
}

// MarkerDedupTag is a tag added to the kept marker of a MarkerDedupMerge.
type MarkerDedupTag struct {
	TagID       uint
	IsFromLabel bool
}

// MarkerDedupMerge folds duplicate markers of a user into one: the KeepID marker gets Label
// (when not nil) and the AddTags it lacks, and the RemoveIDs markers are deleted.
type MarkerDedupMerge struct {
	KeepID    uint
	Label     *string
	AddTags   []MarkerDedupTag
	RemoveIDs []uint
}
