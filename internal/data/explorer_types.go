package data

// FolderInfo represents a folder in the explorer view
type FolderInfo struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	VideoCount    int64  `json:"video_count"`
	TotalDuration int64  `json:"total_duration"` // Total duration in seconds
	TotalSize     int64  `json:"total_size"`     // Total size in bytes
}

// StoragePathWithCount extends StoragePath with video count
type StoragePathWithCount struct {
	StoragePath
	VideoCount int64 `json:"video_count"`
}
