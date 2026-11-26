package request

import "goonhub/internal/data"

// UpdateNavLayoutRequest is the body of PUT /settings/navigation: the whole header layout
// (docs/NAVIGATION.md). The service validates it.
type UpdateNavLayoutRequest struct {
	Layout *data.NavLayout `json:"layout" binding:"required"`
}
