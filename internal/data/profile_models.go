package data

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// Banner types of a profile (user_profiles.banner_type).
const (
	ProfileBannerNone   = "none"
	ProfileBannerScene  = "scene"
	ProfileBannerUpload = "upload"
)

// UserProfile is the editable part of a user's profile page (docs/PROFILES.md). No row means
// the defaults (DefaultUserProfile).
type UserProfile struct {
	UserID        uint          `gorm:"primaryKey;autoIncrement:false"`
	DisplayName   string        `gorm:"size:64;not null;default:''"`
	Bio           string        `gorm:"not null;default:''"`
	AvatarVersion int64         `gorm:"not null;default:0"`
	BannerType    string        `gorm:"size:16;not null;default:'none'"`
	BannerSceneID *uint         `gorm:"column:banner_scene_id"`
	BannerVersion int64         `gorm:"not null;default:0"`
	Layout        ProfileLayout `gorm:"type:jsonb"`
	CreatedAt     time.Time     `gorm:"not null;default:now()"`
	UpdatedAt     time.Time     `gorm:"not null;default:now()"`
}

func (UserProfile) TableName() string { return "user_profiles" }

// DefaultUserProfile is the profile of a user who never edited theirs.
func DefaultUserProfile(userID uint) *UserProfile {
	return &UserProfile{UserID: userID, BannerType: ProfileBannerNone}
}

// ProfileBlockConfig is one block of a profile layout: its type, whether it is shown, whether
// viewers who may not see it get a "private" placeholder, and its options.
type ProfileBlockConfig struct {
	Type              string         `json:"type"`
	Visible           bool           `json:"visible"`
	LockedPlaceholder bool           `json:"locked_placeholder"`
	Config            map[string]any `json:"config"`
}

// ProfileLayout is the ordered list of blocks of a profile. nil = the default layout.
type ProfileLayout []ProfileBlockConfig

// Value stores the layout as JSONB (NULL when nil).
func (l ProfileLayout) Value() (driver.Value, error) {
	if l == nil {
		return nil, nil
	}
	b, err := json.Marshal([]ProfileBlockConfig(l))
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan reads the JSONB layout (NULL gives nil).
func (l *ProfileLayout) Scan(value any) error {
	if value == nil {
		*l = nil
		return nil
	}
	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("profile layout: unsupported type %T", value)
	}
	var blocks []ProfileBlockConfig
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return fmt.Errorf("profile layout: %w", err)
	}
	if blocks == nil {
		blocks = []ProfileBlockConfig{}
	}
	*l = blocks
	return nil
}

// ProfileSceneRef is a scene of a profile block with the figure of the block: when it was
// watched or liked, or how many times it was watched.
type ProfileSceneRef struct {
	SceneID uint       `gorm:"column:scene_id"`
	At      *time.Time `gorm:"column:at"`
	Count   int64      `gorm:"column:count"`
}

// ProfileStats are the figures of the stats block, over a period (zero since: all time) for
// the view, watch time and dated o figures; likes, markers and ratings are totals.
type ProfileStats struct {
	WatchTime     int64    `gorm:"column:watch_time"`
	Views         int64    `gorm:"column:views"`
	ScenesWatched int64    `gorm:"column:scenes_watched"`
	Jizz          int64    `gorm:"column:jizz"`
	Liked         int64    `gorm:"column:liked"`
	Markers       int64    `gorm:"column:markers"`
	Rated         int64    `gorm:"column:rated"`
	AvgRating     *float64 `gorm:"column:avg_rating"`
}

// ProfileTopEntry is a tag, actor or studio ranked by the user's views over a period.
type ProfileTopEntry struct {
	ID    uint   `gorm:"column:id" json:"id"`
	UUID  string `gorm:"column:uuid" json:"uuid,omitempty"`
	Name  string `gorm:"column:name" json:"name"`
	Views int64  `gorm:"column:views" json:"views"`
}

// ProfilePlaylist is a public playlist of a user, as shown by the playlists block.
type ProfilePlaylist struct {
	UUID         string    `gorm:"column:uuid"`
	Name         string    `gorm:"column:name"`
	SceneCount   int64     `gorm:"column:scene_count"`
	CoverSceneID *uint     `gorm:"column:cover_scene_id"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}
