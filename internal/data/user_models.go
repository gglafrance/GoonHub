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
	// ShareMarkers makes the user's markers visible (read-only) to every user.
	ShareMarkers bool `gorm:"not null;default:false" json:"share_markers"`
}

// Role is a named set of permission settings (docs/RBAC.md). It inherits the permissions of
// its parent roles (role_parents). A superuser role has every permission; a system role cannot
// be deleted.
type Role struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Name        string    `gorm:"uniqueIndex;not null;size:50" json:"name"`
	Description string    `gorm:"size:255" json:"description"`
	IsSystem    bool      `gorm:"not null;default:false" json:"is_system"`
	IsSuperuser bool      `gorm:"not null;default:false" json:"is_superuser"`
}

// Permission is a named capability, synced from the catalog (core/rbac_catalog.go).
type Permission struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	Name        string    `gorm:"uniqueIndex;not null;size:100" json:"name"`
	Description string    `gorm:"size:255" json:"description"`
	GroupName   string    `gorm:"column:group_name;not null;default:''" json:"group"`
}

// Permission setting effects (role settings and user overrides). No row means "inherit".
const (
	PermissionEffectAllow = "allow"
	PermissionEffectDeny  = "deny"
)

// RolePermission is a role's own setting for a permission.
type RolePermission struct {
	ID           uint   `gorm:"primarykey"`
	RoleID       uint   `gorm:"not null"`
	PermissionID uint   `gorm:"not null"`
	Effect       string `gorm:"not null;default:'allow'"`
}

// RoleParent makes RoleID inherit the permissions of ParentID.
type RoleParent struct {
	RoleID    uint      `gorm:"primaryKey;autoIncrement:false"`
	ParentID  uint      `gorm:"primaryKey;autoIncrement:false"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

func (RoleParent) TableName() string {
	return "role_parents"
}

// UserPermissionOverride is a per-user setting: it beats the role's effective value.
type UserPermissionOverride struct {
	UserID       uint      `gorm:"primaryKey;autoIncrement:false"`
	PermissionID uint      `gorm:"primaryKey;autoIncrement:false"`
	Effect       string    `gorm:"not null"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
}

func (UserPermissionOverride) TableName() string {
	return "user_permission_overrides"
}

type RevokedToken struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	TokenHash string    `gorm:"uniqueIndex;not null;size:64" json:"-"`
	ExpiresAt time.Time `gorm:"not null;index" json:"expires_at"`
	Reason    string    `gorm:"size:255" json:"reason,omitempty"`
}

type UserSettings struct {
	ID                       uint                 `gorm:"primarykey" json:"id"`
	CreatedAt                time.Time            `json:"created_at"`
	UpdatedAt                time.Time            `json:"updated_at"`
	UserID                   uint                 `gorm:"uniqueIndex;not null" json:"user_id"`
	Autoplay                 bool                 `gorm:"not null;default:false" json:"autoplay"`
	DefaultVolume            int                  `gorm:"not null;default:100" json:"default_volume"`
	Loop                     bool                 `gorm:"not null;default:false" json:"loop"`
	AbLoopControls           bool                 `gorm:"not null;default:false" json:"ab_loop_controls"`
	MarkerLoop               bool                 `gorm:"not null;default:false" json:"marker_loop"`
	VideosPerPage            int                  `gorm:"not null;default:20" json:"videos_per_page"`
	DefaultSortOrder         string               `gorm:"not null;default:'created_at_desc'" json:"default_sort_order"`
	DefaultTagSort           string               `gorm:"not null;default:'az'" json:"default_tag_sort"`
	MarkerThumbnailCycling   bool                 `gorm:"not null;default:true" json:"marker_thumbnail_cycling"`
	HomepageConfig           HomepageConfig       `gorm:"type:jsonb;not null" json:"homepage_config"`
	ParsingRules             ParsingRulesSettings `gorm:"type:jsonb;not null" json:"parsing_rules"`
	SortPreferences          SortPreferences      `gorm:"type:jsonb;not null" json:"sort_preferences"`
	PlaylistAutoAdvance      string               `gorm:"not null;default:'countdown'" json:"playlist_auto_advance"`
	PlaylistCountdownSeconds int                  `gorm:"not null;default:5" json:"playlist_countdown_seconds"`
	ShowPageSizeSelector     bool                 `gorm:"not null;default:false" json:"show_page_size_selector"`
	AdvancedMarkerFilters    bool                 `gorm:"not null;default:false" json:"advanced_marker_filters"`
	DefaultMarkerView        string               `gorm:"size:10;not null;default:'grouped'" json:"default_marker_view"`
	// PaginationPosition: where page controls are shown on paginated pages (top, bottom, both)
	PaginationPosition string `gorm:"size:10;not null;default:'bottom'" json:"pagination_position"`
	// DateFormat: how dates are displayed (auto = browser locale, or a pattern like DD/MM/YYYY)
	DateFormat string `gorm:"size:20;not null;default:'auto'" json:"date_format"`
	// PlayerGestures: touch gestures on the video player (double-tap seek, swipes, long-press 2x).
	// No gorm default so that false is written when the row is created (the column defaults to true).
	PlayerGestures bool `gorm:"not null" json:"player_gestures"`
	// GestureSeekStep: seconds skipped by a double-tap on the sides of the player (5, 10, 15, 30)
	GestureSeekStep int `gorm:"not null;default:10" json:"gesture_seek_step"`
	// ShowReplayHeatmap / RecordReplayHeatmap: "most replayed" heatmap above the player
	// progress bar, and recording of the watched parts (both on by default)
	ShowReplayHeatmap   bool            `gorm:"not null;default:true" json:"show_replay_heatmap"`
	RecordReplayHeatmap bool            `gorm:"not null;default:true" json:"record_replay_heatmap"`
	SceneCardConfig     SceneCardConfig `gorm:"type:jsonb;not null" json:"scene_card_config"`
	// GridDensity: size of the cards in the scene grids (compact, comfortable, large)
	GridDensity string `gorm:"size:12;not null;default:'comfortable'" json:"grid_density"`
	// InfiniteScroll: scene list pages load the next page on scroll instead of page controls
	InfiniteScroll bool `gorm:"not null;default:false" json:"infinite_scroll"`
	// SceneListView: grid or table per scene list page, and the table columns
	SceneListView SceneListView `gorm:"type:jsonb;not null" json:"scene_list_view"`
	// ShowScrubber: strip of the scene thumbnails under the player on the watch page (off by default)
	ShowScrubber bool `gorm:"not null;default:false" json:"show_scrubber"`
	// ScrubberMode: how the scrubber strip behaves, free (scrolled on its own, click to seek) or
	// centered (like Stash: the centre line is the playback position, moving the strip seeks)
	ScrubberMode string `gorm:"size:10;not null;default:'free'" json:"scrubber_mode"`
	// ContentWidth: maximum width of the scene list pages, default (today's width), wide or full
	// (edge to edge); the grids get more columns when it is wider
	ContentWidth    string `gorm:"size:10;not null;default:'default'" json:"content_width"`
	MaxItemsPerPage int    `gorm:"-" json:"max_items_per_page"`
}

// HomepageConfig represents the user's homepage layout configuration
type HomepageConfig struct {
	ShowUpload bool `json:"show_upload"`
	// Sections is the legacy single-page layout, migrated into a first tab by Normalize.
	Sections []HomepageSection `json:"sections"`
	// Tabs of the homepage, each with its own sections.
	Tabs []HomepageTab `json:"tabs"`
	// DefaultTabID is the tab opened when OpenOn is "default" (or no tab was visited yet).
	DefaultTabID string `json:"default_tab_id"`
	// OpenOn: "last" (last visited tab, kept by the browser) or "default".
	OpenOn string `json:"open_on"`
}

// HomepageTab is a tab of the homepage.
type HomepageTab struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Icon     string            `json:"icon"`
	Sections []HomepageSection `json:"sections"`
}

// Normalize migrates the legacy sections into a first "Home" tab and fills defaults.
func (h *HomepageConfig) Normalize() {
	if len(h.Tabs) == 0 {
		sections := h.Sections
		if sections == nil {
			sections = []HomepageSection{}
		}
		h.Tabs = []HomepageTab{{ID: "home", Name: "Home", Icon: "heroicons:home", Sections: sections}}
	}
	h.Sections = []HomepageSection{}
	for i := range h.Tabs {
		if h.Tabs[i].Sections == nil {
			h.Tabs[i].Sections = []HomepageSection{}
		}
	}
	if h.FindTab(h.DefaultTabID) == nil {
		h.DefaultTabID = h.Tabs[0].ID
	}
	if h.OpenOn != "last" && h.OpenOn != "default" {
		h.OpenOn = "last"
	}
}

// FindTab returns the tab with the given ID, or nil.
func (h *HomepageConfig) FindTab(id string) *HomepageTab {
	for i := range h.Tabs {
		if h.Tabs[i].ID == id {
			return &h.Tabs[i]
		}
	}
	return nil
}

// FindSection returns the section with the given ID in any tab (or the legacy list), or nil.
func (h *HomepageConfig) FindSection(id string) *HomepageSection {
	for i := range h.Tabs {
		for j := range h.Tabs[i].Sections {
			if h.Tabs[i].Sections[j].ID == id {
				return &h.Tabs[i].Sections[j]
			}
		}
	}
	for i := range h.Sections {
		if h.Sections[i].ID == id {
			return &h.Sections[i]
		}
	}
	return nil
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

	// Migrate legacy sections into tabs; Sections then serializes as []
	h.Normalize()

	return nil
}

// DefaultHomepageConfig returns the default homepage configuration
func DefaultHomepageConfig() HomepageConfig {
	config := HomepageConfig{
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
	config.Normalize()
	return config
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
	// Columns lays the items out in a grid of that many columns (1 to MaxBadgeZoneColumns),
	// filled row by row (horizontal) or column by column (vertical). 0 (absent) keeps the
	// original layout: one column (vertical) or one row (horizontal).
	Columns int `json:"columns,omitempty"`
}

// Limits of a badge zone of the scene card template.
const (
	MaxBadgeZoneItems   = 12
	MaxBadgeZoneColumns = 4
)

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

// Scene list view modes.
const (
	SceneListViewGrid  = "grid"
	SceneListViewTable = "table"
)

// SceneListView is how the scene list pages display their scenes.
type SceneListView struct {
	// Pages maps a page type (search, actor, studio, playlist, explorer) to its view mode
	// (grid or table). A missing page uses the grid.
	Pages map[string]string `json:"pages"`
	// Columns are the visible table columns, in display order.
	Columns []string `json:"columns"`
}

// DefaultSceneListViewColumns are the table columns shown by default.
func DefaultSceneListViewColumns() []string {
	return []string{"thumbnail", "title", "duration", "resolution", "file_size", "rating", "views", "studio", "actors", "added_at"}
}

// DefaultSceneListView returns the default scene list view: grids everywhere.
func DefaultSceneListView() SceneListView {
	return SceneListView{Pages: map[string]string{}, Columns: DefaultSceneListViewColumns()}
}

// Normalize fills the missing parts with their defaults.
func (v *SceneListView) Normalize() {
	if v.Pages == nil {
		v.Pages = map[string]string{}
	}
	if len(v.Columns) == 0 {
		v.Columns = DefaultSceneListViewColumns()
	}
}

// Value implements the driver.Valuer interface for JSONB storage
func (v SceneListView) Value() (driver.Value, error) {
	v.Normalize()
	return json.Marshal(v)
}

// Scan implements the sql.Scanner interface for JSONB retrieval
func (v *SceneListView) Scan(value any) error {
	if value == nil {
		*v = DefaultSceneListView()
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("failed to scan SceneListView: expected []byte")
	}

	*v = SceneListView{}
	if err := json.Unmarshal(bytes, v); err != nil {
		return err
	}
	v.Normalize()
	return nil
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
