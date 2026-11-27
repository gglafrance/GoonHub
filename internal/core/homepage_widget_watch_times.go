package core

import (
	"fmt"
	"time"

	"goonhub/internal/data"
)

// watch_times: the user's views and dated o by hour of day and by day of week.
//
// Config:
//
//	days   int     only count the last N days (0 or absent: whole history, max 3650)
//	metric string  series shown first: views (default) or jizz; both series are always sent
//	tz     string  IANA timezone of the hours (default: server TZ, else UTC)

const watchTimesMaxDays = 3650

// WatchTimesSeries is one metric by hour and weekday.
type WatchTimesSeries struct {
	// Hours[h] is the count at local hour h (0-23)
	Hours [24]int64 `json:"hours"`
	// Weekdays[d] is the count on day d, Monday = 0 ... Sunday = 6
	Weekdays    [7]int64 `json:"weekdays"`
	Total       int64    `json:"total"`
	PeakHour    int      `json:"peak_hour"`
	PeakWeekday int      `json:"peak_weekday"`
}

// WatchTimesPayload is the payload of the watch_times widget: the view series at the top
// level, the dated o series in Jizz.
type WatchTimesPayload struct {
	WatchTimesSeries
	Jizz     WatchTimesSeries `json:"jizz"`
	Metric   string           `json:"metric"`
	Days     int              `json:"days"`
	Timezone string           `json:"timezone"`
}

func init() {
	registerHomepageWidget("watch_times", HomepageWidgetSpec{
		Fetch:    fetchWatchTimesWidget,
		Validate: validateWatchTimesWidget,
	})
}

// buildWatchTimesPayload folds the SQL buckets into fixed arrays and finds the peaks.
func buildWatchTimesPayload(hours, weekdays []data.BucketCount) WatchTimesPayload {
	return WatchTimesPayload{WatchTimesSeries: buildWatchTimesSeries(hours, weekdays)}
}

// buildWatchTimesSeries folds the SQL buckets of one metric into fixed arrays with peaks.
func buildWatchTimesSeries(hours, weekdays []data.BucketCount) WatchTimesSeries {
	var p WatchTimesSeries
	for _, h := range hours {
		if h.Bucket >= 0 && h.Bucket < 24 {
			p.Hours[h.Bucket] += h.Count
			p.Total += h.Count
		}
	}
	for _, d := range weekdays {
		if d.Bucket >= 1 && d.Bucket <= 7 {
			p.Weekdays[d.Bucket-1] += d.Count
		}
	}
	for h := range p.Hours {
		if p.Hours[h] > p.Hours[p.PeakHour] {
			p.PeakHour = h
		}
	}
	for d := range p.Weekdays {
		if p.Weekdays[d] > p.Weekdays[p.PeakWeekday] {
			p.PeakWeekday = d
		}
	}
	return p
}

func fetchWatchTimesWidget(s *HomepageService, userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	db := s.WidgetDeps().DB
	if db == nil {
		return nil, fmt.Errorf("watch times widget: database not available")
	}
	cfg := section.Config
	days := clampInt(widgetConfigInt(cfg, "days", 0), 0, watchTimesMaxDays)
	tz, _ := statsWidgetTimezone(cfg)

	var since time.Time
	if days > 0 {
		since = time.Now().AddDate(0, 0, -days)
	}
	hours, weekdays, err := data.QueryViewsByHourAndWeekday(db, userID, tz, since)
	if err != nil {
		return nil, fmt.Errorf("watch times widget: %w", err)
	}
	payload := buildWatchTimesPayload(hours, weekdays)
	jizzHours, jizzWeekdays, err := data.QueryJizzByHourAndWeekday(db, userID, tz, since)
	if err != nil {
		return nil, fmt.Errorf("watch times widget: %w", err)
	}
	payload.Jizz = buildWatchTimesSeries(jizzHours, jizzWeekdays)
	payload.Days = days
	payload.Metric = statsMetric(cfg, false)
	payload.Timezone = tz
	return emptyWidgetData(section, payload), nil
}

func validateWatchTimesWidget(cfg map[string]interface{}) error {
	if err := validateStatsIntRange(cfg, "days", 0, watchTimesMaxDays); err != nil {
		return fmt.Errorf("watch times: %w", err)
	}
	if err := validateStatsMetric(cfg, false); err != nil {
		return fmt.Errorf("watch times: %w", err)
	}
	return validateStatsTimezone(cfg)
}
