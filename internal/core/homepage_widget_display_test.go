package core

import (
	"testing"
	"time"

	"goonhub/internal/data"
)

func TestDisplayWidgetsAreRegistered(t *testing.T) {
	for _, widget := range []string{"shorts", "preview_wall", "hero", "top_period", "quick_rate"} {
		if !IsHomepageWidgetType(widget) {
			t.Fatalf("widget %q is not registered", widget)
		}
	}
}

func TestDisplayWidgetValidation(t *testing.T) {
	tests := []struct {
		name     string
		validate func(map[string]interface{}) error
		cfg      map[string]interface{}
		wantErr  bool
	}{
		{"shorts empty", validateShortsWidget, map[string]interface{}{}, false},
		{"shorts ok", validateShortsWidget, map[string]interface{}{"max_duration": float64(90), "unwatched_only": true}, false},
		{"shorts too short", validateShortsWidget, map[string]interface{}{"max_duration": float64(1)}, true},
		{"shorts bad bool", validateShortsWidget, map[string]interface{}{"unwatched_only": "yes"}, true},
		{"wall ok", validatePreviewWallWidget, map[string]interface{}{"source": "liked", "columns": float64(5)}, false},
		{"wall bad source", validatePreviewWallWidget, map[string]interface{}{"source": "top_jizz"}, true},
		{"wall bad columns", validatePreviewWallWidget, map[string]interface{}{"columns": float64(9)}, true},
		{"hero ok", validateHeroWidget, map[string]interface{}{"source": "top_jizz", "rotate_seconds": float64(10)}, false},
		{"hero manual rotation", validateHeroWidget, map[string]interface{}{"rotate_seconds": float64(0)}, false},
		{"hero rotation too fast", validateHeroWidget, map[string]interface{}{"rotate_seconds": float64(2)}, true},
		{"hero bad source", validateHeroWidget, map[string]interface{}{"source": "latest"}, true},
		{"top period ok", validateTopPeriodWidget, map[string]interface{}{"period": "week"}, false},
		{"top period bad", validateTopPeriodWidget, map[string]interface{}{"period": "day"}, true},
		{"quick rate ok", validateQuickRateWidget, map[string]interface{}{"days": float64(7)}, false},
		{"quick rate bad", validateQuickRateWidget, map[string]interface{}{"days": float64(0)}, true},
		{"quick rate not a number", validateQuickRateWidget, map[string]interface{}{"days": "7"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.validate(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDisplayPeriodWindow(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		period string
		days   int
	}{{"week", 7}, {"month", 30}, {"year", 365}, {"unknown", 30}}
	for _, tt := range tests {
		from, to, prevFrom := displayPeriodWindow(tt.period, now)
		if !to.Equal(now) || !from.Equal(now.AddDate(0, 0, -tt.days)) || !prevFrom.Equal(from.AddDate(0, 0, -tt.days)) {
			t.Fatalf("%s: unexpected window %v %v %v", tt.period, prevFrom, from, to)
		}
	}
}

func TestDisplayOrderScenes(t *testing.T) {
	scenes := []data.Scene{{ID: 1}, {ID: 2}, {ID: 3}}
	got := displayOrderScenes(scenes, []uint{3, 9, 1})
	if len(got) != 2 || got[0].ID != 3 || got[1].ID != 1 {
		t.Fatalf("unexpected order: %+v", got)
	}
}

func TestDisplayWidgetsRequireDatabase(t *testing.T) {
	svc := &HomepageService{}
	section := data.HomepageSection{ID: "s", Type: "shorts", Limit: 5, Config: map[string]interface{}{}}
	if _, err := fetchShortsWidget(svc, 1, section); err == nil {
		t.Fatal("expected an error without database")
	}
}
