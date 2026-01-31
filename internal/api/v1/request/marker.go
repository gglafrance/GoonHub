package request

type CreateMarkerRequest struct {
	Timestamp int    `json:"timestamp" binding:"min=0"`
	Label     string `json:"label"`
	Color     string `json:"color"`
}

type UpdateMarkerRequest struct {
	Timestamp *int    `json:"timestamp,omitempty"`
	Label     *string `json:"label,omitempty"`
	Color     *string `json:"color,omitempty"`
}
