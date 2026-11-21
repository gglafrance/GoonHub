package data

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SavedMarkerFilter is a named set of marker page filters (advanced marker filters).
type SavedMarkerFilter struct {
	ID        uint               `gorm:"primarykey" json:"id"`
	UUID      uuid.UUID          `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	UserID    uint               `gorm:"not null" json:"user_id"`
	Name      string             `gorm:"size:100;not null" json:"name"`
	Filters   MarkerFilterValues `gorm:"type:jsonb;not null;default:'{}'" json:"filters"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
}

func (SavedMarkerFilter) TableName() string {
	return "saved_marker_filters"
}

// BeforeCreate generates a UUID if not set.
func (f *SavedMarkerFilter) BeforeCreate(tx *gorm.DB) error {
	if f.UUID == uuid.Nil {
		f.UUID = uuid.New()
	}
	return nil
}

// MarkerFilterValues are the marker page filters stored by a saved marker filter.
type MarkerFilterValues struct {
	Labels     []string `json:"labels,omitempty"`
	MarkerTags []string `json:"marker_tags,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Actors     []string `json:"actors,omitempty"`
	Studio     string   `json:"studio,omitempty"`
	RangesOnly bool     `json:"ranges_only,omitempty"`
	Sort       string   `json:"sort,omitempty"`
	// View of the markers index page: grouped or all
	View string `json:"view,omitempty"`
}

// Value implements driver.Valuer for JSONB storage.
func (v MarkerFilterValues) Value() (driver.Value, error) {
	return json.Marshal(v)
}

// Scan implements sql.Scanner for JSONB retrieval.
func (v *MarkerFilterValues) Scan(value any) error {
	if value == nil {
		*v = MarkerFilterValues{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("failed to scan MarkerFilterValues: expected []byte")
	}
	return json.Unmarshal(bytes, v)
}

// SavedMarkerFilterRepository persists saved marker filters.
type SavedMarkerFilterRepository interface {
	Create(filter *SavedMarkerFilter) error
	GetByUUID(uuid string) (*SavedMarkerFilter, error)
	Update(filter *SavedMarkerFilter) error
	Delete(id uint) error
	ListByUserID(userID uint) ([]SavedMarkerFilter, error)
}

type SavedMarkerFilterRepositoryImpl struct {
	DB *gorm.DB
}

func NewSavedMarkerFilterRepository(db *gorm.DB) *SavedMarkerFilterRepositoryImpl {
	return &SavedMarkerFilterRepositoryImpl{DB: db}
}

func (r *SavedMarkerFilterRepositoryImpl) Create(filter *SavedMarkerFilter) error {
	return r.DB.Create(filter).Error
}

func (r *SavedMarkerFilterRepositoryImpl) GetByUUID(id string) (*SavedMarkerFilter, error) {
	var filter SavedMarkerFilter
	if err := r.DB.Where("uuid = ?", id).First(&filter).Error; err != nil {
		return nil, err
	}
	return &filter, nil
}

func (r *SavedMarkerFilterRepositoryImpl) Update(filter *SavedMarkerFilter) error {
	return r.DB.Save(filter).Error
}

func (r *SavedMarkerFilterRepositoryImpl) Delete(id uint) error {
	return r.DB.Delete(&SavedMarkerFilter{}, id).Error
}

func (r *SavedMarkerFilterRepositoryImpl) ListByUserID(userID uint) ([]SavedMarkerFilter, error) {
	var filters []SavedMarkerFilter
	if err := r.DB.Where("user_id = ?", userID).Order("name ASC").Find(&filters).Error; err != nil {
		return nil, err
	}
	return filters, nil
}
