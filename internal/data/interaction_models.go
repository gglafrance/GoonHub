package data

import (
	"time"
)

type UserVideoRating struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	UserID    uint      `gorm:"not null" json:"user_id"`
	VideoID   uint      `gorm:"not null" json:"video_id"`
	Rating    float64   `gorm:"type:decimal(2,1);not null" json:"rating"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UserVideoLike struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	UserID    uint      `gorm:"not null" json:"user_id"`
	VideoID   uint      `gorm:"not null" json:"video_id"`
	CreatedAt time.Time `json:"created_at"`
}

type UserVideoJizzed struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	UserID    uint      `gorm:"not null" json:"user_id"`
	VideoID   uint      `gorm:"not null" json:"video_id"`
	Count     int       `gorm:"not null;default:0" json:"count"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (UserVideoJizzed) TableName() string {
	return "user_video_jizzed"
}

type UserVideoWatch struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	UserID        uint      `gorm:"not null" json:"user_id"`
	VideoID       uint      `gorm:"not null" json:"video_id"`
	WatchedAt     time.Time `gorm:"not null;default:now()" json:"watched_at"`
	WatchDuration int       `gorm:"not null;default:0" json:"watch_duration"`
	LastPosition  int       `gorm:"not null;default:0" json:"last_position"`
	Completed     bool      `gorm:"not null;default:false" json:"completed"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (UserVideoWatch) TableName() string {
	return "user_video_watches"
}
