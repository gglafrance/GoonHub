package core

import (
	"fmt"

	"goonhub/internal/data"
)

// roulette: one random scene matching the criteria, played with a "Surprise me" button.
// Config: unwatched_only (default true), min_duration, max_duration, tags, storage_path_id.
// Widget payload: {"candidates": N}

type roulettePayload struct {
	Candidates int64 `json:"candidates"`
}

func init() {
	registerHomepageWidget("roulette", HomepageWidgetSpec{
		Fetch:    fetchRouletteWidget,
		Validate: validateSceneWidgetFilter,
	})
}

func fetchRouletteWidget(s *HomepageService, userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	queries, err := sceneWidgetQueries(s)
	if err != nil {
		return nil, err
	}
	unwatchedOnly := widgetConfigBool(section.Config, "unwatched_only", true)
	ids, total, err := queries.SceneIDs(userID, sceneWidgetFilter(section.Config), unwatchedOnly, data.HomepageSceneOrderRandom, 1)
	if err != nil {
		return nil, fmt.Errorf("failed to pick a random scene: %w", err)
	}
	scenes, err := loadWidgetScenes(s, ids)
	if err != nil {
		return nil, err
	}
	return &HomepageSectionData{
		Section: section,
		Scenes:  scenes,
		Total:   total,
		Widget:  roulettePayload{Candidates: total},
	}, nil
}
