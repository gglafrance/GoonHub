package core

import (
	"fmt"
	"time"

	"goonhub/internal/data"
)

// top_period: the user's most watched scenes over a rolling period, compared to the period
// before it.
//
// Config: period (week = 7 days, month = 30 days, year = 365 days; default month).
// Payload: {"period", "from", "to", "total_views", "previous_total_views",
// "items": [{"scene_id", "views", "previous_views"}]} (items in rank order, like Scenes).

var topPeriodPeriods = []string{"week", "month", "year"}

type topPeriodItem struct {
	SceneID       uint  `json:"scene_id"`
	Views         int64 `json:"views"`
	PreviousViews int64 `json:"previous_views"`
}

type topPeriodWidgetPayload struct {
	Period             string          `json:"period"`
	From               time.Time       `json:"from"`
	To                 time.Time       `json:"to"`
	TotalViews         int64           `json:"total_views"`
	PreviousTotalViews int64           `json:"previous_total_views"`
	Items              []topPeriodItem `json:"items"`
}

// topPeriodNow is replaced in tests.
var topPeriodNow = time.Now

func init() {
	registerHomepageWidget("top_period", HomepageWidgetSpec{
		Fetch:    fetchTopPeriodWidget,
		Validate: validateTopPeriodWidget,
	})
}

func validateTopPeriodWidget(cfg map[string]interface{}) error {
	if err := displayValidateOneOf(cfg, "period", topPeriodPeriods...); err != nil {
		return fmt.Errorf("top of the period: %w", err)
	}
	return nil
}

func fetchTopPeriodWidget(s *HomepageService, userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	q, err := displayWidgetQueries(s)
	if err != nil {
		return nil, err
	}
	period := widgetConfigString(section.Config, "period", "month")
	from, to, prevFrom := displayPeriodWindow(period, topPeriodNow())

	top, err := q.TopWatchedScenes(userID, from, to, clampInt(section.Limit, 1, 50))
	if err != nil {
		return nil, fmt.Errorf("failed to get top watched scenes: %w", err)
	}
	ids := make([]uint, len(top))
	for i, t := range top {
		ids[i] = t.SceneID
	}
	previous, err := q.ViewCountsForScenes(userID, ids, prevFrom, from)
	if err != nil {
		return nil, fmt.Errorf("failed to get previous period views: %w", err)
	}
	total, err := q.CountViews(userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to count views: %w", err)
	}
	previousTotal, err := q.CountViews(userID, prevFrom, from)
	if err != nil {
		return nil, fmt.Errorf("failed to count previous views: %w", err)
	}

	var scenes []data.Scene
	if len(ids) > 0 {
		found, err := s.sceneRepo.GetByIDs(ids)
		if err != nil {
			return nil, fmt.Errorf("failed to get scenes: %w", err)
		}
		scenes = displayOrderScenes(found, ids)
	}
	items := make([]topPeriodItem, 0, len(scenes))
	views := make(map[uint]int64, len(top))
	for _, t := range top {
		views[t.SceneID] = t.Views
	}
	for _, sc := range scenes {
		items = append(items, topPeriodItem{SceneID: sc.ID, Views: views[sc.ID], PreviousViews: previous[sc.ID]})
	}
	if scenes == nil {
		scenes = []data.Scene{}
	}

	return &HomepageSectionData{
		Section: section,
		Scenes:  scenes,
		Total:   int64(len(scenes)),
		Widget: topPeriodWidgetPayload{
			Period:             period,
			From:               from,
			To:                 to,
			TotalViews:         total,
			PreviousTotalViews: previousTotal,
			Items:              items,
		},
	}, nil
}
