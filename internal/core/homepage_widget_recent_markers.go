package core

import (
	"goonhub/internal/data"
)

// recent_markers: the markers the user created most recently.
//
// Config: labels ([]string, empty = all), ranges_only (bool).

func init() {
	registerHomepageWidget("recent_markers", HomepageWidgetSpec{
		Fetch:    fetchRecentMarkersWidget,
		Validate: validateMarkerWidgetLabels,
	})
}

func fetchRecentMarkersWidget(s *HomepageService, userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	markers, err := fetchWidgetMarkers(s, userID, data.WidgetMarkerQuery{
		Labels:     widgetConfigStrings(section.Config, "labels"),
		RangesOnly: widgetConfigBool(section.Config, "ranges_only", false),
		Order:      "recent",
		Limit:      markerWidgetLimit(section, 12),
	})
	if err != nil {
		return nil, err
	}
	result := emptyWidgetData(section, MarkerWidgetPayload{Items: markers, Order: "recent"})
	result.Total = int64(len(markers))
	return result, nil
}
