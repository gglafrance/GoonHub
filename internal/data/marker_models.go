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
	VideoTitle string `json:"video_title"`
}
