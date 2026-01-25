package request

type RecordWatchRequest struct {
	Duration  int  `json:"duration" binding:"min=0"`
	Position  int  `json:"position" binding:"min=0"`
	Completed bool `json:"completed"`
}
