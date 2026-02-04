package request

// BulkUpdateTagsRequest represents a request to bulk update tags for multiple scenes
type BulkUpdateTagsRequest struct {
	SceneIDs []uint `json:"scene_ids" binding:"required,min=1"`
	TagIDs   []uint `json:"tag_ids"`
	Mode     string `json:"mode" binding:"required,oneof=add remove replace"`
}

// BulkUpdateActorsRequest represents a request to bulk update actors for multiple scenes
type BulkUpdateActorsRequest struct {
	SceneIDs []uint `json:"scene_ids" binding:"required,min=1"`
	ActorIDs []uint `json:"actor_ids"`
	Mode     string `json:"mode" binding:"required,oneof=add remove replace"`
}

// BulkUpdateStudioRequest represents a request to bulk update studio for multiple scenes
type BulkUpdateStudioRequest struct {
	SceneIDs []uint `json:"scene_ids" binding:"required,min=1"`
	Studio   string `json:"studio"`
}

// FolderSceneIDsRequest represents a request to get scene IDs in a folder
// Supports optional filters to get only IDs matching search criteria
type FolderSceneIDsRequest struct {
	StoragePathID uint     `json:"storage_path_id" binding:"required"`
	FolderPath    string   `json:"folder_path"`
	Recursive     bool     `json:"recursive"`
	Query         string   `json:"query"`
	TagIDs        []uint   `json:"tag_ids"`
	Actors        []string `json:"actors"`
	HasPornDBID   *bool    `json:"has_porndb_id"` // nil = no filter, true = has, false = missing
}

// BulkDeleteRequest represents a request to delete multiple videos
type BulkDeleteRequest struct {
	SceneIDs  []uint `json:"scene_ids" binding:"required,min=1"`
	Permanent bool   `json:"permanent"` // false = trash (default), true = permanent delete
}

// ScenesMatchInfoRequest represents a request to get minimal scene data for bulk matching
type ScenesMatchInfoRequest struct {
	SceneIDs []uint `json:"scene_ids" binding:"required,min=1"`
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
	HasPornDBID   *bool    `json:"has_porndb_id"` // nil = no filter, true = has, false = missing
	Page          int      `json:"page"`
	Limit         int      `json:"limit"`
}
