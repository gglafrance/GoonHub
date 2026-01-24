package request

type UpdateVideoDetailsRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type SetRatingRequest struct {
	Rating float64 `json:"rating" binding:"required,min=0.5,max=5"`
}
