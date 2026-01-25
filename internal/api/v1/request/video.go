package request

type UpdateVideoDetailsRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type SetRatingRequest struct {
	Rating float64 `json:"rating" binding:"required,min=0.5,max=5"`
}

type SearchVideosRequest struct {
	Query        string  `form:"q"`
	Tags         string  `form:"tags"`
	Actors       string  `form:"actors"`
	Studio       string  `form:"studio"`
	MinDuration  int     `form:"min_duration"`
	MaxDuration  int     `form:"max_duration"`
	MinDate      string  `form:"min_date"`
	MaxDate      string  `form:"max_date"`
	Resolution   string  `form:"resolution"`
	Sort         string  `form:"sort"`
	Page         int     `form:"page"`
	Limit        int     `form:"limit"`
	Liked        *bool   `form:"liked"`
	MinRating    float64 `form:"min_rating"`
	MaxRating    float64 `form:"max_rating"`
	MinJizzCount int     `form:"min_jizz_count"`
	MaxJizzCount int     `form:"max_jizz_count"`
}
