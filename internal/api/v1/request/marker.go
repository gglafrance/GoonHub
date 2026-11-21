package request

type CreateMarkerRequest struct {
	Timestamp    int    `json:"timestamp" binding:"min=0"`
	EndTimestamp *int   `json:"end_timestamp,omitempty"`
	Label        string `json:"label"`
	Color        string `json:"color"`
}

type UpdateMarkerRequest struct {
	Timestamp    *int `json:"timestamp,omitempty"`
	EndTimestamp *int `json:"end_timestamp,omitempty"`
	// ClearEndTimestamp removes the end of the marker range (turns it back into a point marker).
	ClearEndTimestamp bool    `json:"clear_end_timestamp,omitempty"`
	Label             *string `json:"label,omitempty"`
	Color             *string `json:"color,omitempty"`
}

type SetLabelTagsRequest struct {
	TagIDs []uint `json:"tag_ids" binding:"required"`
}

type SetMarkerTagsRequest struct {
	TagIDs []uint `json:"tag_ids" binding:"required"`
}
