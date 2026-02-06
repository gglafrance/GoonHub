package response

import (
	"goonhub/internal/core"
	"goonhub/internal/data"
)

// WatchProgress represents the watch progress for a scene in homepage sections.
type WatchProgress struct {
	LastPosition int `json:"last_position"`
	Duration     int `json:"duration"`
}

// HomepageSectionData represents a section with its scene data (lightweight).
type HomepageSectionData struct {
	Section       data.HomepageSection     `json:"section"`
	Scenes        []SceneListItem          `json:"scenes"`
	Total         int64                    `json:"total"`
	Seed          int64                    `json:"seed,omitempty"`
	WatchProgress map[uint]WatchProgress   `json:"watch_progress,omitempty"`
	Ratings       map[uint]float64         `json:"ratings,omitempty"`
	Playlists     []PlaylistListItemResponse `json:"playlists,omitempty"`
}

// HomepageResponse represents the full homepage data response.
type HomepageResponse struct {
	Config   data.HomepageConfig   `json:"config"`
	Sections []HomepageSectionData `json:"sections"`
}

// ToHomepageResponse converts the service response to an API response with lightweight scenes.
func ToHomepageResponse(resp *core.HomepageResponse) *HomepageResponse {
	sections := make([]HomepageSectionData, len(resp.Sections))
	for i, s := range resp.Sections {
		// Convert watch progress
		var watchProgress map[uint]WatchProgress
		if len(s.WatchProgress) > 0 {
			watchProgress = make(map[uint]WatchProgress, len(s.WatchProgress))
			for k, v := range s.WatchProgress {
				watchProgress[k] = WatchProgress{
					LastPosition: v.LastPosition,
					Duration:     v.Duration,
				}
			}
		}

		var playlists []PlaylistListItemResponse
		if len(s.Playlists) > 0 {
			playlists = NewPlaylistListResponse(s.Playlists)
		}

		sections[i] = HomepageSectionData{
			Section:       s.Section,
			Scenes:        ToSceneListItems(s.Scenes),
			Total:         s.Total,
			Seed:          s.Seed,
			WatchProgress: watchProgress,
			Ratings:       s.Ratings,
			Playlists:     playlists,
		}
	}

	return &HomepageResponse{
		Config:   resp.Config,
		Sections: sections,
	}
}

// ToHomepageSectionDataResponse converts a single section response to lightweight format.
func ToHomepageSectionDataResponse(s *core.HomepageSectionData) *HomepageSectionData {
	var watchProgress map[uint]WatchProgress
	if len(s.WatchProgress) > 0 {
		watchProgress = make(map[uint]WatchProgress, len(s.WatchProgress))
		for k, v := range s.WatchProgress {
			watchProgress[k] = WatchProgress{
				LastPosition: v.LastPosition,
				Duration:     v.Duration,
			}
		}
	}

	var playlists []PlaylistListItemResponse
	if len(s.Playlists) > 0 {
		playlists = NewPlaylistListResponse(s.Playlists)
	}

	return &HomepageSectionData{
		Section:       s.Section,
		Scenes:        ToSceneListItems(s.Scenes),
		Total:         s.Total,
		Seed:          s.Seed,
		WatchProgress: watchProgress,
		Ratings:       s.Ratings,
		Playlists:     playlists,
	}
}
