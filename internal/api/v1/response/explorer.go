package response

import (
	"goonhub/internal/core"
	"goonhub/internal/data"
)

// FolderContentsResponse contains the contents of a folder with lightweight videos.
type FolderContentsResponse struct {
	StoragePath *data.StoragePath `json:"storage_path"`
	CurrentPath string            `json:"current_path"`
	Subfolders  []data.FolderInfo `json:"subfolders"`
	Videos      []VideoListItem   `json:"videos"`
	TotalVideos int64             `json:"total_videos"`
	Page        int               `json:"page"`
	Limit       int               `json:"limit"`
}

// ToFolderContentsResponse converts the service response to an API response.
func ToFolderContentsResponse(resp *core.FolderContentsResponse) *FolderContentsResponse {
	return &FolderContentsResponse{
		StoragePath: resp.StoragePath,
		CurrentPath: resp.CurrentPath,
		Subfolders:  resp.Subfolders,
		Videos:      ToVideoListItems(resp.Videos),
		TotalVideos: resp.TotalVideos,
		Page:        resp.Page,
		Limit:       resp.Limit,
	}
}

// FolderSearchResponse contains the search results with lightweight videos.
type FolderSearchResponse struct {
	Videos []VideoListItem `json:"videos"`
	Total  int64           `json:"total"`
	Page   int             `json:"page"`
	Limit  int             `json:"limit"`
}

// ToFolderSearchResponse converts the service response to an API response.
func ToFolderSearchResponse(resp *core.FolderSearchResponse) *FolderSearchResponse {
	return &FolderSearchResponse{
		Videos: ToVideoListItems(resp.Videos),
		Total:  resp.Total,
		Page:   resp.Page,
		Limit:  resp.Limit,
	}
}
