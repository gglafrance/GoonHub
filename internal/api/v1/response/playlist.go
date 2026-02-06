package response

import (
	"time"

	"goonhub/internal/core"
)

// PlaylistOwnerResponse represents the owner of a playlist
type PlaylistOwnerResponse struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}

// PlaylistTagResponse represents a tag on a playlist
type PlaylistTagResponse struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// PlaylistThumbnailSceneResponse represents a scene thumbnail in the playlist grid
type PlaylistThumbnailSceneResponse struct {
	ID            uint   `json:"id"`
	ThumbnailPath string `json:"thumbnail_path"`
}

// PlaylistListItemResponse is the API response for a playlist list item
type PlaylistListItemResponse struct {
	UUID            string                           `json:"uuid"`
	Name            string                           `json:"name"`
	Description     *string                          `json:"description"`
	Visibility      string                           `json:"visibility"`
	SceneCount      int64                            `json:"scene_count"`
	TotalDuration   int64                            `json:"total_duration"`
	Owner           PlaylistOwnerResponse            `json:"owner"`
	Tags            []PlaylistTagResponse            `json:"tags"`
	ThumbnailScenes []PlaylistThumbnailSceneResponse `json:"thumbnail_scenes"`
	IsLiked         bool                             `json:"is_liked"`
	LikeCount       int64                            `json:"like_count"`
	CreatedAt       time.Time                        `json:"created_at"`
	UpdatedAt       time.Time                        `json:"updated_at"`
}

// PlaylistSceneEntryResponse is the API response for a scene in a playlist
type PlaylistSceneEntryResponse struct {
	Position int           `json:"position"`
	Scene    SceneListItem `json:"scene"`
	AddedAt  time.Time     `json:"added_at"`
}

// PlaylistResumeResponse is the API response for playlist resume info
type PlaylistResumeResponse struct {
	SceneID   *uint   `json:"scene_id"`
	PositionS float64 `json:"position_s"`
}

// PlaylistDetailResponse is the API response for a full playlist detail
type PlaylistDetailResponse struct {
	PlaylistListItemResponse
	Scenes []PlaylistSceneEntryResponse `json:"scenes"`
	Resume *PlaylistResumeResponse      `json:"resume"`
}

// NewPlaylistListItemResponse converts a service PlaylistListItem to an API response
func NewPlaylistListItemResponse(item core.PlaylistListItem) PlaylistListItemResponse {
	tags := make([]PlaylistTagResponse, len(item.Tags))
	for i, t := range item.Tags {
		tags[i] = PlaylistTagResponse{
			ID:    t.ID,
			Name:  t.Name,
			Color: t.Color,
		}
	}

	thumbnails := make([]PlaylistThumbnailSceneResponse, len(item.ThumbnailScenes))
	for i, ts := range item.ThumbnailScenes {
		thumbnails[i] = PlaylistThumbnailSceneResponse{
			ID:            ts.ID,
			ThumbnailPath: ts.ThumbnailPath,
		}
	}

	return PlaylistListItemResponse{
		UUID:          item.UUID,
		Name:          item.Name,
		Description:   item.Description,
		Visibility:    item.Visibility,
		SceneCount:    item.SceneCount,
		TotalDuration: item.TotalDuration,
		Owner: PlaylistOwnerResponse{
			ID:       item.Owner.ID,
			Username: item.Owner.Username,
		},
		Tags:            tags,
		ThumbnailScenes: thumbnails,
		IsLiked:         item.IsLiked,
		LikeCount:       item.LikeCount,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}
}

// NewPlaylistListResponse converts a slice of service PlaylistListItems to API responses
func NewPlaylistListResponse(items []core.PlaylistListItem) []PlaylistListItemResponse {
	result := make([]PlaylistListItemResponse, len(items))
	for i, item := range items {
		result[i] = NewPlaylistListItemResponse(item)
	}
	return result
}

// NewPlaylistDetailResponse converts a service PlaylistDetail to an API response
func NewPlaylistDetailResponse(detail *core.PlaylistDetail) PlaylistDetailResponse {
	scenes := make([]PlaylistSceneEntryResponse, len(detail.Scenes))
	for i, entry := range detail.Scenes {
		scenes[i] = PlaylistSceneEntryResponse{
			Position: entry.Position,
			Scene:    ToSceneListItem(entry.Scene),
			AddedAt:  entry.AddedAt,
		}
	}

	var resume *PlaylistResumeResponse
	if detail.Resume != nil {
		resume = &PlaylistResumeResponse{
			SceneID:   detail.Resume.SceneID,
			PositionS: detail.Resume.PositionS,
		}
	}

	return PlaylistDetailResponse{
		PlaylistListItemResponse: NewPlaylistListItemResponse(detail.PlaylistListItem),
		Scenes:                   scenes,
		Resume:                   resume,
	}
}
