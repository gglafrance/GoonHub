package request

type CreatePlaylistRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description *string `json:"description,omitempty"`
	Visibility  string  `json:"visibility,omitempty"`
	// Kind is "scenes" (default) or "markers", fixed at creation
	Kind      string `json:"kind,omitempty"`
	TagIDs    []uint `json:"tag_ids,omitempty"`
	SceneIDs  []uint `json:"scene_ids,omitempty"`
	MarkerIDs []uint `json:"marker_ids,omitempty"`
}

type UpdatePlaylistRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Visibility  *string `json:"visibility,omitempty"`
}

type AddPlaylistScenesRequest struct {
	SceneIDs []uint `json:"scene_ids" binding:"required"`
}

type RemovePlaylistScenesRequest struct {
	SceneIDs []uint `json:"scene_ids" binding:"required"`
}

type ReorderPlaylistScenesRequest struct {
	SceneIDs []uint `json:"scene_ids" binding:"required"`
}

type SetPlaylistTagsRequest struct {
	TagIDs []uint `json:"tag_ids"`
}

// UpdatePlaylistProgressRequest: scene_id for a scene playlist, marker_id for a marker
// playlist (its scene is stored).
type UpdatePlaylistProgressRequest struct {
	SceneID   uint    `json:"scene_id"`
	MarkerID  *uint   `json:"marker_id,omitempty"`
	PositionS float64 `json:"position_s"`
}

type PlaylistMarkerIDsRequest struct {
	MarkerIDs []uint `json:"marker_ids" binding:"required"`
}

// AddPlaylistMarkersFromFilterRequest selects markers like the marker pages: label for a label
// page, else the listing of GET /markers/all (same values as its query parameters, lists
// comma-separated).
type AddPlaylistMarkersFromFilterRequest struct {
	Label      string `json:"label"`
	Labels     string `json:"labels"`
	Tags       string `json:"tags"`
	Actors     string `json:"actors"`
	Studio     string `json:"studio"`
	MarkerTags string `json:"marker_tags"`
	RangesOnly bool   `json:"ranges_only"`
	Sort       string `json:"sort"`
	Seed       int64  `json:"seed"`
}
