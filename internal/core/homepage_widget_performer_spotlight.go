package core

import (
	"fmt"

	"goonhub/internal/data"
)

// performer_spotlight: the user's top performers with their scene count, views, jizz count on
// their scenes, liked scenes and last watch. Config order: views (default) | jizz | scenes.

// InsightsPerformerSpotlightPayload is the performer_spotlight widget payload.
type InsightsPerformerSpotlightPayload struct {
	Order string                      `json:"order"`
	Items []data.InsightsPerformerRow `json:"items"`
}

func init() {
	registerHomepageWidget("performer_spotlight", HomepageWidgetSpec{
		Fetch:    fetchInsightsPerformerSpotlightWidget,
		Validate: validateInsightsPerformerSpotlightWidget,
	})
}

func validateInsightsPerformerSpotlightWidget(cfg map[string]interface{}) error {
	if err := insightsValidateOneOf(cfg, "order", "views", "jizz", "scenes"); err != nil {
		return fmt.Errorf("performer spotlight: %w", err)
	}
	return nil
}

func fetchInsightsPerformerSpotlightWidget(s *HomepageService, userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	q, err := insightsQueries(s)
	if err != nil {
		return nil, err
	}
	order := widgetConfigString(section.Config, "order", "views")
	if order != "jizz" && order != "scenes" {
		order = "views"
	}
	rows, err := q.TopPerformers(userID, order, clampInt(section.Limit, 1, 50))
	if err != nil {
		return nil, fmt.Errorf("performer spotlight: failed to load performers: %w", err)
	}
	if rows == nil {
		rows = []data.InsightsPerformerRow{}
	}
	return emptyWidgetData(section, InsightsPerformerSpotlightPayload{Order: order, Items: rows}), nil
}
