package data

import (
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
	ID               uint      `gorm:"primarykey" json:"id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	UserID           uint      `gorm:"uniqueIndex;not null" json:"user_id"`
	Autoplay         bool      `gorm:"not null;default:false" json:"autoplay"`
	DefaultVolume    int       `gorm:"not null;default:100" json:"default_volume"`
	Loop             bool      `gorm:"not null;default:false" json:"loop"`
	VideosPerPage    int       `gorm:"not null;default:20" json:"videos_per_page"`
	DefaultSortOrder string    `gorm:"not null;default:'created_at_desc'" json:"default_sort_order"`
	DefaultTagSort   string    `gorm:"not null;default:'az'" json:"default_tag_sort"`
}
