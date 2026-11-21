package core

import (
	"fmt"

	"goonhub/internal/data"
)

// marker_compilation: an in-page player chaining the markers of some labels.
//
// Config: labels ([]string, empty = all), ranges_only (bool), order ("random" default, "recent"),
// clip_seconds (int, duration played for markers without an end, default 15).

const (
	compilationDefaultClipSeconds = 15
	compilationMinClipSeconds     = 3
	compilationMaxClipSeconds     = 120
)

func init() {
	registerHomepageWidget("marker_compilation", HomepageWidgetSpec{
		Fetch: fetchMarkerCompilationWidget,
		Validate: func(cfg map[string]interface{}) error {
			if err := validateMarkerWidgetLabels(cfg); err != nil {
				return err
			}
			if err := validateMarkerWidgetOrder(cfg); err != nil {
				return err
			}
			if _, ok := cfg["clip_seconds"]; ok {
				clip := widgetConfigInt(cfg, "clip_seconds", compilationDefaultClipSeconds)
				if clip < compilationMinClipSeconds || clip > compilationMaxClipSeconds {
					return fmt.Errorf("clip_seconds must be between %d and %d", compilationMinClipSeconds, compilationMaxClipSeconds)
				}
			}
			return nil
		},
	})
}

func fetchMarkerCompilationWidget(s *HomepageService, userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	order := widgetConfigString(section.Config, "order", "random")
	clip := clampInt(
		widgetConfigInt(section.Config, "clip_seconds", compilationDefaultClipSeconds),
		compilationMinClipSeconds, compilationMaxClipSeconds,
	)
	markers, err := fetchWidgetMarkers(s, userID, data.WidgetMarkerQuery{
		Labels:     widgetConfigStrings(section.Config, "labels"),
		RangesOnly: widgetConfigBool(section.Config, "ranges_only", false),
		Order:      order,
		Limit:      markerWidgetLimit(section, 30),
	})
	if err != nil {
		return nil, err
	}
	result := emptyWidgetData(section, MarkerWidgetPayload{Items: markers, Order: order, ClipSeconds: clip})
	result.Total = int64(len(markers))
	return result, nil
}
