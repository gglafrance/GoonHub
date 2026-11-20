package core

import (
	"fmt"
	"time"

	"goonhub/internal/data"
)

// jizz_insights: the user's dated o over the last months: monthly bars, totals, favorite hour
// and weekday, top scenes and the share of the library already o'd.
//
// Config:
//
//	months int     number of months shown, current month included (3-120, default 24)
//	tz     string  IANA timezone of the month, hour and weekday buckets (default: server TZ,
//	               else UTC)

const (
	jizzInsightsDefaultMonths = 24
	jizzInsightsMinMonths     = 3
	jizzInsightsMaxMonths     = 120
	jizzInsightsTopScenes     = 5
)

// JizzInsightsMonth is the o count of a local month (YYYY-MM).
type JizzInsightsMonth struct {
	Month string `json:"month"`
	Count int64  `json:"count"`
}

// JizzInsightsPayload is the payload of the jizz_insights widget. Figures cover the window
// of Months months (from Start, the first day of the oldest month) unless noted.
type JizzInsightsPayload struct {
	Months      int                 `json:"months"`
	Start       string              `json:"start"`
	Series      []JizzInsightsMonth `json:"series"`
	Total       int64               `json:"total"`
	AvgPerMonth float64             `json:"avg_per_month"`
	Max         int64               `json:"max"`
	// FavoriteHour (0-23) and FavoriteWeekday (Monday = 0) are -1 without dated o
	FavoriteHour    int                   `json:"favorite_hour"`
	FavoriteWeekday int                   `json:"favorite_weekday"`
	TopScenes       []data.JizzSceneCount `json:"top_scenes"`
	// Library share over all time, dated o or not
	JizzedScenes int64   `json:"jizzed_scenes"`
	LibraryCount int64   `json:"library_count"`
	JizzedShare  float64 `json:"jizzed_share"`
	Timezone     string  `json:"timezone"`
}

func init() {
	registerHomepageWidget("jizz_insights", HomepageWidgetSpec{
		Fetch:    fetchJizzInsightsWidget,
		Validate: validateJizzInsightsWidget,
	})
}

// jizzInsightsStart returns the first day of the oldest month of a window of `months` months
// ending with the month of now.
func jizzInsightsStart(now time.Time, months int, loc *time.Location) time.Time {
	local := now.In(loc)
	return time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, loc).AddDate(0, -(months - 1), 0)
}

// buildJizzMonthSeries fills every month of the window (oldest first), missing months at 0.
func buildJizzMonthSeries(rows []data.DailyViewCount, start time.Time, months int) ([]JizzInsightsMonth, int64, int64) {
	counts := make(map[string]int64, len(rows))
	for _, r := range rows {
		counts[r.Day] = r.Count
	}
	series := make([]JizzInsightsMonth, months)
	var total, max int64
	for i := 0; i < months; i++ {
		month := start.AddDate(0, i, 0).Format("2006-01")
		c := counts[month]
		series[i] = JizzInsightsMonth{Month: month, Count: c}
		total += c
		if c > max {
			max = c
		}
	}
	return series, total, max
}

func fetchJizzInsightsWidget(s *HomepageService, userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	db := s.WidgetDeps().DB
	if db == nil {
		return nil, fmt.Errorf("jizz insights widget: database not available")
	}
	cfg := section.Config
	months := clampInt(widgetConfigInt(cfg, "months", jizzInsightsDefaultMonths), jizzInsightsMinMonths, jizzInsightsMaxMonths)
	tz, loc := statsWidgetTimezone(cfg)
	now := time.Now()
	start := jizzInsightsStart(now, months, loc)

	monthly, err := data.QueryMonthlyJizz(db, userID, tz, start)
	if err != nil {
		return nil, fmt.Errorf("jizz insights widget: %w", err)
	}
	hours, weekdays, err := data.QueryJizzByHourAndWeekday(db, userID, tz, start)
	if err != nil {
		return nil, fmt.Errorf("jizz insights widget: %w", err)
	}
	top, err := data.QueryTopJizzScenes(db, userID, start, now.Add(time.Minute), jizzInsightsTopScenes)
	if err != nil {
		return nil, fmt.Errorf("jizz insights widget: %w", err)
	}
	share, err := data.QueryJizzedSceneShare(db, userID)
	if err != nil {
		return nil, fmt.Errorf("jizz insights widget: %w", err)
	}

	series, total, max := buildJizzMonthSeries(monthly, start, months)
	payload := JizzInsightsPayload{
		Months:          months,
		Start:           start.Format(statsWidgetDayLayout),
		Series:          series,
		Total:           total,
		AvgPerMonth:     float64(total) / float64(months),
		Max:             max,
		FavoriteHour:    -1,
		FavoriteWeekday: -1,
		TopScenes:       top,
		JizzedScenes:    share.JizzedScenes,
		LibraryCount:    share.LibraryCount,
		Timezone:        tz,
	}
	if payload.TopScenes == nil {
		payload.TopScenes = []data.JizzSceneCount{}
	}
	if total > 0 {
		peaks := buildWatchTimesSeries(hours, weekdays)
		payload.FavoriteHour = peaks.PeakHour
		payload.FavoriteWeekday = peaks.PeakWeekday
	}
	if share.LibraryCount > 0 {
		payload.JizzedShare = float64(share.JizzedScenes) * 100 / float64(share.LibraryCount)
	}
	return emptyWidgetData(section, payload), nil
}

func validateJizzInsightsWidget(cfg map[string]interface{}) error {
	if err := validateStatsIntRange(cfg, "months", jizzInsightsMinMonths, jizzInsightsMaxMonths); err != nil {
		return fmt.Errorf("o insights: %w", err)
	}
	return validateStatsTimezone(cfg)
}
