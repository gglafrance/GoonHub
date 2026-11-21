package response

import "github.com/google/uuid"

// StudioListItem is a lightweight representation for list endpoints
type StudioListItem struct {
	ID         uint      `json:"id"`
	UUID       uuid.UUID `json:"uuid"`
	Name       string    `json:"name"`
	Aliases    []string  `json:"aliases"`
	ShortName  string    `json:"short_name"`
	Logo       string    `json:"logo"`
	SceneCount int64     `json:"scene_count"`
	// MatchedAlias is the alias that matched the search query when the name does not.
	MatchedAlias string `json:"matched_alias,omitempty"`
}
