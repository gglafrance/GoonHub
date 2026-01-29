package data

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Studio struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	UUID      uuid.UUID      `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Name        string   `gorm:"size:255;not null" json:"name"`
	ShortName   string   `gorm:"size:100" json:"short_name"`
	URL         string   `gorm:"size:512" json:"url"`
	Description string   `gorm:"type:text" json:"description"`
	Rating      *float64 `gorm:"type:decimal(3,1)" json:"rating"`

	Logo    string `gorm:"size:512" json:"logo"`
	Favicon string `gorm:"size:512" json:"favicon"`
	Poster  string `gorm:"size:512" json:"poster"`

	PornDBID  string `gorm:"column:porndb_id;size:100" json:"porndb_id"`
	ParentID  *uint  `json:"parent_id"`
	NetworkID *uint  `json:"network_id"`
}

// BeforeCreate generates a UUID if not set
func (s *Studio) BeforeCreate(tx *gorm.DB) error {
	if s.UUID == uuid.Nil {
		s.UUID = uuid.New()
	}
	return nil
}

type StudioWithCount struct {
	Studio
	VideoCount int64 `json:"video_count"`
}

// Studio interaction models

type UserStudioRating struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	UserID    uint      `gorm:"not null" json:"user_id"`
	StudioID  uint      `gorm:"not null" json:"studio_id"`
	Rating    float64   `gorm:"type:decimal(2,1);not null" json:"rating"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UserStudioLike struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	UserID    uint      `gorm:"not null" json:"user_id"`
	StudioID  uint      `gorm:"not null" json:"studio_id"`
	CreatedAt time.Time `json:"created_at"`
}

// StudioInteractions holds all interaction data for a studio
type StudioInteractions struct {
	Rating float64 `json:"rating"`
	Liked  bool    `json:"liked"`
}
