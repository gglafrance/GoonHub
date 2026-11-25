package response

import (
	"goonhub/internal/core"
	"goonhub/internal/data"
)

// ScenesMatchInfoResponse contains minimal scene data for bulk PornDB matching
type ScenesMatchInfoResponse struct {
	Scenes []core.SceneMatchInfo `json:"scenes"`
}

// ToScenesMatchInfoResponse drops the original file names and sizes the visibility hides.
func ToScenesMatchInfoResponse(scenes []core.SceneMatchInfo, vis FileVisibility) ScenesMatchInfoResponse {
	out := make([]core.SceneMatchInfo, len(scenes))
	for i, s := range scenes {
		if !vis.Paths {
			s.OriginalFilename = nil
		}
		if !vis.Size {
			s.Size = nil
		}
		out[i] = s
	}
	return ScenesMatchInfoResponse{Scenes: out}
}

// FolderContentsResponse contains the contents of a folder with lightweight scenes.
type FolderContentsResponse struct {
	StoragePath *data.StoragePath `json:"storage_path"`
	CurrentPath string            `json:"current_path"`
	Subfolders  []FolderInfo      `json:"subfolders"`
	Scenes      []SceneListItem   `json:"scenes"`
	TotalScenes int64             `json:"total_scenes"`
	Page        int               `json:"page"`
	Limit       int               `json:"limit"`
	CardSidecars
}

// FolderInfo is a subfolder of the explorer; total_size is omitted without files:view_size.
type FolderInfo struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	SceneCount    int64  `json:"scene_count"`
	TotalDuration int64  `json:"total_duration"`
	TotalSize     *int64 `json:"total_size,omitempty"`
}

// ToFolderInfos converts the subfolders, without the sizes the visibility hides.
func ToFolderInfos(folders []data.FolderInfo, vis FileVisibility) []FolderInfo {
	out := make([]FolderInfo, len(folders))
	for i, f := range folders {
		out[i] = FolderInfo{
			Name:          f.Name,
			Path:          f.Path,
			SceneCount:    f.SceneCount,
			TotalDuration: f.TotalDuration,
			TotalSize:     int64If(vis.Size, f.TotalSize),
		}
	}
	return out
}

// ToFolderContentsResponse converts the service response to an API response.
func ToFolderContentsResponse(resp *core.FolderContentsResponse, vis FileVisibility) *FolderContentsResponse {
	return &FolderContentsResponse{
		StoragePath: resp.StoragePath,
		CurrentPath: resp.CurrentPath,
		Subfolders:  ToFolderInfos(resp.Subfolders, vis),
		Scenes:      ToSceneListItemsWithFields(resp.Scenes, SceneModelFields, vis),
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
	CardSidecars
}

// ToFolderSearchResponse converts the service response to an API response.
func ToFolderSearchResponse(resp *core.FolderSearchResponse, vis FileVisibility) *FolderSearchResponse {
	return &FolderSearchResponse{
		Scenes: ToSceneListItemsWithFields(resp.Scenes, SceneModelFields, vis),
		Total:  resp.Total,
		Page:   resp.Page,
		Limit:  resp.Limit,
	}
}
