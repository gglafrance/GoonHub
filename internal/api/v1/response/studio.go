package response

import "github.com/google/uuid"

// StudioListItem is a lightweight representation for list endpoints
type StudioListItem struct {
	ID         uint      `json:"id"`
	UUID       uuid.UUID `json:"uuid"`
	Name       string    `json:"name"`
	ShortName  string    `json:"short_name"`
	Logo       string    `json:"logo"`
	VideoCount int64     `json:"video_count"`
}
