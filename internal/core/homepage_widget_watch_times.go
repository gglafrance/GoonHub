package core

import (
	"fmt"
	"time"

	"goonhub/internal/data"
)

// watch_times: the user's views by hour of day and by day of week.
//
// Config:
//
//	days int     only count views of the last N days (0 or absent: whole history, max 3650)
//	tz   string  IANA timezone of the hours (default: server TZ, else UTC)

const watchTimesMaxDays = 3650

// WatchTimesPayload is the payload of the watch_times widget.
type WatchTimesPayload struct {
	// Hours[h] is the view count at local hour h (0-23)
	Hours [24]int64 `json:"hours"`
	// Weekdays[d] is the view count on day d, Monday = 0 ... Sunday = 6
	Weekdays    [7]int64 `json:"weekdays"`
	Total       int64    `json:"total"`
	PeakHour    int      `json:"peak_hour"`
	PeakWeekday int      `json:"peak_weekday"`
	Days        int      `json:"days"`
	Timezone    string   `json:"timezone"`
}

func init() {
	registerHomepageWidget("watch_times", HomepageWidgetSpec{
		Fetch:    fetchWatchTimesWidget,
		Validate: validateWatchTimesWidget,
	})
}

// buildWatchTimesPayload folds the SQL buckets into fixed arrays and finds the peaks.
func buildWatchTimesPayload(hours, weekdays []data.BucketCount) WatchTimesPayload {
	var p WatchTimesPayload
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
	payload.Days = days
	payload.Timezone = tz
	return emptyWidgetData(section, payload), nil
}

func validateWatchTimesWidget(cfg map[string]interface{}) error {
	if err := validateStatsIntRange(cfg, "days", 0, watchTimesMaxDays); err != nil {
		return fmt.Errorf("watch times: %w", err)
	}
	return validateStatsTimezone(cfg)
}
