package data

import "time"

const (
	ShareTypePublic       = "public"
	ShareTypeAuthRequired = "auth_required"
)

// ValidShareTypes returns all valid share type values.
func ValidShareTypes() []string {
	return []string{ShareTypePublic, ShareTypeAuthRequired}
}

// IsValidShareType checks if the given share type is valid.
func IsValidShareType(shareType string) bool {
	for _, v := range ValidShareTypes() {
		if v == shareType {
			return true
		}
	}
	return false
}

type ShareLink struct {
	ID        uint       `gorm:"primarykey" json:"id"`
	Token     string     `gorm:"uniqueIndex;size:32;not null" json:"token"`
	SceneID   uint       `gorm:"not null" json:"scene_id"`
	UserID    uint       `gorm:"not null" json:"user_id"`
	ShareType string     `gorm:"size:20;not null;default:'public'" json:"share_type"`
	ExpiresAt *time.Time `json:"expires_at"`
	ViewCount int64      `gorm:"not null;default:0" json:"view_count"`
	CreatedAt time.Time  `gorm:"not null" json:"created_at"`
}

func (ShareLink) TableName() string {
	return "share_links"
}
