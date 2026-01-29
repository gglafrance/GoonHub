package data

import (
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

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
	FileCreatedAt    *time.Time     `json:"file_created_at"`
	Description      string         `json:"description"`
	Studio           string         `json:"studio"`
	Tags             pq.StringArray `json:"tags" gorm:"type:text[]"`
	Actors           pq.StringArray `json:"actors" gorm:"type:text[]"`
	CoverImagePath   string         `json:"cover_image_path"`
	FileHash         string         `json:"file_hash"`
	FrameRate        float64        `json:"frame_rate"`
	BitRate          int64          `json:"bit_rate"`
	VideoCodec       string         `json:"video_codec"`
	AudioCodec       string         `json:"audio_codec"`
	StoragePathID    *uint          `json:"storage_path_id"`
	StudioID         *uint          `json:"studio_id"`
	ReleaseDate      *time.Time     `json:"release_date" gorm:"type:date"`
	PornDBSceneID    string         `json:"porndb_scene_id" gorm:"column:porndb_scene_id"`
}

type Tag struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Name      string    `gorm:"uniqueIndex;not null;size:100" json:"name"`
	Color     string    `gorm:"not null;size:7;default:'#6B7280'" json:"color"`
}

type VideoTag struct {
	ID      uint `gorm:"primarykey"`
	VideoID uint `gorm:"not null"`
	TagID   uint `gorm:"not null"`
}
