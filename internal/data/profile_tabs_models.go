package data

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// Tabs, sections and custom lists of a profile (docs/PROFILES.md, migration 103).

// Tab kinds: the main profile tab, the built-in list tabs and the owner's custom tabs.
const (
	ProfileTabProfile = "profile"
	ProfileTabLikes   = "likes"
	ProfileTabRatings = "ratings"
	ProfileTabJizz    = "jizz"
	ProfileTabCustom  = "custom"
)

// ProfileSection is a row of a tab: a column template and the blocks of each column.
type ProfileSection struct {
	ID       string                 `json:"id"`
	Template string                 `json:"template"`
	Columns  [][]ProfileBlockConfig `json:"columns"`
}

// ProfileTab is a tab of a profile. The profile and custom tabs have sections; the list tabs
// (likes, ratings, jizz) show their list.
type ProfileTab struct {
	ID       string           `json:"id"`
	Kind     string           `json:"kind"`
	Title    string           `json:"title,omitempty"`
	Icon     string           `json:"icon,omitempty"`
	Slug     string           `json:"slug,omitempty"`
	Visible  bool             `json:"visible"`
	Sections []ProfileSection `json:"sections,omitempty"`
}

// ProfileTabs are the ordered tabs of a profile. nil = derived from the older layout.
type ProfileTabs []ProfileTab

// Value stores the tabs as JSONB (NULL when nil).
func (t ProfileTabs) Value() (driver.Value, error) {
	if t == nil {
		return nil, nil
	}
	b, err := json.Marshal([]ProfileTab(t))
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan reads the JSONB tabs (NULL gives nil).
func (t *ProfileTabs) Scan(value any) error {
	if value == nil {
		*t = nil
		return nil
	}
	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("profile tabs: unsupported type %T", value)
	}
	var tabs []ProfileTab
	if err := json.Unmarshal(raw, &tabs); err != nil {
		return fmt.Errorf("profile tabs: %w", err)
	}
	if tabs == nil {
		tabs = []ProfileTab{}
	}
	*t = tabs
	return nil
}

// ProfileList is a custom list of a profile: search criteria (the saved search filters) run
// with the owner's interactions. SavedSearchUUID, when set, is a saved search of the owner
// whose current filters replace Filters while it exists.
type ProfileList struct {
	ID              uint    `gorm:"primaryKey"`
	UserID          uint    `gorm:"not null"`
	Title           string  `gorm:"size:80;not null"`
	Filters         Filters `gorm:"type:jsonb;not null"`
	SavedSearchUUID *string `gorm:"column:saved_search_uuid;type:uuid"`
	Audience        string  `gorm:"size:16;not null"`
	// Figures is which owner figures the cards show: auto, none, jizz, rating, liked, views
	// (migration 105; "" is read as auto)
	Figures   string    `gorm:"size:16;not null;default:auto"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (ProfileList) TableName() string { return "user_profile_lists" }
