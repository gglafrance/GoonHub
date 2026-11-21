package response

// ImportCapabilitiesResponse advertises the import API version and features.
type ImportCapabilitiesResponse struct {
	Version  int      `json:"version"`
	Features []string `json:"features"`
}

// ImportSceneResponse is returned on a successful scene import (201 created / 200 updated).
type ImportSceneResponse struct {
	ID      uint   `json:"id"`
	Title   string `json:"title"`
	Created bool   `json:"created"`
}

// ImportSceneConflictResponse is returned with 409 when the scene already exists.
type ImportSceneConflictResponse struct {
	ID    uint   `json:"id"`
	Title string `json:"title"`
	Error string `json:"error"`
}

// ImportMarkerResponse is returned on a successful marker import (201 created / 200 updated).
type ImportMarkerResponse struct {
	ID      uint `json:"id"`
	SceneID uint `json:"scene_id"`
	Created bool `json:"created"`
}

// ImportMarkerConflictResponse is returned with 409 when the marker already exists.
type ImportMarkerConflictResponse struct {
	ID      uint   `json:"id"`
	SceneID uint   `json:"scene_id"`
	Error   string `json:"error"`
}

// ImportSceneSummaryItem is the imported state of a scene for the current user.
type ImportSceneSummaryItem struct {
	ID          uint    `json:"id"`
	Title       string  `json:"title"`
	StoredPath  string  `json:"stored_path"`
	StudioID    *uint   `json:"studio_id"`
	TagIDs      []uint  `json:"tag_ids"`
	ActorIDs    []uint  `json:"actor_ids"`
	ViewCount   int64   `json:"view_count"`
	MarkerCount int64   `json:"marker_count"`
	Rating      float64 `json:"rating"`
	Liked       bool    `json:"liked"`
	JizzedCount int     `json:"jizzed_count"`
	WatchCount  int64   `json:"watch_count"`
	Trashed     bool    `json:"trashed"`
}

// ImportSceneSummaryResponse is the response of the import summary endpoint.
type ImportSceneSummaryResponse struct {
	Data    []ImportSceneSummaryItem `json:"data"`
	Missing []uint                   `json:"missing"`
}
