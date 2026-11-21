package core

import (
	"fmt"
	"time"

	"goonhub/internal/data"
)

// quick_rate: recently watched scenes the user has not rated yet, rated one card at a time.
//
// Config: days (1-365, default 30).
// Payload: {"days", "total_unrated", "liked": {scene_id: bool}}.

const (
	quickRateDefaultDays = 30
	quickRateMaxDays     = 365
)

type quickRateWidgetPayload struct {
	Days         int           `json:"days"`
	TotalUnrated int64         `json:"total_unrated"`
	Liked        map[uint]bool `json:"liked"`
}

// quickRateNow is replaced in tests.
var quickRateNow = time.Now

func init() {
	registerHomepageWidget("quick_rate", HomepageWidgetSpec{
		Fetch:    fetchQuickRateWidget,
		Validate: validateQuickRateWidget,
	})
}

func validateQuickRateWidget(cfg map[string]interface{}) error {
	if err := displayValidateIntRange(cfg, "days", 1, quickRateMaxDays); err != nil {
		return fmt.Errorf("quick rate: %w", err)
	}
	return nil
}

func fetchQuickRateWidget(s *HomepageService, userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	q, err := displayWidgetQueries(s)
	if err != nil {
		return nil, err
	}
	days := clampInt(widgetConfigInt(section.Config, "days", quickRateDefaultDays), 1, quickRateMaxDays)
	since := quickRateNow().AddDate(0, 0, -days)

	scenes, total, err := q.UnratedRecentlyWatched(userID, since, clampInt(section.Limit, 1, 50))
	if err != nil {
		return nil, fmt.Errorf("failed to get unrated scenes: %w", err)
	}
	liked := map[uint]bool{}
	if s.interactionRepo != nil && len(scenes) > 0 {
		if likes, err := s.interactionRepo.GetLikesBySceneIDs(userID, displaySceneIDs(scenes)); err == nil {
			liked = likes
		}
	}
	if scenes == nil {
		scenes = []data.Scene{}
	}
	return &HomepageSectionData{
		Section: section,
		Scenes:  scenes,
		Total:   total,
		Widget:  quickRateWidgetPayload{Days: days, TotalUnrated: total, Liked: liked},
	}, nil
}
