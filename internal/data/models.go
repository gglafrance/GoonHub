package data

import (
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

type User struct {
	ID          uint       `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Username    string     `gorm:"uniqueIndex;not null" json:"username"`
	Password    string     `gorm:"not null" json:"-"`
	Role        string     `gorm:"not null;default:'user'" json:"role"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

type Role struct {
	ID          uint         `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	Name        string       `gorm:"uniqueIndex;not null;size:50" json:"name"`
	Description string       `gorm:"size:255" json:"description"`
	Permissions []Permission `gorm:"many2many:role_permissions" json:"permissions,omitempty"`
}

type Permission struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	Name        string    `gorm:"uniqueIndex;not null;size:100" json:"name"`
	Description string    `gorm:"size:255" json:"description"`
}

type RolePermission struct {
	ID           uint `gorm:"primarykey"`
	RoleID       uint `gorm:"not null"`
	PermissionID uint `gorm:"not null"`
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

type JobHistory struct {
	ID           uint       `gorm:"primarykey" json:"id"`
	JobID        string     `gorm:"uniqueIndex;not null;size:36" json:"job_id"`
	VideoID      uint       `gorm:"not null" json:"video_id"`
	VideoTitle   string     `gorm:"not null;size:255;default:''" json:"video_title"`
	Phase        string     `gorm:"not null;size:20" json:"phase"`
	Status       string     `gorm:"not null;size:20;default:'running'" json:"status"`
	ErrorMessage *string    `gorm:"type:text" json:"error_message,omitempty"`
	StartedAt    time.Time  `gorm:"not null;default:now()" json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	CreatedAt    time.Time  `gorm:"not null;default:now()" json:"created_at"`
}

func (JobHistory) TableName() string {
	return "job_history"
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
}
