package request

// ShortsFeedRequest is the query for the Shorts feed.
type ShortsFeedRequest struct {
	Sort  string `form:"sort"` // newest (default) or random
	Seed  int64  `form:"seed"` // random shuffle seed (0 = new seed)
	Page  int    `form:"page"`
	Limit int    `form:"limit"`
	Clips string `form:"clips"` // all (default), only or hide
}

// CreateShortRequest is the A/B range to cut from a scene, in seconds.
type CreateShortRequest struct {
	Start *float64 `json:"start" binding:"required"`
	End   *float64 `json:"end" binding:"required"`
	Title string   `json:"title"`
}

// UpdateShortsSettingsRequest sets the Shorts feed limit and save folder.
type UpdateShortsSettingsRequest struct {
	MaxDuration int     `json:"max_duration"`
	SaveDir     *string `json:"save_dir"`
	// What the search page's Shorts filter starts on; empty keeps the stored one
	SearchDefault string `json:"search_default"`
}
