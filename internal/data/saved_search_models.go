package data

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SavedSearch struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	UUID      uuid.UUID `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	UserID    uint      `gorm:"not null" json:"user_id"`
	Name      string    `gorm:"size:255;not null" json:"name"`
	Filters   Filters   `gorm:"type:jsonb;not null;default:'{}'" json:"filters"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate generates a UUID if not set
func (s *SavedSearch) BeforeCreate(tx *gorm.DB) error {
	if s.UUID == uuid.Nil {
		s.UUID = uuid.New()
	}
	return nil
}

// Filters represents the saved search filter parameters
type Filters struct {
	Query          string   `json:"query,omitempty"`
	MatchType      string   `json:"match_type,omitempty"`
	SelectedTags   []string `json:"selected_tags,omitempty"`
	SelectedActors []string `json:"selected_actors,omitempty"`
	Studio         string   `json:"studio,omitempty"`
	Resolution     string   `json:"resolution,omitempty"`
	MinDuration    *int     `json:"min_duration,omitempty"`
	MaxDuration    *int     `json:"max_duration,omitempty"`
	MinDate        string   `json:"min_date,omitempty"`
	MaxDate        string   `json:"max_date,omitempty"`
	Liked          *bool    `json:"liked,omitempty"`
	MinRating      *float64 `json:"min_rating,omitempty"`
	MaxRating      *float64 `json:"max_rating,omitempty"`
	MinJizzCount   *int     `json:"min_jizz_count,omitempty"`
	MaxJizzCount   *int     `json:"max_jizz_count,omitempty"`
	Sort           string   `json:"sort,omitempty"`
}

// Value implements the driver.Valuer interface for JSONB storage
func (f Filters) Value() (driver.Value, error) {
	return json.Marshal(f)
}

// Scan implements the sql.Scanner interface for JSONB retrieval
func (f *Filters) Scan(value any) error {
	if value == nil {
		*f = Filters{}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("failed to scan Filters: expected []byte")
	}

	return json.Unmarshal(bytes, f)
}
