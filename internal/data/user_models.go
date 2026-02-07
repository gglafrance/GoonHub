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
	MarkerThumbnailCycling     bool                 `gorm:"not null;default:true" json:"marker_thumbnail_cycling"`
	HomepageConfig             HomepageConfig       `gorm:"type:jsonb;not null" json:"homepage_config"`
	ParsingRules               ParsingRulesSettings `gorm:"type:jsonb;not null" json:"parsing_rules"`
	SortPreferences            SortPreferences      `gorm:"type:jsonb;not null" json:"sort_preferences"`
	PlaylistAutoAdvance        string               `gorm:"not null;default:'countdown'" json:"playlist_auto_advance"`
	PlaylistCountdownSeconds   int                  `gorm:"not null;default:5" json:"playlist_countdown_seconds"`
	ShowPageSizeSelector       bool                 `gorm:"not null;default:false" json:"show_page_size_selector"`
	SceneCardConfig            SceneCardConfig      `gorm:"type:jsonb;not null" json:"scene_card_config"`
	MaxItemsPerPage            int                  `gorm:"-" json:"max_items_per_page"`
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

	if err := json.Unmarshal(bytes, h); err != nil {
		return err
	}

	// Ensure Sections is never nil so it serializes as [] instead of null
	if h.Sections == nil {
		h.Sections = []HomepageSection{}
	}

	return nil
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

// ParsingRulesSettings represents the user's parsing rules configuration
type ParsingRulesSettings struct {
	Presets        []ParsingPreset `json:"presets"`
	ActivePresetID *string         `json:"activePresetId"`
}

// ParsingPreset represents a saved set of parsing rules
type ParsingPreset struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	IsBuiltIn bool          `json:"isBuiltIn"`
	Rules     []ParsingRule `json:"rules"`
}

// ParsingRule represents a single filename parsing rule
type ParsingRule struct {
	ID      string            `json:"id"`
	Type    string            `json:"type"`
	Enabled bool              `json:"enabled"`
	Order   int               `json:"order"`
	Config  ParsingRuleConfig `json:"config"`
}

// ParsingRuleConfig holds configuration for specific rule types
type ParsingRuleConfig struct {
	KeepContent   bool   `json:"keepContent,omitempty"`   // remove_brackets: keep content inside brackets
	Pattern       string `json:"pattern,omitempty"`       // regex_remove
	Find          string `json:"find,omitempty"`          // text_replace
	Replace       string `json:"replace,omitempty"`       // text_replace
	CaseSensitive bool   `json:"caseSensitive,omitempty"` // text_replace
	MinLength     int    `json:"minLength,omitempty"`     // word_length_filter
	CaseType      string `json:"caseType,omitempty"`      // case_normalize
}

// Value implements the driver.Valuer interface for JSONB storage
func (p ParsingRulesSettings) Value() (driver.Value, error) {
	return json.Marshal(p)
}

// Scan implements the sql.Scanner interface for JSONB retrieval
func (p *ParsingRulesSettings) Scan(value any) error {
	if value == nil {
		*p = DefaultParsingRulesSettings()
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("failed to scan ParsingRulesSettings: expected []byte")
	}

	if err := json.Unmarshal(bytes, p); err != nil {
		return err
	}

	// Ensure Presets is never nil so it serializes as [] instead of null
	if p.Presets == nil {
		p.Presets = []ParsingPreset{}
	}

	return nil
}

// DefaultParsingRulesSettings returns the default parsing rules configuration
func DefaultParsingRulesSettings() ParsingRulesSettings {
	return ParsingRulesSettings{
		Presets:        []ParsingPreset{},
		ActivePresetID: nil,
	}
}

// SortPreferences represents user-configurable default sort orders per page
type SortPreferences struct {
	Actors       string `json:"actors"`
	Studios      string `json:"studios"`
	Markers      string `json:"markers"`
	ActorScenes  string `json:"actor_scenes"`
	StudioScenes string `json:"studio_scenes"`
}

// Value implements the driver.Valuer interface for JSONB storage
func (s SortPreferences) Value() (driver.Value, error) {
	return json.Marshal(s)
}

// Scan implements the sql.Scanner interface for JSONB retrieval
func (s *SortPreferences) Scan(value any) error {
	if value == nil {
		*s = DefaultSortPreferences()
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("failed to scan SortPreferences: expected []byte")
	}

	return json.Unmarshal(bytes, s)
}

// SceneCardConfig represents the user's scene card template configuration
type SceneCardConfig struct {
	Badges      BadgeZones   `json:"badges"`
	ContentRows []ContentRow `json:"content_rows"`
}

// BadgeZones represents the 4 corner badge zones on the thumbnail overlay
type BadgeZones struct {
	TopLeft     BadgeZone `json:"top_left"`
	TopRight    BadgeZone `json:"top_right"`
	BottomLeft  BadgeZone `json:"bottom_left"`
	BottomRight BadgeZone `json:"bottom_right"`
}

// BadgeZone represents a single corner badge zone
type BadgeZone struct {
	Items     []string `json:"items"`
	Direction string   `json:"direction"`
}

// ContentRow represents a single content row below the title
type ContentRow struct {
	Type      string `json:"type"`
	Field     string `json:"field,omitempty"`
	Mode      string `json:"mode,omitempty"`
	Left      string `json:"left,omitempty"`
	Right     string `json:"right,omitempty"`
	LeftMode  string `json:"left_mode,omitempty"`
	RightMode string `json:"right_mode,omitempty"`
}

// Value implements the driver.Valuer interface for JSONB storage
func (s SceneCardConfig) Value() (driver.Value, error) {
	return json.Marshal(s)
}

// Scan implements the sql.Scanner interface for JSONB retrieval
func (s *SceneCardConfig) Scan(value any) error {
	if value == nil {
		*s = DefaultSceneCardConfig()
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("failed to scan SceneCardConfig: expected []byte")
	}

	if err := json.Unmarshal(bytes, s); err != nil {
		return err
	}

	// Ensure slices are never nil so they serialize as [] instead of null
	if s.ContentRows == nil {
		s.ContentRows = []ContentRow{}
	}
	if s.Badges.TopLeft.Items == nil {
		s.Badges.TopLeft.Items = []string{}
	}
	if s.Badges.TopRight.Items == nil {
		s.Badges.TopRight.Items = []string{}
	}
	if s.Badges.BottomLeft.Items == nil {
		s.Badges.BottomLeft.Items = []string{}
	}
	if s.Badges.BottomRight.Items == nil {
		s.Badges.BottomRight.Items = []string{}
	}

	return nil
}

// DefaultSceneCardConfig returns the default scene card configuration
func DefaultSceneCardConfig() SceneCardConfig {
	return SceneCardConfig{
		Badges: BadgeZones{
			TopLeft:     BadgeZone{Items: []string{"rating"}, Direction: "vertical"},
			TopRight:    BadgeZone{Items: []string{"watched"}, Direction: "vertical"},
			BottomLeft:  BadgeZone{Items: []string{}, Direction: "vertical"},
			BottomRight: BadgeZone{Items: []string{"duration"}, Direction: "horizontal"},
		},
		ContentRows: []ContentRow{
			{Type: "split", Left: "file_size", Right: "added_at"},
		},
	}
}

// DefaultSortPreferences returns the default sort preferences
func DefaultSortPreferences() SortPreferences {
	return SortPreferences{
		Actors:       "name_asc",
		Studios:      "name_asc",
		Markers:      "label_asc",
		ActorScenes:  "",
		StudioScenes: "",
	}
}
