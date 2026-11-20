package core

import (
	"fmt"

	"goonhub/internal/data"
)

// shorts: random short clips for a "zapping" feed.
//
// Config: max_duration (seconds, default 60), unwatched_only (default false).
// Payload: {"max_duration": n}.

const (
	shortsDefaultMaxDuration = 60
	shortsMinMaxDuration     = 5
	shortsMaxMaxDuration     = 3600
)

type shortsWidgetPayload struct {
	MaxDuration int `json:"max_duration"`
}

func init() {
	registerHomepageWidget("shorts", HomepageWidgetSpec{
		Fetch:    fetchShortsWidget,
		Validate: validateShortsWidget,
	})
}

func validateShortsWidget(cfg map[string]interface{}) error {
	if err := displayValidateIntRange(cfg, "max_duration", shortsMinMaxDuration, shortsMaxMaxDuration); err != nil {
		return fmt.Errorf("shorts: %w", err)
	}
	if err := displayValidateBool(cfg, "unwatched_only"); err != nil {
		return fmt.Errorf("shorts: %w", err)
	}
	return nil
}

func fetchShortsWidget(s *HomepageService, userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	q, err := displayWidgetQueries(s)
	if err != nil {
		return nil, err
	}
	maxDuration := clampInt(widgetConfigInt(section.Config, "max_duration", shortsDefaultMaxDuration), shortsMinMaxDuration, shortsMaxMaxDuration)
	unwatchedOnly := widgetConfigBool(section.Config, "unwatched_only", false)

	scenes, err := q.ShortScenes(userID, maxDuration, unwatchedOnly, clampInt(section.Limit, 1, 50))
	if err != nil {
		return nil, fmt.Errorf("failed to get short scenes: %w", err)
	}
	return &HomepageSectionData{
		Section: section,
		Scenes:  scenes,
		Total:   int64(len(scenes)),
		Widget:  shortsWidgetPayload{MaxDuration: maxDuration},
	}, nil
}
