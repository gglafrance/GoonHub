package data

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Username  string    `gorm:"uniqueIndex;not null" json:"username"`
	Password  string    `gorm:"not null" json:"-"`
	Role      string    `gorm:"not null;default:'user'" json:"role"`
}

type RevokedToken struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	TokenHash string    `gorm:"uniqueIndex;not null;size:64" json:"-"`
	ExpiresAt time.Time `gorm:"not null;index" json:"expires_at"`
	Reason    string    `gorm:"size:255" json:"reason,omitempty"`
}

type Video struct {
	ID               uint           `gorm:"primarykey" json:"id"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
	Title            string         `json:"title"`
	OriginalFilename string         `json:"original_filename"`
	StoredPath       string         `json:"-"` // Don't expose internal path
	Size             int64          `json:"size"`
	ViewCount        int64          `json:"view_count"`
	Duration         int            `json:"duration"`
	Width            int            `json:"width"`
	Height           int            `json:"height"`
	ThumbnailPath    string         `json:"thumbnail_path"`
	FramePaths       string         `json:"frame_paths" gorm:"type:text"`
	FrameCount       int            `json:"frame_count"`
	FrameInterval    int            `json:"frame_interval"`
	ProcessingStatus string         `json:"processing_status" gorm:"default:'pending'"`
	ProcessingError  string         `json:"processing_error" gorm:"type:text"`
}
