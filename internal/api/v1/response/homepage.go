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
	Section       data.HomepageSection       `json:"section"`
	Scenes        []SceneListItem            `json:"scenes"`
	Total         int64                      `json:"total"`
	Seed          int64                      `json:"seed,omitempty"`
	WatchProgress map[uint]WatchProgress     `json:"watch_progress,omitempty"`
	Ratings       map[uint]float64           `json:"ratings,omitempty"`
	Playlists     []PlaylistListItemResponse `json:"playlists,omitempty"`
	Widget        any                        `json:"widget,omitempty"`
}

// HomepageResponse represents the full homepage data response.
type HomepageResponse struct {
	Config      data.HomepageConfig   `json:"config"`
	ActiveTabID string                `json:"active_tab_id"`
	Sections    []HomepageSectionData `json:"sections"`
}

// ToHomepageResponse converts the service response to an API response with lightweight scenes.
func ToHomepageResponse(resp *core.HomepageResponse, vis FileVisibility) *HomepageResponse {
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
			Scenes:        ToSceneListItems(s.Scenes, vis),
			Total:         s.Total,
			Seed:          s.Seed,
			WatchProgress: watchProgress,
			Ratings:       s.Ratings,
			Playlists:     playlists,
			Widget:        redactWidget(s.Widget, vis),
		}
	}

	return &HomepageResponse{
		Config:      resp.Config,
		ActiveTabID: resp.ActiveTabID,
		Sections:    sections,
	}
}

// ToHomepageSectionDataResponse converts a single section response to lightweight format.
func ToHomepageSectionDataResponse(s *core.HomepageSectionData, vis FileVisibility) *HomepageSectionData {
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
		Scenes:        ToSceneListItems(s.Scenes, vis),
		Total:         s.Total,
		Seed:          s.Seed,
		WatchProgress: watchProgress,
		Ratings:       s.Ratings,
		Playlists:     playlists,
	}
}

// redactWidget removes from a widget payload the file information the visibility hides
// (stats total_size, records biggest_file). The payload is copied, never modified.
func redactWidget(widget any, vis FileVisibility) any {
	if vis.Size {
		return widget
	}
	switch p := widget.(type) {
	case core.StatsWidgetPayload:
		p.TotalSize = nil
		return p
	case *core.StatsWidgetPayload:
		if p == nil {
			return widget
		}
		cp := *p
		cp.TotalSize = nil
		return &cp
	case core.RecordsPayload:
		p.BiggestFile = nil
		return p
	case *core.RecordsPayload:
		if p == nil {
			return widget
		}
		cp := *p
		cp.BiggestFile = nil
		return &cp
	}
	return widget
}
