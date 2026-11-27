package request

type CreateShareLinkRequest struct {
	ShareType string `json:"share_type" binding:"required"`
	ExpiresIn string `json:"expires_in" binding:"required"`
	// Start (seconds, optional): the shared player starts there
	Start *int `json:"start"`
	// MarkerID (optional): share the marker's range (start, end or default length, label)
	MarkerID *uint `json:"marker_id"`
}
