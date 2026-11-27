package core

import (
	"fmt"

	"goonhub/internal/data"
)

// Helpers shared by the scene based widgets (discover, rediscover, roulette, on_this_day,
// folder_latest, because_you_watched, showcase_4k, long_form).

// maxSceneWidgetDuration bounds the duration filters (one week, in seconds).
const maxSceneWidgetDuration = 7 * 24 * 3600

// sceneWidgetQueries returns the widget queries, or an error when the database is not wired.
func sceneWidgetQueries(s *HomepageService) (*data.HomepageSceneQueries, error) {
	db := s.WidgetDeps().DB
	if db == nil {
		return nil, fmt.Errorf("homepage widget database is not configured")
	}
	return data.NewHomepageSceneQueries(db), nil
}

// sceneWidgetFilter reads the common filter keys of a section config:
// min_duration, max_duration (seconds), min_height, storage_path_id, folder, tags. The
// storage path and folder filters (path oracles) are dropped for a user without
// files:view_paths.
func sceneWidgetFilter(s *HomepageService, userID uint, cfg map[string]interface{}) data.HomepageSceneFilter {
	f := sceneWidgetFilterFromConfig(cfg)
	if !s.userHas(userID, PermFilesViewPaths) {
		f.StoragePathID = 0
		f.Folder = ""
	}
	return f
}

// sceneWidgetFilterFromConfig reads the filter keys as configured.
func sceneWidgetFilterFromConfig(cfg map[string]interface{}) data.HomepageSceneFilter {
	return data.HomepageSceneFilter{
		MinDuration:   max(widgetConfigInt(cfg, "min_duration", 0), 0),
		MaxDuration:   max(widgetConfigInt(cfg, "max_duration", 0), 0),
		MinHeight:     max(widgetConfigInt(cfg, "min_height", 0), 0),
		StoragePathID: uint(max(widgetConfigInt(cfg, "storage_path_id", 0), 0)),
		Folder:        widgetConfigString(cfg, "folder", ""),
		Tags:          widgetConfigStrings(cfg, "tags"),
	}
}

// validateSceneWidgetFilter checks the common filter keys.
func validateSceneWidgetFilter(cfg map[string]interface{}) error {
	minDuration := widgetConfigInt(cfg, "min_duration", 0)
	maxDuration := widgetConfigInt(cfg, "max_duration", 0)
	if minDuration < 0 || maxDuration < 0 || minDuration > maxSceneWidgetDuration || maxDuration > maxSceneWidgetDuration {
		return fmt.Errorf("durations must be between 0 and %d seconds", maxSceneWidgetDuration)
	}
	if maxDuration > 0 && minDuration > maxDuration {
		return fmt.Errorf("min_duration must not exceed max_duration")
	}
	if widgetConfigInt(cfg, "min_height", 0) < 0 {
		return fmt.Errorf("min_height must not be negative")
	}
	if widgetConfigInt(cfg, "storage_path_id", 0) < 0 {
		return fmt.Errorf("storage_path_id must not be negative")
	}
	if len(widgetConfigStrings(cfg, "tags")) > 50 {
		return fmt.Errorf("at most 50 tags")
	}
	if len(widgetConfigString(cfg, "folder", "")) > 500 {
		return fmt.Errorf("folder must be at most 500 characters")
	}
	return nil
}

// sceneWidgetLimit returns the section limit with a fallback.
func sceneWidgetLimit(section data.HomepageSection, def int) int {
	if section.Limit <= 0 {
		return def
	}
	return clampInt(section.Limit, 1, 50)
}

// loadWidgetScenes loads scenes by ID, keeping the given order (trashed scenes are skipped).
func loadWidgetScenes(s *HomepageService, ids []uint) ([]data.Scene, error) {
	if len(ids) == 0 {
		return []data.Scene{}, nil
	}
	scenes, err := s.sceneRepo.GetByIDs(ids)
	if err != nil {
		return nil, fmt.Errorf("failed to load scenes: %w", err)
	}
	if scenes == nil {
		scenes = []data.Scene{}
	}
	return scenes, nil
}
