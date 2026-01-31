package data

import (
	"time"
)

type UserVideoMarker struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	UserID        uint      `gorm:"not null" json:"user_id"`
	VideoID       uint      `gorm:"not null" json:"video_id"`
	Timestamp     int       `gorm:"not null" json:"timestamp"` // seconds
	Label         string    `gorm:"size:100" json:"label"`
	Color         string    `gorm:"size:7;default:'#FFFFFF'" json:"color"`
	ThumbnailPath string    `gorm:"size:255" json:"thumbnail_path"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (UserVideoMarker) TableName() string {
	return "user_video_markers"
}

type MarkerLabelSuggestion struct {
	Label string `json:"label"`
	Count int64  `json:"count"`
}

// MarkerLabelGroup represents a group of markers with the same label
type MarkerLabelGroup struct {
	Label             string `json:"label"`
	Count             int64  `json:"count"`
	ThumbnailMarkerID uint   `json:"thumbnail_marker_id"`
}

// MarkerWithVideo extends UserVideoMarker with video information
type MarkerWithVideo struct {
	UserVideoMarker
	VideoTitle string          `json:"video_title"`
	Tags       []MarkerTagInfo `json:"tags,omitempty" gorm:"-"`
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
