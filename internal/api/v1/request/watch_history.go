package request

type RecordWatchRequest struct {
	Duration  int  `json:"duration" binding:"min=0"`
	Position  int  `json:"position" binding:"min=0"`
	Completed bool `json:"completed"`
}

// RecordReplayHeatmapRequest reports parts of a scene that were played continuously, as
// [start, end] pairs in seconds. Duration is the player's duration of the scene, used while
// the scene duration is not known yet.
type RecordReplayHeatmapRequest struct {
	Ranges   [][2]float64 `json:"ranges" binding:"required,max=500"`
	Duration float64      `json:"duration" binding:"min=0"`
}
