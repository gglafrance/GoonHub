package response

import (
	"goonhub/internal/core"
	"goonhub/internal/data"
)

// ScenesMatchInfoResponse contains minimal scene data for bulk PornDB matching
type ScenesMatchInfoResponse struct {
	Scenes []core.SceneMatchInfo `json:"scenes"`
}

// FolderContentsResponse contains the contents of a folder with lightweight scenes.
type FolderContentsResponse struct {
	StoragePath *data.StoragePath `json:"storage_path"`
	CurrentPath string            `json:"current_path"`
	Subfolders  []data.FolderInfo `json:"subfolders"`
	Scenes      []SceneListItem   `json:"scenes"`
	TotalScenes int64             `json:"total_scenes"`
	Page        int               `json:"page"`
	Limit       int               `json:"limit"`
}

// ToFolderContentsResponse converts the service response to an API response.
func ToFolderContentsResponse(resp *core.FolderContentsResponse) *FolderContentsResponse {
	return &FolderContentsResponse{
		StoragePath: resp.StoragePath,
		CurrentPath: resp.CurrentPath,
		Subfolders:  resp.Subfolders,
		Scenes:      ToSceneListItems(resp.Scenes),
		TotalScenes: resp.TotalScenes,
		Page:        resp.Page,
		Limit:       resp.Limit,
	}
}

// FolderSearchResponse contains the search results with lightweight scenes.
type FolderSearchResponse struct {
	Scenes []SceneListItem `json:"scenes"`
	Total  int64           `json:"total"`
	Page   int             `json:"page"`
	Limit  int             `json:"limit"`
}

// ToFolderSearchResponse converts the service response to an API response.
func ToFolderSearchResponse(resp *core.FolderSearchResponse) *FolderSearchResponse {
	return &FolderSearchResponse{
		Scenes: ToSceneListItems(resp.Scenes),
		Total:  resp.Total,
		Page:   resp.Page,
		Limit:  resp.Limit,
	}
}
