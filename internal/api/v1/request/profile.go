package request

import "goonhub/internal/data"

// UpdateProfileRequest is the body of PATCH /profiles/me (absent fields are unchanged).
type UpdateProfileRequest struct {
	DisplayName *string `json:"display_name"`
	Bio         *string `json:"bio"`
}

// SetProfileLayoutRequest is the body of PUT /profiles/me/layout: the ordered blocks, or
// reset to go back to the default layout.
type SetProfileLayoutRequest struct {
	Blocks []data.ProfileBlockConfig `json:"blocks"`
	Reset  bool                      `json:"reset"`
}

// SetFavoriteScenesRequest is the body of PUT /profiles/me/showcases/scenes.
type SetFavoriteScenesRequest struct {
	SceneIDs []uint `json:"scene_ids"`
}

// SetFavoriteMarkersRequest is the body of PUT /profiles/me/showcases/markers.
type SetFavoriteMarkersRequest struct {
	MarkerIDs []uint `json:"marker_ids"`
}

// SetProfileBannerRequest is the body of PUT /profiles/me/banner: type "scene" (with
// scene_id) or "none". Uploads use POST /profiles/me/banner.
type SetProfileBannerRequest struct {
	Type    string `json:"type"`
	SceneID uint   `json:"scene_id"`
}
