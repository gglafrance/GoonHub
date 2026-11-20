package core

import (
	"fmt"

	"go.uber.org/zap"

	"goonhub/internal/data"
)

// saved_search_tiles: the user's saved searches as tiles with their result count and the first
// result as cover. Counts reuse the saved_search section logic (one search query each, with a
// single result), capped to the section limit.

// InsightsSavedSearchTile is one saved search tile.
type InsightsSavedSearchTile struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
	// Count is nil when the search failed
	Count        *int64 `json:"count"`
	CoverSceneID *uint  `json:"cover_scene_id,omitempty"`
}

// InsightsSavedSearchTilesPayload is the saved_search_tiles widget payload.
type InsightsSavedSearchTilesPayload struct {
	Items []InsightsSavedSearchTile `json:"items"`
	// Total number of saved searches (tiles are capped to the section limit)
	Total int `json:"total"`
}

func init() {
	registerHomepageWidget("saved_search_tiles", HomepageWidgetSpec{Fetch: fetchInsightsSavedSearchTilesWidget})
}

func fetchInsightsSavedSearchTilesWidget(s *HomepageService, userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	if s.savedSearchService == nil {
		return nil, fmt.Errorf("saved search tiles: saved searches not available")
	}
	searches, err := s.savedSearchService.List(userID)
	if err != nil {
		return nil, fmt.Errorf("saved search tiles: failed to list saved searches: %w", err)
	}
	limit := clampInt(section.Limit, 1, 50)
	payload := InsightsSavedSearchTilesPayload{Items: []InsightsSavedSearchTile{}, Total: len(searches)}
	for i, search := range searches {
		if i >= limit {
			break
		}
		tile := InsightsSavedSearchTile{UUID: search.UUID.String(), Name: search.Name}
		if s.searchService != nil {
			probe := data.HomepageSection{
				ID:     section.ID + "-probe",
				Type:   "saved_search",
				Limit:  1,
				Config: map[string]interface{}{"saved_search_uuid": tile.UUID},
			}
			result, err := s.fetchSavedSearchSection(userID, probe)
			if err != nil {
				s.logger.Warn("saved search tiles: search failed", zap.String("uuid", tile.UUID), zap.Error(err))
			} else {
				total := result.Total
				tile.Count = &total
				if len(result.Scenes) > 0 {
					id := result.Scenes[0].ID
					tile.CoverSceneID = &id
				}
			}
		}
		payload.Items = append(payload.Items, tile)
	}
	return emptyWidgetData(section, payload), nil
}
