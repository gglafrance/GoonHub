package response

import (
	"time"

	"goonhub/internal/data"
)

// SceneListItem is a lightweight representation for scene list/grid endpoints.
// Contains only the fields needed for displaying scene cards.
type SceneListItem struct {
	ID               uint      `json:"id"`
	Title            string    `json:"title"`
	Duration         int       `json:"duration"`
	Size             int64     `json:"size"`
	ThumbnailPath    string    `json:"thumbnail_path"`
	ProcessingStatus string    `json:"processing_status"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	StoredPath       string    `json:"stored_path"`
}

// ToSceneListItem converts a full Scene model to a lightweight SceneListItem.
func ToSceneListItem(v data.Scene) SceneListItem {
	return SceneListItem{
		ID:               v.ID,
		Title:            v.Title,
		Duration:         v.Duration,
		Size:             v.Size,
		ThumbnailPath:    v.ThumbnailPath,
		ProcessingStatus: v.ProcessingStatus,
		CreatedAt:        v.CreatedAt,
		UpdatedAt:        v.UpdatedAt,
		StoredPath:       v.StoredPath,
	}
}

// ToSceneListItems converts a slice of Scene models to SceneListItems.
func ToSceneListItems(scenes []data.Scene) []SceneListItem {
	items := make([]SceneListItem, len(scenes))
	for i, v := range scenes {
		items[i] = ToSceneListItem(v)
	}
	return items
}
