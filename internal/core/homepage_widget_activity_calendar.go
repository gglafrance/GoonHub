package core

import (
	"fmt"
	"time"

	"goonhub/internal/data"
)

// activity_calendar: heatmap of the user's views per day over the last weeks, with streaks.
//
// Config:
//
//	weeks int     number of weeks shown (4-104, default 52)
//	tz    string  IANA timezone of the day boundaries (default: server TZ, else UTC)

const (
	activityCalendarDefaultWeeks = 52
	activityCalendarMinWeeks     = 4
	activityCalendarMaxWeeks     = 104
)

// ActivityCalendarDay is the view count of a day.
type ActivityCalendarDay struct {
	Day   string `json:"day"`
	Count int64  `json:"count"`
}

// ActivityCalendarPayload is the payload of the activity_calendar widget. Days without
// views are omitted; the calendar starts on Start (a Monday) and ends on Today.
type ActivityCalendarPayload struct {
	Weeks         int                   `json:"weeks"`
	Start         string                `json:"start"`
	Today         string                `json:"today"`
	Days          []ActivityCalendarDay `json:"days"`
	Total         int64                 `json:"total"`
	ActiveDays    int                   `json:"active_days"`
	Max           int64                 `json:"max"`
	CurrentStreak int                   `json:"current_streak"`
	LongestStreak int                   `json:"longest_streak"`
	Timezone      string                `json:"timezone"`
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

func fetchActivityCalendarWidget(s *HomepageService, userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	db := s.WidgetDeps().DB
	if db == nil {
		return nil, fmt.Errorf("activity calendar widget: database not available")
	}
	cfg := section.Config
	weeks := clampInt(widgetConfigInt(cfg, "weeks", activityCalendarDefaultWeeks), activityCalendarMinWeeks, activityCalendarMaxWeeks)
	tz, loc := statsWidgetTimezone(cfg)

	today := startOfDay(time.Now(), loc)
	start := activityCalendarStart(today, weeks)

	// Whole history (one row per active day) for the streaks; the calendar is a subset of it
	all, err := data.QueryDailyViews(db, userID, tz, time.Time{})
	if err != nil {
		return nil, fmt.Errorf("activity calendar widget: %w", err)
	}

	payload := ActivityCalendarPayload{
		Weeks:    weeks,
		Start:    start.Format(statsWidgetDayLayout),
		Today:    today.Format(statsWidgetDayLayout),
		Days:     []ActivityCalendarDay{},
		Timezone: tz,
	}
	activeDays := make([]string, 0, len(all))
	for _, d := range all {
		if d.Count <= 0 {
			continue
		}
		activeDays = append(activeDays, d.Day)
		if d.Day < payload.Start || d.Day > payload.Today {
			continue
		}
		payload.Days = append(payload.Days, ActivityCalendarDay{Day: d.Day, Count: d.Count})
		payload.Total += d.Count
		payload.ActiveDays++
		if d.Count > payload.Max {
			payload.Max = d.Count
		}
	}
	payload.CurrentStreak, payload.LongestStreak, _, _ = viewStreaks(activeDays, payload.Today)
	return emptyWidgetData(section, payload), nil
}

func validateActivityCalendarWidget(cfg map[string]interface{}) error {
	if err := validateStatsIntRange(cfg, "weeks", activityCalendarMinWeeks, activityCalendarMaxWeeks); err != nil {
		return fmt.Errorf("activity calendar: %w", err)
	}
	return validateStatsTimezone(cfg)
}
