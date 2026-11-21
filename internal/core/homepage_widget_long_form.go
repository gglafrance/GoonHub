package core

import (
	"fmt"

	"goonhub/internal/data"
)

// long_form: long scenes, in progress first (latest watch stopped before 90%), then random
// unwatched ones.
// Config: min_duration (default 1200 s), max_duration, tags, storage_path_id,
// include_in_progress (default true), include_unwatched (default true).
// Widget payload: {"min_duration": N, "in_progress": [ids]}

const (
	longFormDefaultMinDuration = 1200
	longFormMaxProgress        = 0.9
)

type longFormPayload struct {
	MinDuration int    `json:"min_duration"`
	InProgress  []uint `json:"in_progress"`
}

func init() {
	registerHomepageWidget("long_form", HomepageWidgetSpec{
		Fetch: fetchLongFormWidget,
		Validate: func(cfg map[string]interface{}) error {
			if err := validateSceneWidgetFilter(cfg); err != nil {
				return err
			}
			if !widgetConfigBool(cfg, "include_in_progress", true) && !widgetConfigBool(cfg, "include_unwatched", true) {
				return fmt.Errorf("include in progress or unwatched scenes")
			}
			return nil
		},
	})
}

func fetchLongFormWidget(s *HomepageService, userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	queries, err := sceneWidgetQueries(s)
	if err != nil {
		return nil, err
	}
	filter := sceneWidgetFilter(section.Config)
	if filter.MinDuration <= 0 {
		filter.MinDuration = longFormDefaultMinDuration
	}
	limit := sceneWidgetLimit(section, 12)

	var ids []uint
	positions := map[uint]int{}
	payload := longFormPayload{MinDuration: filter.MinDuration, InProgress: []uint{}}
	if widgetConfigBool(section.Config, "include_in_progress", true) {
		rows, err := queries.InProgressScenes(userID, filter, longFormMaxProgress, limit)
		if err != nil {
			return nil, fmt.Errorf("failed to list scenes in progress: %w", err)
		}
		for _, r := range rows {
			ids = append(ids, r.SceneID)
			positions[r.SceneID] = r.LastPosition
			payload.InProgress = append(payload.InProgress, r.SceneID)
		}
	}
	var total int64 = int64(len(ids))
	if remaining := limit - len(ids); remaining > 0 && widgetConfigBool(section.Config, "include_unwatched", true) {
		unwatched, unwatchedTotal, err := queries.SceneIDs(userID, filter, true, data.HomepageSceneOrderRandom, remaining)
		if err != nil {
			return nil, fmt.Errorf("failed to list unwatched long scenes: %w", err)
		}
		ids = append(ids, unwatched...)
		total += unwatchedTotal
	}

	scenes, err := loadWidgetScenes(s, ids)
	if err != nil {
		return nil, err
	}
	progress := make(map[uint]WatchProgress, len(positions))
	for _, scene := range scenes {
		if pos, ok := positions[scene.ID]; ok {
			progress[scene.ID] = WatchProgress{LastPosition: pos, Duration: scene.Duration}
		}
	}
	return &HomepageSectionData{
		Section:       section,
		Scenes:        scenes,
		Total:         total,
		WatchProgress: progress,
		Widget:        payload,
	}, nil
}
