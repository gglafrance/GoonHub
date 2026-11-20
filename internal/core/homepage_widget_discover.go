package core

import (
	"fmt"

	"goonhub/internal/data"
)

// discover: random scenes the user has never watched.
// Config: min_duration, max_duration, storage_path_id, tags (names, any of them).
// Widget payload: {"unwatched_total": N} (scenes matching the filters never watched).

type discoverPayload struct {
	UnwatchedTotal int64 `json:"unwatched_total"`
}

func init() {
	registerHomepageWidget("discover", HomepageWidgetSpec{
		Fetch:    fetchDiscoverWidget,
		Validate: validateSceneWidgetFilter,
	})
}

func fetchDiscoverWidget(s *HomepageService, userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	queries, err := sceneWidgetQueries(s)
	if err != nil {
		return nil, err
	}
	filter := sceneWidgetFilter(section.Config)
	ids, total, err := queries.SceneIDs(userID, filter, true, data.HomepageSceneOrderRandom, sceneWidgetLimit(section, 12))
	if err != nil {
		return nil, fmt.Errorf("failed to pick unwatched scenes: %w", err)
	}
	scenes, err := loadWidgetScenes(s, ids)
	if err != nil {
		return nil, err
	}
	return &HomepageSectionData{
		Section: section,
		Scenes:  scenes,
		Total:   total,
		Widget:  discoverPayload{UnwatchedTotal: total},
	}, nil
}
