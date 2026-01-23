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

type UserSettings struct {
	ID               uint      `gorm:"primarykey" json:"id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	UserID           uint      `gorm:"uniqueIndex;not null" json:"user_id"`
	Autoplay         bool      `gorm:"not null;default:false" json:"autoplay"`
	DefaultVolume    int       `gorm:"not null;default:100" json:"default_volume"`
	Loop             bool      `gorm:"not null;default:false" json:"loop"`
	VideosPerPage    int       `gorm:"not null;default:20" json:"videos_per_page"`
	DefaultSortOrder string    `gorm:"not null;default:'created_at_desc'" json:"default_sort_order"`
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
	SpriteSheetPath  string         `json:"sprite_sheet_path"`
	VttPath          string         `json:"vtt_path"`
	SpriteSheetCount int            `json:"sprite_sheet_count"`
	ThumbnailWidth   int            `json:"thumbnail_width"`
	ThumbnailHeight  int            `json:"thumbnail_height"`
	ProcessingStatus string         `json:"processing_status" gorm:"default:'pending'"`
	ProcessingError  string         `json:"processing_error" gorm:"type:text"`
}
