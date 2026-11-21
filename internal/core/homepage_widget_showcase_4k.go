package core

import (
	"fmt"

	"goonhub/internal/data"
)

// showcase_4k: the highest resolution scenes.
// Config: min_height (default 2160), order random|newest (default random), tags,
// unwatched_only (default false).
// Widget payload: {"min_height": N, "total": N}

const showcase4KDefaultHeight = 2160

type showcase4KPayload struct {
	MinHeight int   `json:"min_height"`
	Total     int64 `json:"total"`
}

func init() {
	registerHomepageWidget("showcase_4k", HomepageWidgetSpec{
		Fetch: fetchShowcase4KWidget,
		Validate: func(cfg map[string]interface{}) error {
			if err := validateSceneWidgetFilter(cfg); err != nil {
				return err
			}
			if h := widgetConfigInt(cfg, "min_height", showcase4KDefaultHeight); h < 1 || h > 10000 {
				return fmt.Errorf("min_height must be between 1 and 10000")
			}
			switch widgetConfigString(cfg, "order", "random") {
			case "random", "newest":
				return nil
			default:
				return fmt.Errorf("order must be random or newest")
			}
		},
	})
}

func fetchShowcase4KWidget(s *HomepageService, userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	queries, err := sceneWidgetQueries(s)
	if err != nil {
		return nil, err
	}
	filter := sceneWidgetFilter(section.Config)
	filter.MinHeight = widgetConfigInt(section.Config, "min_height", showcase4KDefaultHeight)
	if filter.MinHeight <= 0 {
		filter.MinHeight = showcase4KDefaultHeight
	}
	order := data.HomepageSceneOrderRandom
	if widgetConfigString(section.Config, "order", "random") == "newest" {
		order = data.HomepageSceneOrderNewest
	}
	unwatchedOnly := widgetConfigBool(section.Config, "unwatched_only", false)
	ids, total, err := queries.SceneIDs(userID, filter, unwatchedOnly, order, sceneWidgetLimit(section, 12))
	if err != nil {
		return nil, fmt.Errorf("failed to list high resolution scenes: %w", err)
	}
	scenes, err := loadWidgetScenes(s, ids)
	if err != nil {
		return nil, err
	}
	return &HomepageSectionData{
		Section: section,
		Scenes:  scenes,
		Total:   total,
		Widget:  showcase4KPayload{MinHeight: filter.MinHeight, Total: total},
	}, nil
}
