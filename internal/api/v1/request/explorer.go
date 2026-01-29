package request

// BulkUpdateTagsRequest represents a request to bulk update tags for multiple videos
type BulkUpdateTagsRequest struct {
	VideoIDs []uint `json:"video_ids" binding:"required,min=1"`
	TagIDs   []uint `json:"tag_ids"`
	Mode     string `json:"mode" binding:"required,oneof=add remove replace"`
}

// BulkUpdateActorsRequest represents a request to bulk update actors for multiple videos
type BulkUpdateActorsRequest struct {
	VideoIDs []uint `json:"video_ids" binding:"required,min=1"`
	ActorIDs []uint `json:"actor_ids"`
	Mode     string `json:"mode" binding:"required,oneof=add remove replace"`
}

// BulkUpdateStudioRequest represents a request to bulk update studio for multiple videos
type BulkUpdateStudioRequest struct {
	VideoIDs []uint `json:"video_ids" binding:"required,min=1"`
	Studio   string `json:"studio"`
}

// FolderVideoIDsRequest represents a request to get video IDs in a folder
type FolderVideoIDsRequest struct {
	StoragePathID uint   `json:"storage_path_id" binding:"required"`
	FolderPath    string `json:"folder_path"`
	Recursive     bool   `json:"recursive"`
}

// BulkDeleteRequest represents a request to delete multiple videos
type BulkDeleteRequest struct {
	VideoIDs []uint `json:"video_ids" binding:"required,min=1"`
}

// FolderSearchRequest represents a request to search within a folder
type FolderSearchRequest struct {
	StoragePathID uint     `json:"storage_path_id" binding:"required"`
	FolderPath    string   `json:"folder_path"`
	Recursive     bool     `json:"recursive"`
	Query         string   `json:"query"`
	TagIDs        []uint   `json:"tag_ids"`
	Actors        []string `json:"actors"`
	Studio        string   `json:"studio"`
	MinDuration   int      `json:"min_duration"`
	MaxDuration   int      `json:"max_duration"`
	Sort          string   `json:"sort"`
	Page          int      `json:"page"`
	Limit         int      `json:"limit"`
}
