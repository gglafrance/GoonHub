package request

type ExtractThumbnailRequest struct {
	Timecode float64 `json:"timecode" binding:"required,min=0"`
}
