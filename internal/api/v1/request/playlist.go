package request

type CreatePlaylistRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description *string `json:"description,omitempty"`
	Visibility  string  `json:"visibility,omitempty"`
	TagIDs      []uint  `json:"tag_ids,omitempty"`
	SceneIDs    []uint  `json:"scene_ids,omitempty"`
}

type UpdatePlaylistRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Visibility  *string `json:"visibility,omitempty"`
}

type AddPlaylistScenesRequest struct {
	SceneIDs []uint `json:"scene_ids" binding:"required"`
}

type ReorderPlaylistScenesRequest struct {
	SceneIDs []uint `json:"scene_ids" binding:"required"`
}

type SetPlaylistTagsRequest struct {
	TagIDs []uint `json:"tag_ids"`
}

type UpdatePlaylistProgressRequest struct {
	SceneID   uint    `json:"scene_id" binding:"required"`
	PositionS float64 `json:"position_s"`
}
