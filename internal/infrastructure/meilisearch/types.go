package meilisearch

// VideoDocument represents a video document in Meilisearch.
type VideoDocument struct {
	ID               uint     `json:"id"`
	Title            string   `json:"title"`
	OriginalFilename string   `json:"original_filename"`
	Description      string   `json:"description"`
	Studio           string   `json:"studio"`
	Actors           []string `json:"actors"`
	TagIDs           []uint   `json:"tag_ids"`
	TagNames         []string `json:"tag_names"`
	Duration         float64  `json:"duration"`
	Height           int      `json:"height"`
	CreatedAt        int64    `json:"created_at"`
	ProcessingStatus string   `json:"processing_status"`
}

// SearchParams contains parameters for searching videos.
type SearchParams struct {
	Query            string
	TagIDs           []uint
	Actors           []string
	Studio           string
	MinDuration      *float64
	MaxDuration      *float64
	MinHeight        *int
	MaxHeight        *int
	DateAfter        *int64
	DateBefore       *int64
	ProcessingStatus string
	VideoIDs         []uint // Pre-filtered video IDs (for user-specific filters)
	Sort             string
	SortDir          string
	Offset           int
	Limit            int
}

// SearchResult contains the result of a search query.
type SearchResult struct {
	IDs        []uint
	TotalCount int64
}
