package core

import (
	"testing"
	"time"

	"goonhub/internal/data"
)

func TestStatsWidgetsAreRegistered(t *testing.T) {
	for _, widget := range []string{"stats", "activity_calendar", "watch_times", "records"} {
		if !IsHomepageWidgetType(widget) {
			t.Fatalf("widget %q not registered", widget)
		}
	}
}

func TestStatsWidgetValidation(t *testing.T) {
	tests := []struct {
		name    string
		widget  string
		cfg     map[string]interface{}
		wantErr bool
	}{
		{"stats empty", "stats", map[string]interface{}{}, false},
		{"stats valid", "stats", map[string]interface{}{"period": "30d", "tiles": []interface{}{"views", "size"}, "tz": "Europe/Paris"}, false},
		{"stats bad period", "stats", map[string]interface{}{"period": "year"}, true},
		{"stats bad tile", "stats", map[string]interface{}{"tiles": []interface{}{"views", "nope"}}, true},
		{"stats tiles not a list", "stats", map[string]interface{}{"tiles": "views"}, true},
		{"bad timezone", "stats", map[string]interface{}{"tz": "Mars/Olympus"}, true},
		{"local timezone rejected", "watch_times", map[string]interface{}{"tz": "Local"}, true},
		{"empty timezone allowed", "records", map[string]interface{}{"tz": ""}, false},
		{"calendar weeks ok", "activity_calendar", map[string]interface{}{"weeks": float64(26)}, false},
		{"calendar weeks too small", "activity_calendar", map[string]interface{}{"weeks": float64(2)}, true},
		{"calendar weeks not a number", "activity_calendar", map[string]interface{}{"weeks": "x"}, true},
		{"watch times days ok", "watch_times", map[string]interface{}{"days": float64(90)}, false},
		{"watch times days too big", "watch_times", map[string]interface{}{"days": float64(99999)}, true},
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

func TestStatsWidgetTimezoneFallback(t *testing.T) {
	t.Setenv("TZ", "")
	name, loc := statsWidgetTimezone(map[string]interface{}{"tz": "Invalid/Zone"})
	if name != "UTC" || loc != time.UTC {
		t.Fatalf("expected UTC fallback, got %s", name)
	}
	t.Setenv("TZ", "Europe/Paris")
	if name, _ := statsWidgetTimezone(map[string]interface{}{}); name != "Europe/Paris" {
		t.Fatalf("expected server TZ, got %s", name)
	}
	if name, _ := statsWidgetTimezone(map[string]interface{}{"tz": "America/New_York"}); name != "America/New_York" {
		t.Fatalf("expected config TZ, got %s", name)
	}
}

func TestStatsPeriodBounds(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 3, 18, 15, 0, 0, 0, loc) // a Wednesday

	prev, start := statsPeriodBounds("month", now, loc)
	if !start.Equal(time.Date(2026, 3, 1, 0, 0, 0, 0, loc)) || !prev.Equal(time.Date(2026, 2, 1, 0, 0, 0, 0, loc)) {
		t.Fatalf("month bounds: %v %v", prev, start)
	}
	prev, start = statsPeriodBounds("week", now, loc)
	if !start.Equal(time.Date(2026, 3, 16, 0, 0, 0, 0, loc)) || !prev.Equal(time.Date(2026, 3, 9, 0, 0, 0, 0, loc)) {
		t.Fatalf("week bounds: %v %v", prev, start)
	}
	prev, start = statsPeriodBounds("7d", now, loc)
	if now.Sub(start) != 7*24*time.Hour || start.Sub(prev) != 7*24*time.Hour {
		t.Fatalf("7d bounds: %v %v", prev, start)
	}
}

func TestStatsWidgetSelectedTiles(t *testing.T) {
	if got := statsWidgetSelectedTiles(map[string]interface{}{}); len(got) != len(statsWidgetTiles) {
		t.Fatalf("default should be all tiles, got %v", got)
	}
	got := statsWidgetSelectedTiles(map[string]interface{}{"tiles": []interface{}{"views", "nope", "views", "size"}})
	if len(got) != 2 || got[0] != "views" || got[1] != "size" {
		t.Fatalf("unexpected tiles: %v", got)
	}
}

func TestViewStreaks(t *testing.T) {
	days := []string{"2026-01-01", "2026-01-02", "2026-01-03", "2026-01-10", "2026-01-14", "2026-01-15"}
	current, longest, start, end := viewStreaks(days, "2026-01-15")
	if current != 2 || longest != 3 || start != "2026-01-01" || end != "2026-01-03" {
		t.Fatalf("got current=%d longest=%d %s..%s", current, longest, start, end)
	}
	// Nothing watched yet today: the streak ending yesterday still counts
	if current, _, _, _ := viewStreaks(days, "2026-01-16"); current != 2 {
		t.Fatalf("streak ending yesterday: got %d", current)
	}
	// A gap of two days breaks it
	if current, _, _, _ := viewStreaks(days, "2026-01-17"); current != 0 {
		t.Fatalf("broken streak: got %d", current)
	}
	if current, longest, _, _ := viewStreaks(nil, "2026-01-17"); current != 0 || longest != 0 {
		t.Fatal("empty history must give no streak")
	}
	// Month boundary
	if _, longest, _, _ := viewStreaks([]string{"2026-02-28", "2026-03-01"}, "2026-03-01"); longest != 2 {
		t.Fatalf("month boundary streak: %d", longest)
	}
}

func TestActivityCalendarStartIsMonday(t *testing.T) {
	today := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC) // Thursday
	start := activityCalendarStart(today, 52)
	if start.Weekday() != time.Monday {
		t.Fatalf("calendar must start on a Monday, got %v", start.Weekday())
	}
	if days := int(today.Sub(start).Hours() / 24); days != 51*7+3 {
		t.Fatalf("unexpected span: %d days", days)
	}
}

func TestBuildWatchTimesPayload(t *testing.T) {
	p := buildWatchTimesPayload(
		[]data.BucketCount{{Bucket: 9, Count: 2}, {Bucket: 18, Count: 7}, {Bucket: 30, Count: 99}},
		[]data.BucketCount{{Bucket: 1, Count: 3}, {Bucket: 7, Count: 6}},
	)
	if p.Total != 9 || p.Hours[18] != 7 || p.PeakHour != 18 {
		t.Fatalf("hours: %+v", p)
	}
	if p.Weekdays[0] != 3 || p.Weekdays[6] != 6 || p.PeakWeekday != 6 {
		t.Fatalf("weekdays: %+v", p.Weekdays)
	}
}

func TestBuildDayRecords(t *testing.T) {
	streak, busiest := buildDayRecords([]data.DailyViewCount{
		{Day: "2026-01-01", Count: 1},
		{Day: "2026-01-02", Count: 9},
		{Day: "2026-01-05", Count: 2},
	}, "2026-01-20")
	if streak == nil || streak.Days != 2 || streak.Start != "2026-01-01" || streak.End != "2026-01-02" {
		t.Fatalf("streak: %+v", streak)
	}
	if busiest == nil || busiest.Day != "2026-01-02" || busiest.Count != 9 {
		t.Fatalf("busiest: %+v", busiest)
	}
	if streak, busiest := buildDayRecords(nil, "2026-01-20"); streak != nil || busiest != nil {
		t.Fatal("empty history must give no records")
	}
}

func TestStatsWidgetsWithoutDatabase(t *testing.T) {
	svc := &HomepageService{}
	section := data.HomepageSection{ID: "s", Type: "stats", Config: map[string]interface{}{}}
	for _, widget := range []string{"stats", "activity_calendar", "watch_times", "records"} {
		section.Type = widget
		if _, err := homepageWidgets[widget].Fetch(svc, 1, section); err == nil {
			t.Fatalf("%s: expected an error without database", widget)
		}
	}
}
