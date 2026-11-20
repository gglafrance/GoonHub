package core

import (
	"fmt"

	"goonhub/internal/data"
)

// browse_tags: tiles of marker labels or scene tags with a cover image.
//
// Config: source ("labels" default, or "tags"), items ([]string: explicit list, in that order;
// empty = most used).

// BrowseTagsPayload is the payload of the browse_tags widget.
type BrowseTagsPayload struct {
	Source string            `json:"source"`
	Tiles  []data.WidgetTile `json:"tiles"`
}

func init() {
	registerHomepageWidget("browse_tags", HomepageWidgetSpec{
		Fetch: fetchBrowseTagsWidget,
		Validate: func(cfg map[string]interface{}) error {
			switch source := widgetConfigString(cfg, "source", "labels"); source {
			case "labels", "tags":
			default:
				return fmt.Errorf("source must be labels or tags, got %q", source)
			}
			if items := widgetConfigStrings(cfg, "items"); len(items) > markerWidgetMaxLabels {
				return fmt.Errorf("at most %d items", markerWidgetMaxLabels)
			}
			return nil
		},
	})
}

func fetchBrowseTagsWidget(s *HomepageService, userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	source := widgetConfigString(section.Config, "source", "labels")
	if source != "tags" {
		source = "labels"
	}
	items := widgetConfigStrings(section.Config, "items")
	limit := markerWidgetLimit(section, 12)
	if len(items) > 0 {
		limit = len(items)
	}

	payload := BrowseTagsPayload{Source: source, Tiles: []data.WidgetTile{}}
	db := s.WidgetDeps().DB
	if db != nil {
		var tiles []data.WidgetTile
		var err error
		if source == "tags" {
			tiles, err = data.ListSceneTagTiles(db, items, limit)
		} else {
			tiles, err = data.ListMarkerLabelTiles(db, userID, items, limit)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list %s tiles: %w", source, err)
		}
		if tiles != nil {
			payload.Tiles = tiles
		}
	}
	result := emptyWidgetData(section, payload)
	result.Total = int64(len(payload.Tiles))
	return result, nil
}
