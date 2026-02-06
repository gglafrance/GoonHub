package data

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Playlist struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	UUID        uuid.UUID `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	UserID      uint      `gorm:"not null" json:"user_id"`
	Name        string    `gorm:"size:255;not null" json:"name"`
	Description *string   `gorm:"type:text" json:"description"`
	Visibility  string    `gorm:"size:20;not null;default:'private'" json:"visibility"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// BeforeCreate generates a UUID if not set
func (p *Playlist) BeforeCreate(tx *gorm.DB) error {
	if p.UUID == uuid.Nil {
		p.UUID = uuid.New()
	}
	return nil
}

type PlaylistScene struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	PlaylistID uint      `gorm:"not null" json:"playlist_id"`
	SceneID    uint      `gorm:"not null" json:"scene_id"`
	Position   int       `gorm:"not null" json:"position"`
	AddedAt    time.Time `gorm:"not null;default:now()" json:"added_at"`

	Scene Scene `gorm:"foreignKey:SceneID" json:"scene,omitempty"`
}

type PlaylistTag struct {
	PlaylistID uint `gorm:"not null" json:"playlist_id"`
	TagID      uint `gorm:"not null" json:"tag_id"`

	Tag Tag `gorm:"foreignKey:TagID" json:"tag,omitempty"`
}

type PlaylistLike struct {
	UserID     uint      `gorm:"not null" json:"user_id"`
	PlaylistID uint      `gorm:"not null" json:"playlist_id"`
	CreatedAt  time.Time `json:"created_at"`
}

type PlaylistProgress struct {
	UserID       uint      `gorm:"not null" json:"user_id"`
	PlaylistID   uint      `gorm:"not null" json:"playlist_id"`
	LastSceneID  *uint     `json:"last_scene_id"`
	LastPositionS float64  `gorm:"not null;default:0" json:"last_position_s"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// PlaylistListParams holds query params for listing playlists
type PlaylistListParams struct {
	UserID     uint
	Owner      string // "me" or "all"
	Visibility string // "public", "private", or "" (all)
	TagIDs     []uint
	Sort       string
	Page       int
	Limit      int
}
