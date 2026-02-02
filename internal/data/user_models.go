package data

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
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
	ID               uint           `gorm:"primarykey" json:"id"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	UserID           uint           `gorm:"uniqueIndex;not null" json:"user_id"`
	Autoplay         bool           `gorm:"not null;default:false" json:"autoplay"`
	DefaultVolume    int            `gorm:"not null;default:100" json:"default_volume"`
	Loop             bool           `gorm:"not null;default:false" json:"loop"`
	VideosPerPage    int            `gorm:"not null;default:20" json:"videos_per_page"`
	DefaultSortOrder string         `gorm:"not null;default:'created_at_desc'" json:"default_sort_order"`
	DefaultTagSort          string         `gorm:"not null;default:'az'" json:"default_tag_sort"`
	MarkerThumbnailCycling  bool           `gorm:"not null;default:true" json:"marker_thumbnail_cycling"`
	HomepageConfig          HomepageConfig `gorm:"type:jsonb;not null" json:"homepage_config"`
}

// HomepageConfig represents the user's homepage layout configuration
type HomepageConfig struct {
	ShowUpload bool              `json:"show_upload"`
	Sections   []HomepageSection `json:"sections"`
}

// HomepageSection represents a single section on the homepage
type HomepageSection struct {
	ID      string                 `json:"id"`
	Type    string                 `json:"type"`
	Title   string                 `json:"title"`
	Enabled bool                   `json:"enabled"`
	Limit   int                    `json:"limit"`
	Order   int                    `json:"order"`
	Sort    string                 `json:"sort"`
	Config  map[string]interface{} `json:"config"`
}

// Value implements the driver.Valuer interface for JSONB storage
func (h HomepageConfig) Value() (driver.Value, error) {
	return json.Marshal(h)
}

// Scan implements the sql.Scanner interface for JSONB retrieval
func (h *HomepageConfig) Scan(value any) error {
	if value == nil {
		*h = DefaultHomepageConfig()
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("failed to scan HomepageConfig: expected []byte")
	}

	return json.Unmarshal(bytes, h)
}

// DefaultHomepageConfig returns the default homepage configuration
func DefaultHomepageConfig() HomepageConfig {
	return HomepageConfig{
		ShowUpload: true,
		Sections: []HomepageSection{
			{
				ID:      "default-latest",
				Type:    "latest",
				Title:   "Latest Uploads",
				Enabled: true,
				Limit:   12,
				Order:   0,
				Sort:    "created_at_desc",
				Config:  map[string]interface{}{},
			},
		},
	}
}
