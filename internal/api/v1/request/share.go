package request

type CreateShareLinkRequest struct {
	ShareType string `json:"share_type" binding:"required"`
	ExpiresIn string `json:"expires_in" binding:"required"`
}
