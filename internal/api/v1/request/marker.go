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

type SetLabelTagsRequest struct {
	TagIDs []uint `json:"tag_ids" binding:"required"`
}

type SetMarkerTagsRequest struct {
	TagIDs []uint `json:"tag_ids" binding:"required"`
}
