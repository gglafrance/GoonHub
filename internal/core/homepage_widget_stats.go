package core

import (
	"fmt"
	"time"

	"goonhub/internal/data"
)

// stats: library and personal figures as tiles.
//
// Config:
//
//	tiles  []string  tiles to show, in order (default: all, see statsWidgetTiles)
//	period string    "month" (calendar month, default), "week" (ISO week), "30d" or "7d":
//	                 period of the views figure, compared to the previous period
//	tz     string    IANA timezone for the period boundaries (default: server TZ, else UTC)

var statsWidgetTiles = []string{"scenes", "hours", "size", "watched", "views", "jizz", "liked", "markers", "rating"}

var statsWidgetPeriods = map[string]bool{"month": true, "week": true, "30d": true, "7d": true}

// StatsWidgetPayload is the payload of the stats widget.
type StatsWidgetPayload struct {
	Tiles           []string `json:"tiles"`
	Period          string   `json:"period"`
	SceneCount      int64    `json:"scene_count"`
	TotalDuration   int64    `json:"total_duration"`
	TotalSize       int64    `json:"total_size"`
	WatchedScenes   int64    `json:"watched_scenes"`
	WatchedPercent  float64  `json:"watched_percent"`
	ViewsPeriod     int64    `json:"views_period"`
	ViewsPrevPeriod int64    `json:"views_prev_period"`
	JizzTotal       int64    `json:"jizz_total"`
	LikedCount      int64    `json:"liked_count"`
	MarkerCount     int64    `json:"marker_count"`
	RatedCount      int64    `json:"rated_count"`
	AvgRating       *float64 `json:"avg_rating"`
}

func init() {
	registerHomepageWidget("stats", HomepageWidgetSpec{Fetch: fetchStatsWidget, Validate: validateStatsWidget})
}

// statsPeriodBounds returns the start of the current and previous periods.
func statsPeriodBounds(period string, now time.Time, loc *time.Location) (prevStart, start time.Time) {
	today := startOfDay(now, loc)
	switch period {
	case "week":
		offset := (int(today.Weekday()) + 6) % 7 // days since Monday
		start = today.AddDate(0, 0, -offset)
		return start.AddDate(0, 0, -7), start
	case "30d":
		start = now.Add(-30 * 24 * time.Hour)
		return start.Add(-30 * 24 * time.Hour), start
	case "7d":
		start = now.Add(-7 * 24 * time.Hour)
		return start.Add(-7 * 24 * time.Hour), start
	default: // month
		start = time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, loc)
		return start.AddDate(0, -1, 0), start
	}
}

func statsWidgetSelectedTiles(cfg map[string]interface{}) []string {
	selected := widgetConfigStrings(cfg, "tiles")
	if len(selected) == 0 {
		return statsWidgetTiles
	}
	valid := make(map[string]bool, len(statsWidgetTiles))
	for _, t := range statsWidgetTiles {
		valid[t] = true
	}
	seen := make(map[string]bool)
	out := make([]string, 0, len(selected))
	for _, t := range selected {
		if valid[t] && !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return statsWidgetTiles
	}
	return out
}

func fetchStatsWidget(s *HomepageService, userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	db := s.WidgetDeps().DB
	if db == nil {
		return nil, fmt.Errorf("stats widget: database not available")
	}
	cfg := section.Config
	period := widgetConfigString(cfg, "period", "month")
	if !statsWidgetPeriods[period] {
		period = "month"
	}
	_, loc := statsWidgetTimezone(cfg)
	prevStart, start := statsPeriodBounds(period, time.Now(), loc)

	stats, err := data.QueryLibraryStats(db, userID, prevStart, start)
	if err != nil {
		return nil, fmt.Errorf("stats widget: %w", err)
	}

	payload := StatsWidgetPayload{
		Tiles:           statsWidgetSelectedTiles(cfg),
		Period:          period,
		SceneCount:      stats.SceneCount,
		TotalDuration:   stats.TotalDuration,
		TotalSize:       stats.TotalSize,
		WatchedScenes:   stats.WatchedScenes,
		ViewsPeriod:     stats.ViewsPeriod,
		ViewsPrevPeriod: stats.ViewsPrevPeriod,
		JizzTotal:       stats.JizzTotal,
		LikedCount:      stats.LikedCount,
		MarkerCount:     stats.MarkerCount,
		RatedCount:      stats.RatedCount,
		AvgRating:       stats.AvgRating,
	}
	if stats.SceneCount > 0 {
		payload.WatchedPercent = float64(stats.WatchedScenes) * 100 / float64(stats.SceneCount)
	}
	return emptyWidgetData(section, payload), nil
}

func validateStatsWidget(cfg map[string]interface{}) error {
	if raw, ok := cfg["period"]; ok && raw != nil {
		period, isString := raw.(string)
		if !isString || (period != "" && !statsWidgetPeriods[period]) {
			return fmt.Errorf("stats: period must be month, week, 30d or 7d")
		}
	}
	if raw, ok := cfg["tiles"]; ok && raw != nil {
		list, isList := raw.([]interface{})
		if !isList {
			return fmt.Errorf("stats: tiles must be a list")
		}
		valid := make(map[string]bool, len(statsWidgetTiles))
		for _, t := range statsWidgetTiles {
			valid[t] = true
		}
		for _, item := range list {
			name, isString := item.(string)
			if !isString || !valid[name] {
				return fmt.Errorf("stats: unknown tile %v", item)
			}
		}
	}
	return validateStatsTimezone(cfg)
}
