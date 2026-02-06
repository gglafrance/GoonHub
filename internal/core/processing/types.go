package processing

// PoolConfig holds the worker pool configuration
type PoolConfig struct {
	MetadataWorkers           int `json:"metadata_workers"`
	ThumbnailWorkers          int `json:"thumbnail_workers"`
	SpritesWorkers            int `json:"sprites_workers"`
	AnimatedThumbnailsWorkers int `json:"animated_thumbnails_workers"`
}

// QualityConfig holds the processing quality configuration
type QualityConfig struct {
	MaxFrameDimensionSm    int    `json:"max_frame_dimension_sm"`
	MaxFrameDimensionLg    int    `json:"max_frame_dimension_lg"`
	FrameQualitySm         int    `json:"frame_quality_sm"`
	FrameQualityLg         int    `json:"frame_quality_lg"`
	FrameQualitySprites    int    `json:"frame_quality_sprites"`
	SpritesConcurrency     int    `json:"sprites_concurrency"`
	MarkerThumbnailType    string `json:"marker_thumbnail_type"`
	MarkerAnimatedDuration int    `json:"marker_animated_duration"`
}

// QueueStatus holds the current queue status for all pools
type QueueStatus struct {
	MetadataQueued            int `json:"metadata_queued"`
	ThumbnailQueued           int `json:"thumbnail_queued"`
	SpritesQueued             int `json:"sprites_queued"`
	AnimatedThumbnailsQueued  int `json:"animated_thumbnails_queued"`
	MetadataActive            int `json:"metadata_active"`
	ThumbnailActive           int `json:"thumbnail_active"`
	SpritesActive             int `json:"sprites_active"`
	AnimatedThumbnailsActive  int `json:"animated_thumbnails_active"`
}

// BulkPhaseResult contains the results of a bulk phase submission
type BulkPhaseResult struct {
	Submitted int `json:"submitted"`
	Skipped   int `json:"skipped"`
	Errors    int `json:"errors"`
}

// phaseState tracks completion of parallel phases for a scene
type PhaseState struct {
	ThumbnailDone           bool
	SpritesDone             bool
	AnimatedThumbnailsDone  bool
}
