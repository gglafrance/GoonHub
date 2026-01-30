package response

import "github.com/google/uuid"

// ActorListItem is a lightweight representation for list endpoints
type ActorListItem struct {
	ID         uint      `json:"id"`
	UUID       uuid.UUID `json:"uuid"`
	Name       string    `json:"name"`
	ImageURL   string    `json:"image_url"`
	Gender     string    `json:"gender"`
	VideoCount int64     `json:"video_count"`
}
