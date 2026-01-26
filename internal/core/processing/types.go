package processing

// PoolConfig holds the worker pool configuration
type PoolConfig struct {
	MetadataWorkers  int `json:"metadata_workers"`
	ThumbnailWorkers int `json:"thumbnail_workers"`
	SpritesWorkers   int `json:"sprites_workers"`
}

// QualityConfig holds the processing quality configuration
type QualityConfig struct {
	MaxFrameDimensionSm int `json:"max_frame_dimension_sm"`
	MaxFrameDimensionLg int `json:"max_frame_dimension_lg"`
	FrameQualitySm      int `json:"frame_quality_sm"`
	FrameQualityLg      int `json:"frame_quality_lg"`
	FrameQualitySprites int `json:"frame_quality_sprites"`
	SpritesConcurrency  int `json:"sprites_concurrency"`
}

// QueueStatus holds the current queue status for all pools
type QueueStatus struct {
	MetadataQueued  int `json:"metadata_queued"`
	ThumbnailQueued int `json:"thumbnail_queued"`
	SpritesQueued   int `json:"sprites_queued"`
}

// BulkPhaseResult contains the results of a bulk phase submission
type BulkPhaseResult struct {
	Submitted int `json:"submitted"`
	Skipped   int `json:"skipped"`
	Errors    int `json:"errors"`
}

// phaseState tracks completion of parallel phases for a video
type PhaseState struct {
	ThumbnailDone bool
	SpritesDone   bool
}
