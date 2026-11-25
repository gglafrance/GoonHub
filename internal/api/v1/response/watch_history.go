package response

import (
	"goonhub/internal/core"
	"goonhub/internal/data"
)

// WatchHistoryEntryResponse represents a watch history entry with lightweight scene data.
type WatchHistoryEntryResponse struct {
	Watch data.UserSceneWatch `json:"watch"`
	Scene *SceneListItem      `json:"scene,omitempty"`
}

// ToWatchHistoryEntryResponse converts a service WatchHistoryEntry to the response type.
func ToWatchHistoryEntryResponse(entry core.WatchHistoryEntry, vis FileVisibility) WatchHistoryEntryResponse {
	var scene *SceneListItem
	if entry.Scene != nil {
		item := ToSceneListItem(*entry.Scene, vis)
		scene = &item
	}
	return WatchHistoryEntryResponse{
		Watch: entry.Watch,
		Scene: scene,
	}
}

// ToWatchHistoryEntriesResponse converts a slice of WatchHistoryEntry to response types.
func ToWatchHistoryEntriesResponse(entries []core.WatchHistoryEntry, vis FileVisibility) []WatchHistoryEntryResponse {
	result := make([]WatchHistoryEntryResponse, len(entries))
	for i, e := range entries {
		result[i] = ToWatchHistoryEntryResponse(e, vis)
	}
	return result
}
