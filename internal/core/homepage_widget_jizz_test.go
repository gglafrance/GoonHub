package core

import (
	"math"
	"testing"
	"time"

	"goonhub/internal/data"
)

func TestJizzWidgetsAreRegistered(t *testing.T) {
	if !IsHomepageWidgetType("jizz_insights") {
		t.Fatal("jizz_insights not registered")
	}
}

func TestJizzWidgetValidation(t *testing.T) {
	tests := []struct {
		name    string
		widget  string
		cfg     map[string]interface{}
		wantErr bool
	}{
		{"calendar metric jizz", "activity_calendar", map[string]interface{}{"metric": "jizz"}, false},
		{"calendar metric both", "activity_calendar", map[string]interface{}{"metric": "both"}, false},
		{"calendar metric unknown", "activity_calendar", map[string]interface{}{"metric": "likes"}, true},
		{"calendar metric not a string", "activity_calendar", map[string]interface{}{"metric": float64(1)}, true},
		{"watch times metric jizz", "watch_times", map[string]interface{}{"metric": "jizz"}, false},
		{"watch times metric both rejected", "watch_times", map[string]interface{}{"metric": "both"}, true},
		{"top period metric jizz", "top_period", map[string]interface{}{"metric": "jizz", "period": "week"}, false},
		{"top period metric both rejected", "top_period", map[string]interface{}{"metric": "both"}, true},
		{"stats o tiles", "stats", map[string]interface{}{"tiles": []interface{}{"jizz_period", "last_jizz", "jizz_per_week"}}, false},
		{"insights empty", "jizz_insights", map[string]interface{}{}, false},
		{"insights months ok", "jizz_insights", map[string]interface{}{"months": float64(12), "tz": "Europe/Paris"}, false},
		{"insights months too small", "jizz_insights", map[string]interface{}{"months": float64(2)}, true},
		{"insights months too big", "jizz_insights", map[string]interface{}{"months": float64(121)}, true},
		{"insights bad timezone", "jizz_insights", map[string]interface{}{"tz": "Mars/Olympus"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := homepageWidgets[tt.widget].Validate(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate(%v) error = %v, wantErr %v", tt.cfg, err, tt.wantErr)
			}
		})
	}
}

func TestStatsMetric(t *testing.T) {
	tests := []struct {
		cfg       map[string]interface{}
		allowBoth bool
		want      string
	}{
		{map[string]interface{}{}, true, "views"},
		{map[string]interface{}{"metric": "jizz"}, false, "jizz"},
		{map[string]interface{}{"metric": "both"}, true, "both"},
		{map[string]interface{}{"metric": "both"}, false, "views"},
		{map[string]interface{}{"metric": "nope"}, true, "views"},
	}
	for _, tt := range tests {
		if got := statsMetric(tt.cfg, tt.allowBoth); got != tt.want {
			t.Fatalf("statsMetric(%v, %v) = %s, want %s", tt.cfg, tt.allowBoth, got, tt.want)
		}
	}
}

func TestBuildCalendarSeries(t *testing.T) {
	all := []data.DailyViewCount{
		{Day: "2025-12-30", Count: 5}, // before the window: streaks only
		{Day: "2025-12-31", Count: 1},
		{Day: "2026-01-01", Count: 2},
		{Day: "2026-01-02", Count: 3},
		{Day: "2026-01-05", Count: 1},
	}
	s := buildCalendarSeries(all, "2026-01-01", "2026-01-05")
	if len(s.Days) != 3 || s.Total != 6 || s.ActiveDays != 3 || s.Max != 3 {
		t.Fatalf("window figures: %+v", s)
	}
	if s.LongestStreak != 4 || s.CurrentStreak != 1 {
		t.Fatalf("streaks: current=%d longest=%d", s.CurrentStreak, s.LongestStreak)
	}
	if empty := buildCalendarSeries(nil, "2026-01-01", "2026-01-05"); empty.Days == nil || empty.Total != 0 {
		t.Fatalf("empty series: %+v", empty)
	}
}

func TestBuildWatchTimesSeriesJizz(t *testing.T) {
	s := buildWatchTimesSeries(
		[]data.BucketCount{{Bucket: 23, Count: 4}, {Bucket: 0, Count: 1}},
		[]data.BucketCount{{Bucket: 5, Count: 5}},
	)
	if s.PeakHour != 23 || s.PeakWeekday != 4 || s.Total != 5 {
		t.Fatalf("jizz series: %+v", s)
	}
}

func TestJizzPerWeek(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	if got := jizzPerWeek(0, nil, now); got != 0 {
		t.Fatalf("no o: %v", got)
	}
	first := now.AddDate(0, 0, -28)
	if got := jizzPerWeek(8, &first, now); math.Abs(got-2) > 1e-9 {
		t.Fatalf("4 weeks: %v", got)
	}
	// Less than a week counts as one week
	recent := now.Add(-time.Hour)
	if got := jizzPerWeek(3, &recent, now); got != 3 {
		t.Fatalf("under a week: %v", got)
	}
}

func TestJizzInsightsMonths(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Skip("tzdata not available")
	}
	// 23:30Z on Jan 31st is already February in Paris
	now := time.Date(2026, 1, 31, 23, 30, 0, 0, time.UTC)
	start := jizzInsightsStart(now, 3, loc)
	if start.Format("2006-01-02") != "2025-12-01" || start.Location() != loc {
		t.Fatalf("start: %v", start)
	}

	series, total, max := buildJizzMonthSeries([]data.DailyViewCount{
		{Day: "2025-12", Count: 2},
		{Day: "2026-02", Count: 5},
	}, start, 3)
	if len(series) != 3 || series[0].Month != "2025-12" || series[1].Month != "2026-01" || series[1].Count != 0 || series[2].Count != 5 {
		t.Fatalf("series: %+v", series)
	}
	if total != 7 || max != 5 {
		t.Fatalf("total=%d max=%d", total, max)
	}
}

func TestJizzInsightsWithoutDatabase(t *testing.T) {
	section := data.HomepageSection{ID: "s", Type: "jizz_insights", Config: map[string]interface{}{}}
	if _, err := homepageWidgets["jizz_insights"].Fetch(&HomepageService{}, 1, section); err == nil {
		t.Fatal("expected an error without database")
	}
}
