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
	// Kind is what the playlist holds, chosen at creation: PlaylistKindScenes or
	// PlaylistKindMarkers.
	Kind      string    `gorm:"size:20;not null;default:'scenes'" json:"kind"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// BeforeCreate generates a UUID if not set
func (p *Playlist) BeforeCreate(tx *gorm.DB) error {
	if p.UUID == uuid.Nil {
		p.UUID = uuid.New()
	}
	return nil
}

// Playlist kinds: a playlist holds scenes (playlist_scenes) or markers (playlist_markers).
const (
	PlaylistKindScenes  = "scenes"
	PlaylistKindMarkers = "markers"
)

type PlaylistScene struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	PlaylistID uint      `gorm:"not null" json:"playlist_id"`
	SceneID    uint      `gorm:"not null" json:"scene_id"`
	Position   int       `gorm:"not null" json:"position"`
	AddedAt    time.Time `gorm:"not null;default:now()" json:"added_at"`

	Scene Scene `gorm:"foreignKey:SceneID" json:"scene,omitempty"`
}

// PlaylistMarker is a marker in a marker playlist (once per playlist).
type PlaylistMarker struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	PlaylistID uint      `gorm:"not null" json:"playlist_id"`
	MarkerID   uint      `gorm:"not null" json:"marker_id"`
	Position   int       `gorm:"not null" json:"position"`
	AddedAt    time.Time `gorm:"not null;default:now()" json:"added_at"`
}

// PlaylistMarkerRow is a marker of a playlist with what playing it needs from its scene. Only
// markers of scenes neither trashed nor deleted are returned.
type PlaylistMarkerRow struct {
	Position      int
	AddedAt       time.Time
	MarkerID      uint
	MarkerUserID  uint
	SceneID       uint
	Timestamp     int
	EndTimestamp  *int
	Label         string
	Color         string
	ThumbnailPath string
	UpdatedAt     time.Time
	SceneTitle    string
	SceneDuration int
}

// PlaylistMarkerStats sums up the playable markers of a marker playlist.
type PlaylistMarkerStats struct {
	MarkerCount   int64
	SceneCount    int64
	TotalDuration int64
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
	UserID      uint  `gorm:"not null" json:"user_id"`
	PlaylistID  uint  `gorm:"not null" json:"playlist_id"`
	LastSceneID *uint `json:"last_scene_id"`
	// LastMarkerID is the last marker played, in a marker playlist
	LastMarkerID  *uint     `json:"last_marker_id"`
	LastPositionS float64   `gorm:"not null;default:0" json:"last_position_s"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TableName: the table is playlist_progress (GORM would look for playlist_progresses, which
// made the progress calls fail before migration 096)
func (PlaylistProgress) TableName() string {
	return "playlist_progress"
}

// PlaylistListParams holds query params for listing playlists
type PlaylistListParams struct {
	UserID     uint
	Owner      string // "me" or "all"
	Visibility string // "public", "private", or "" (all)
	Kind       string // PlaylistKindScenes, PlaylistKindMarkers, or "" (all)
	TagIDs     []uint
	Search     string // name ILIKE search
	Sort       string
	Page       int
	Limit      int
}
