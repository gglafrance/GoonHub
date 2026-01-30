package response

import (
	"time"

	"goonhub/internal/data"
)

// VideoListItem is a lightweight representation for video list/grid endpoints.
// Contains only the fields needed for displaying video cards.
type VideoListItem struct {
	ID               uint      `json:"id"`
	Title            string    `json:"title"`
	Duration         int       `json:"duration"`
	Size             int64     `json:"size"`
	ThumbnailPath    string    `json:"thumbnail_path"`
	ProcessingStatus string    `json:"processing_status"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// ToVideoListItem converts a full Video model to a lightweight VideoListItem.
func ToVideoListItem(v data.Video) VideoListItem {
	return VideoListItem{
		ID:               v.ID,
		Title:            v.Title,
		Duration:         v.Duration,
		Size:             v.Size,
		ThumbnailPath:    v.ThumbnailPath,
		ProcessingStatus: v.ProcessingStatus,
		CreatedAt:        v.CreatedAt,
		UpdatedAt:        v.UpdatedAt,
	}
}

// ToVideoListItems converts a slice of Video models to VideoListItems.
func ToVideoListItems(videos []data.Video) []VideoListItem {
	items := make([]VideoListItem, len(videos))
	for i, v := range videos {
		items[i] = ToVideoListItem(v)
	}
	return items
}
