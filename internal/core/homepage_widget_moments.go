package core

import (
	"goonhub/internal/data"
)

// moments: carousel of marker previews.
//
// Config: labels ([]string, empty = all), ranges_only (bool), order ("random" default, "recent").

func init() {
	registerHomepageWidget("moments", HomepageWidgetSpec{
		Fetch: fetchMomentsWidget,
		Validate: func(cfg map[string]interface{}) error {
			if err := validateMarkerWidgetLabels(cfg); err != nil {
				return err
			}
			return validateMarkerWidgetOrder(cfg)
		},
	})
}

func fetchMomentsWidget(s *HomepageService, userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	order := widgetConfigString(section.Config, "order", "random")
	markers, err := fetchWidgetMarkers(s, userID, data.WidgetMarkerQuery{
		Labels:     widgetConfigStrings(section.Config, "labels"),
		RangesOnly: widgetConfigBool(section.Config, "ranges_only", false),
		Order:      order,
		Limit:      markerWidgetLimit(section, 16),
	})
	if err != nil {
		return nil, err
	}
	result := emptyWidgetData(section, MarkerWidgetPayload{Items: markers, Order: order})
	result.Total = int64(len(markers))
	return result, nil
}
