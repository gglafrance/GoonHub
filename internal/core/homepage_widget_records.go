package core

import (
	"fmt"
	"time"

	"goonhub/internal/data"
)

// records: the user's viewing records and library extremes.
//
// Config:
//
//	tz string  IANA timezone of the day based records (default: server TZ, else UTC)

// RecordSceneRef is a scene referenced by a record.
type RecordSceneRef struct {
	ID            uint   `json:"id"`
	Title         string `json:"title"`
	ThumbnailPath string `json:"thumbnail_path"`
	Value         int64  `json:"value"`
}

// RecordStreak is the longest run of consecutive days with views.
type RecordStreak struct {
	Days  int    `json:"days"`
	Start string `json:"start"`
	End   string `json:"end"`
}

// RecordDay is the day with the most views.
type RecordDay struct {
	Day   string `json:"day"`
	Count int64  `json:"count"`
}

// RecordsPayload is the payload of the records widget (absent records are null).
type RecordsPayload struct {
	MostRewatched  *RecordSceneRef `json:"most_rewatched"`
	MostJizzed     *RecordSceneRef `json:"most_jizzed"`
	LongestScene   *RecordSceneRef `json:"longest_scene"`
	BiggestFile    *RecordSceneRef `json:"biggest_file"`
	LongestStreak  *RecordStreak   `json:"longest_streak"`
	BusiestDay     *RecordDay      `json:"busiest_day"`
	FirstView      *time.Time      `json:"first_view"`
	TotalWatchTime int64           `json:"total_watch_time"`
	TotalViews     int64           `json:"total_views"`
	Timezone       string          `json:"timezone"`
}

func init() {
	registerHomepageWidget("records", HomepageWidgetSpec{
		Fetch:    fetchRecordsWidget,
		Validate: validateStatsTimezone,
	})
}

func toRecordSceneRef(r *data.RecordScene) *RecordSceneRef {
	if r == nil {
		return nil
	}
	return &RecordSceneRef{ID: r.ID, Title: r.Title, ThumbnailPath: r.ThumbnailPath, Value: r.Value}
}

// buildDayRecords computes the longest streak and the busiest day from the daily views.
func buildDayRecords(days []data.DailyViewCount, today string) (*RecordStreak, *RecordDay) {
	active := make([]string, 0, len(days))
	var busiest *RecordDay
	for _, d := range days {
		if d.Count <= 0 {
			continue
		}
		active = append(active, d.Day)
		if busiest == nil || d.Count > busiest.Count {
			busiest = &RecordDay{Day: d.Day, Count: d.Count}
		}
	}
	_, longest, start, end := viewStreaks(active, today)
	if longest == 0 {
		return nil, busiest
	}
	return &RecordStreak{Days: longest, Start: start, End: end}, busiest
}

func fetchRecordsWidget(s *HomepageService, userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	db := s.WidgetDeps().DB
	if db == nil {
		return nil, fmt.Errorf("records widget: database not available")
	}
	tz, loc := statsWidgetTimezone(section.Config)

	records, err := data.QueryWatchRecords(db, userID)
	if err != nil {
		return nil, fmt.Errorf("records widget: %w", err)
	}
	days, err := data.QueryDailyViews(db, userID, tz, time.Time{})
	if err != nil {
		return nil, fmt.Errorf("records widget: %w", err)
	}
	streak, busiest := buildDayRecords(days, startOfDay(time.Now(), loc).Format(statsWidgetDayLayout))

	payload := RecordsPayload{
		MostRewatched:  toRecordSceneRef(records.MostRewatched),
		MostJizzed:     toRecordSceneRef(records.MostJizzed),
		LongestScene:   toRecordSceneRef(records.LongestScene),
		BiggestFile:    toRecordSceneRef(records.BiggestFile),
		LongestStreak:  streak,
		BusiestDay:     busiest,
		FirstView:      records.FirstView,
		TotalWatchTime: records.TotalWatchTime,
		TotalViews:     records.TotalViews,
		Timezone:       tz,
	}
	return emptyWidgetData(section, payload), nil
}
