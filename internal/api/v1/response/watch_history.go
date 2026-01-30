package response

import (
	"goonhub/internal/core"
	"goonhub/internal/data"
)

// WatchHistoryEntryResponse represents a watch history entry with lightweight video data.
type WatchHistoryEntryResponse struct {
	Watch data.UserVideoWatch `json:"watch"`
	Video *VideoListItem      `json:"video,omitempty"`
}

// ToWatchHistoryEntryResponse converts a service WatchHistoryEntry to the response type.
func ToWatchHistoryEntryResponse(entry core.WatchHistoryEntry) WatchHistoryEntryResponse {
	var video *VideoListItem
	if entry.Video != nil {
		item := ToVideoListItem(*entry.Video)
		video = &item
	}
	return WatchHistoryEntryResponse{
		Watch: entry.Watch,
		Video: video,
	}
}

// ToWatchHistoryEntriesResponse converts a slice of WatchHistoryEntry to response types.
func ToWatchHistoryEntriesResponse(entries []core.WatchHistoryEntry) []WatchHistoryEntryResponse {
	result := make([]WatchHistoryEntryResponse, len(entries))
	for i, e := range entries {
		result[i] = ToWatchHistoryEntryResponse(e)
	}
	return result
}
