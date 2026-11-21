package core

import (
	"fmt"

	"goonhub/internal/data"
)

// Shared helpers of the marker homepage widgets (moments, marker_compilation, recent_markers,
// browse_tags).

const (
	markerWidgetMaxLabels = 50
	markerWidgetMaxItems  = 100
)

// MarkerWidgetPayload is the payload of the marker list widgets.
type MarkerWidgetPayload struct {
	Items []data.WidgetMarker `json:"items"`
	// Order used ("random" or "recent")
	Order string `json:"order,omitempty"`
	// ClipSeconds is the duration played for markers without an end (marker_compilation)
	ClipSeconds int `json:"clip_seconds,omitempty"`
}

func validateMarkerWidgetLabels(cfg map[string]interface{}) error {
	if labels := widgetConfigStrings(cfg, "labels"); len(labels) > markerWidgetMaxLabels {
		return fmt.Errorf("at most %d labels", markerWidgetMaxLabels)
	}
	return nil
}

func validateMarkerWidgetOrder(cfg map[string]interface{}) error {
	switch order := widgetConfigString(cfg, "order", "random"); order {
	case "random", "recent":
		return nil
	default:
		return fmt.Errorf("order must be random or recent, got %q", order)
	}
}

// markerWidgetLimit bounds the section limit for marker widgets.
func markerWidgetLimit(section data.HomepageSection, def int) int {
	if section.Limit <= 0 {
		return def
	}
	return clampInt(section.Limit, 1, markerWidgetMaxItems)
}

// fetchWidgetMarkers loads markers for a marker list widget.
func fetchWidgetMarkers(s *HomepageService, userID uint, q data.WidgetMarkerQuery) ([]data.WidgetMarker, error) {
	db := s.WidgetDeps().DB
	if db == nil {
		return []data.WidgetMarker{}, nil
	}
	markers, err := data.ListWidgetMarkers(db, userID, q)
	if err != nil {
		return nil, fmt.Errorf("failed to list markers: %w", err)
	}
	if markers == nil {
		markers = []data.WidgetMarker{}
	}
	if repo := s.WidgetDeps().Markers; repo != nil && len(markers) > 0 {
		ids := make([]uint, len(markers))
		for i, m := range markers {
			ids[i] = m.ID
		}
		// Best effort: the cards work without their extra tags
		if tags, err := repo.GetMarkerTagsMultiple(ids); err == nil {
			for i := range markers {
				markers[i].Tags = tags[markers[i].ID]
			}
		}
	}
	return markers, nil
}
