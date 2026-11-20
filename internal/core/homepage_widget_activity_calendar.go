package core

import (
	"fmt"
	"time"

	"goonhub/internal/data"
)

// activity_calendar: heatmap of the user's views and/or o per day over the last weeks, with
// streaks.
//
// Config:
//
//	weeks  int     number of weeks shown (4-104, default 52)
//	metric string  views | jizz | both (default views)
//	tz     string  IANA timezone of the day boundaries (default: server TZ, else UTC)

const (
	activityCalendarDefaultWeeks = 52
	activityCalendarMinWeeks     = 4
	activityCalendarMaxWeeks     = 104
)

// Metrics of the time based stats widgets (views and dated o).
const (
	statsMetricViews = "views"
	statsMetricJizz  = "jizz"
	statsMetricBoth  = "both"
)

// ActivityCalendarDay is the count of a day.
type ActivityCalendarDay struct {
	Day   string `json:"day"`
	Count int64  `json:"count"`
}

// ActivityCalendarSeries is the heatmap of one metric. Days without activity are omitted.
type ActivityCalendarSeries struct {
	Days          []ActivityCalendarDay `json:"days"`
	Total         int64                 `json:"total"`
	ActiveDays    int                   `json:"active_days"`
	Max           int64                 `json:"max"`
	CurrentStreak int                   `json:"current_streak"`
	LongestStreak int                   `json:"longest_streak"`
}

// ActivityCalendarPayload is the payload of the activity_calendar widget. The calendar starts
// on Start (a Monday) and ends on Today. The top level figures are those of the first metric
// (views, unless metric is jizz); Series holds every requested metric.
type ActivityCalendarPayload struct {
	Weeks    int    `json:"weeks"`
	Start    string `json:"start"`
	Today    string `json:"today"`
	Metric   string `json:"metric"`
	Timezone string `json:"timezone"`
	ActivityCalendarSeries
	Series map[string]ActivityCalendarSeries `json:"series"`
}

func init() {
	registerHomepageWidget("activity_calendar", HomepageWidgetSpec{
		Fetch:    fetchActivityCalendarWidget,
		Validate: validateActivityCalendarWidget,
	})
}

// activityCalendarStart returns the Monday starting the calendar of `weeks` weeks ending today.
func activityCalendarStart(today time.Time, weeks int) time.Time {
	offset := (int(today.Weekday()) + 6) % 7 // days since Monday
	return today.AddDate(0, 0, -offset-7*(weeks-1))
}

// statsMetric reads the "metric" config of a time based stats widget.
func statsMetric(cfg map[string]interface{}, allowBoth bool) string {
	switch m := widgetConfigString(cfg, "metric", statsMetricViews); m {
	case statsMetricJizz:
		return m
	case statsMetricBoth:
		if allowBoth {
			return m
		}
	}
	return statsMetricViews
}

func validateStatsMetric(cfg map[string]interface{}, allowBoth bool) error {
	raw, ok := cfg["metric"]
	if !ok || raw == nil {
		return nil
	}
	m, isString := raw.(string)
	if !isString {
		return fmt.Errorf("metric must be a string")
	}
	switch m {
	case "", statsMetricViews, statsMetricJizz:
		return nil
	case statsMetricBoth:
		if allowBoth {
			return nil
		}
	}
	if allowBoth {
		return fmt.Errorf("metric must be views, jizz or both")
	}
	return fmt.Errorf("metric must be views or jizz")
}

// buildCalendarSeries builds the series of the calendar window from the whole daily history
// (ascending, one row per active day; the streaks use the whole history).
func buildCalendarSeries(all []data.DailyViewCount, start, today string) ActivityCalendarSeries {
	series := ActivityCalendarSeries{Days: []ActivityCalendarDay{}}
	activeDays := make([]string, 0, len(all))
	for _, d := range all {
		if d.Count <= 0 {
			continue
		}
		activeDays = append(activeDays, d.Day)
		if d.Day < start || d.Day > today {
			continue
		}
		series.Days = append(series.Days, ActivityCalendarDay{Day: d.Day, Count: d.Count})
		series.Total += d.Count
		series.ActiveDays++
		if d.Count > series.Max {
			series.Max = d.Count
		}
	}
	series.CurrentStreak, series.LongestStreak, _, _ = viewStreaks(activeDays, today)
	return series
}

func fetchActivityCalendarWidget(s *HomepageService, userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	db := s.WidgetDeps().DB
	if db == nil {
		return nil, fmt.Errorf("activity calendar widget: database not available")
	}
	cfg := section.Config
	weeks := clampInt(widgetConfigInt(cfg, "weeks", activityCalendarDefaultWeeks), activityCalendarMinWeeks, activityCalendarMaxWeeks)
	metric := statsMetric(cfg, true)
	tz, loc := statsWidgetTimezone(cfg)

	today := statsStartOfDay(time.Now(), loc)
	start := activityCalendarStart(today, weeks)

	payload := ActivityCalendarPayload{
		Weeks:    weeks,
		Start:    start.Format(statsWidgetDayLayout),
		Today:    today.Format(statsWidgetDayLayout),
		Metric:   metric,
		Timezone: tz,
		Series:   map[string]ActivityCalendarSeries{},
	}

	var order []string
	if metric == statsMetricViews || metric == statsMetricBoth {
		order = append(order, statsMetricViews)
	}
	if metric == statsMetricJizz || metric == statsMetricBoth {
		order = append(order, statsMetricJizz)
	}
	for _, m := range order {
		var all []data.DailyViewCount
		var err error
		if m == statsMetricJizz {
			all, err = data.QueryDailyJizz(db, userID, tz, time.Time{})
		} else {
			all, err = data.QueryDailyViews(db, userID, tz, time.Time{})
		}
		if err != nil {
			return nil, fmt.Errorf("activity calendar widget: %w", err)
		}
		payload.Series[m] = buildCalendarSeries(all, payload.Start, payload.Today)
	}
	payload.ActivityCalendarSeries = payload.Series[order[0]]
	return emptyWidgetData(section, payload), nil
}

func validateActivityCalendarWidget(cfg map[string]interface{}) error {
	if err := validateStatsIntRange(cfg, "weeks", activityCalendarMinWeeks, activityCalendarMaxWeeks); err != nil {
		return fmt.Errorf("activity calendar: %w", err)
	}
	if err := validateStatsMetric(cfg, true); err != nil {
		return fmt.Errorf("activity calendar: %w", err)
	}
	return validateStatsTimezone(cfg)
}
