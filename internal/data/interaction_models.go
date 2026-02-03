package data

import (
	"time"
)

type UserSceneRating struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	UserID    uint      `gorm:"not null" json:"user_id"`
	SceneID   uint      `gorm:"not null;column:scene_id" json:"scene_id"`
	Rating    float64   `gorm:"type:decimal(2,1);not null" json:"rating"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (UserSceneRating) TableName() string {
	return "user_scene_ratings"
}

type UserSceneLike struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	UserID    uint      `gorm:"not null" json:"user_id"`
	SceneID   uint      `gorm:"not null;column:scene_id" json:"scene_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (UserSceneLike) TableName() string {
	return "user_scene_likes"
}

type UserSceneJizzed struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	UserID    uint      `gorm:"not null" json:"user_id"`
	SceneID   uint      `gorm:"not null;column:scene_id" json:"scene_id"`
	Count     int       `gorm:"not null;default:0" json:"count"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (UserSceneJizzed) TableName() string {
	return "user_scene_jizzed"
}

type UserSceneWatch struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	UserID        uint      `gorm:"not null" json:"user_id"`
	SceneID       uint      `gorm:"not null;column:scene_id" json:"scene_id"`
	WatchedAt     time.Time `gorm:"not null;default:now()" json:"watched_at"`
	WatchDuration int       `gorm:"not null;default:0" json:"watch_duration"`
	LastPosition  int       `gorm:"not null;default:0" json:"last_position"`
	Completed     bool      `gorm:"not null;default:false" json:"completed"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (UserSceneWatch) TableName() string {
	return "user_scene_watches"
}

// Actor interaction models

type UserActorRating struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	UserID    uint      `gorm:"not null" json:"user_id"`
	ActorID   uint      `gorm:"not null" json:"actor_id"`
	Rating    float64   `gorm:"type:decimal(2,1);not null" json:"rating"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UserActorLike struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	UserID    uint      `gorm:"not null" json:"user_id"`
	ActorID   uint      `gorm:"not null" json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}

// ActorInteractions holds all interaction data for an actor
type ActorInteractions struct {
	Rating float64
	Liked  bool
}

// UserSceneViewCount tracks when view counts were last incremented per user+scene
// Used for atomic 24-hour deduplication to prevent race conditions
type UserSceneViewCount struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	UserID        uint      `gorm:"not null" json:"user_id"`
	SceneID       uint      `gorm:"not null;column:scene_id" json:"scene_id"`
	LastCountedAt time.Time `gorm:"not null;default:now()" json:"last_counted_at"`
	CreatedAt     time.Time `json:"created_at"`
}

func (UserSceneViewCount) TableName() string {
	return "user_scene_view_counts"
}

// DailyActivityCount represents the number of distinct scenes watched on a given day.
type DailyActivityCount struct {
	Date  time.Time `json:"date"`
	Count int       `json:"count"`
}
