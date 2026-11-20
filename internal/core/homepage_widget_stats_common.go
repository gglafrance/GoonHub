package core

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Helpers shared by the stats widgets (stats, activity_calendar, watch_times, records).

const statsWidgetDayLayout = "2006-01-02"

// statsWidgetTimezone returns the IANA timezone of a stats widget: the "tz" config when
// valid, else the server TZ environment variable, else UTC.
func statsWidgetTimezone(cfg map[string]interface{}) (string, *time.Location) {
	if name := widgetConfigString(cfg, "tz", ""); name != "" {
		if loc, err := loadStatsTimezone(name); err == nil {
			return name, loc
		}
	}
	if name := strings.TrimSpace(os.Getenv("TZ")); name != "" {
		if loc, err := loadStatsTimezone(name); err == nil {
			return name, loc
		}
	}
	return "UTC", time.UTC
}

// loadStatsTimezone loads an IANA zone name; "Local" and empty names are rejected because
// PostgreSQL does not know them.
func loadStatsTimezone(name string) (*time.Location, error) {
	if name == "" || name == "Local" || len(name) > 64 {
		return nil, fmt.Errorf("invalid timezone %q", name)
	}
	return time.LoadLocation(name)
}

// validateStatsTimezone validates the optional "tz" config key.
func validateStatsTimezone(cfg map[string]interface{}) error {
	raw, ok := cfg["tz"]
	if !ok || raw == nil {
		return nil
	}
	name, isString := raw.(string)
	if !isString {
		return fmt.Errorf("tz must be a timezone name")
	}
	if strings.TrimSpace(name) == "" {
		return nil
	}
	if _, err := loadStatsTimezone(strings.TrimSpace(name)); err != nil {
		return fmt.Errorf("unknown timezone %q", name)
	}
	return nil
}

// validateStatsIntRange validates an optional numeric config key.
func validateStatsIntRange(cfg map[string]interface{}, key string, lo, hi int) error {
	raw, ok := cfg[key]
	if !ok || raw == nil {
		return nil
	}
	v, isNumber := raw.(float64)
	if !isNumber {
		if i, isInt := raw.(int); isInt {
			v = float64(i)
		} else {
			return fmt.Errorf("%s must be a number", key)
		}
	}
	if v < float64(lo) || v > float64(hi) {
		return fmt.Errorf("%s must be between %d and %d", key, lo, hi)
	}
	return nil
}

// statsStartOfDay returns local midnight of t in loc.
func statsStartOfDay(t time.Time, loc *time.Location) time.Time {
	l := t.In(loc)
	return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, loc)
}

// viewStreaks computes the current and longest runs of consecutive active days. days are
// distinct YYYY-MM-DD local dates in ascending order; today is the local date. The current
// streak counts back from today, or from yesterday when nothing was watched today yet.
func viewStreaks(days []string, today string) (current, longest int, longestStart, longestEnd string) {
	var prev time.Time
	run := 0
	runStart := ""
	for _, d := range days {
		t, err := time.Parse(statsWidgetDayLayout, d)
		if err != nil {
			continue
		}
		if run > 0 && t.Sub(prev) == 24*time.Hour {
			run++
		} else {
			run = 1
			runStart = d
		}
		if run > longest {
			longest, longestStart, longestEnd = run, runStart, d
		}
		prev = t
	}

	todayT, err := time.Parse(statsWidgetDayLayout, today)
	if err != nil || len(days) == 0 {
		return 0, longest, longestStart, longestEnd
	}
	active := make(map[string]bool, len(days))
	for _, d := range days {
		active[d] = true
	}
	cursor := todayT
	if !active[today] {
		cursor = todayT.AddDate(0, 0, -1)
	}
	for active[cursor.Format(statsWidgetDayLayout)] {
		current++
		cursor = cursor.AddDate(0, 0, -1)
	}
	return current, longest, longestStart, longestEnd
}
