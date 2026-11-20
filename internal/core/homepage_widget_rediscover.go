package core

import (
	"fmt"
	"time"

	"goonhub/internal/data"
)

// rediscover: the user's favorites (liked, rated >= min_rating or jizzed) not watched for at
// least `days`, strongest first.
// Config: days (default 180), min_rating (default 4, 0 disables), include_liked (default true),
// include_jizzed (default true).
// Widget payload: {"items": {scene_id: {"last_watched": time|null, "jizz_count", "rating", "liked"}}}

const (
	rediscoverDefaultDays      = 180
	rediscoverDefaultMinRating = 4
)

type rediscoverItem struct {
	LastWatched *time.Time `json:"last_watched"`
	JizzCount   int        `json:"jizz_count"`
	Rating      float64    `json:"rating"`
	Liked       bool       `json:"liked"`
	Views       int        `json:"views"`
}

type rediscoverPayload struct {
	Days  int                     `json:"days"`
	Items map[uint]rediscoverItem `json:"items"`
}

func init() {
	registerHomepageWidget("rediscover", HomepageWidgetSpec{
		Fetch:    fetchRediscoverWidget,
		Validate: validateRediscoverConfig,
	})
}

func rediscoverOptions(cfg map[string]interface{}, now time.Time) (data.RediscoverOptions, int) {
	days := clampInt(widgetConfigInt(cfg, "days", rediscoverDefaultDays), 1, 3650)
	minRating := float64(widgetConfigInt(cfg, "min_rating", rediscoverDefaultMinRating))
	if v, ok := cfg["min_rating"].(float64); ok {
		minRating = v
	}
	return data.RediscoverOptions{
		NotWatchedSince: now.AddDate(0, 0, -days),
		MinRating:       minRating,
		IncludeLiked:    widgetConfigBool(cfg, "include_liked", true),
		IncludeJizzed:   widgetConfigBool(cfg, "include_jizzed", true),
	}, days
}

func validateRediscoverConfig(cfg map[string]interface{}) error {
	days := widgetConfigInt(cfg, "days", rediscoverDefaultDays)
	if days < 1 || days > 3650 {
		return fmt.Errorf("days must be between 1 and 3650")
	}
	opts, _ := rediscoverOptions(cfg, time.Now())
	if opts.MinRating < 0 || opts.MinRating > 5 {
		return fmt.Errorf("min_rating must be between 0 and 5")
	}
	if !opts.IncludeLiked && !opts.IncludeJizzed && opts.MinRating <= 0 {
		return fmt.Errorf("select at least one criterion (liked, jizzed or a minimum rating)")
	}
	return nil
}

func fetchRediscoverWidget(s *HomepageService, userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	queries, err := sceneWidgetQueries(s)
	if err != nil {
		return nil, err
	}
	opts, days := rediscoverOptions(section.Config, time.Now())
	rows, err := queries.RediscoverScenes(userID, opts, sceneWidgetLimit(section, 12))
	if err != nil {
		return nil, fmt.Errorf("failed to list favorites to rediscover: %w", err)
	}
	ids := make([]uint, len(rows))
	items := make(map[uint]rediscoverItem, len(rows))
	for i, r := range rows {
		ids[i] = r.SceneID
		items[r.SceneID] = rediscoverItem{
			LastWatched: r.LastWatched,
			JizzCount:   r.JizzCount,
			Rating:      r.Rating,
			Liked:       r.Liked,
			Views:       r.Views,
		}
	}
	scenes, err := loadWidgetScenes(s, ids)
	if err != nil {
		return nil, err
	}
	return &HomepageSectionData{
		Section: section,
		Scenes:  scenes,
		Total:   int64(len(scenes)),
		Widget:  rediscoverPayload{Days: days, Items: items},
	}, nil
}
