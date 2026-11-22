package core

import (
	"strings"
	"testing"

	"goonhub/internal/data"
)

// cardConfigWithTopLeft is the default card template with its top-left zone replaced.
func cardConfigWithTopLeft(zone data.BadgeZone) data.SceneCardConfig {
	config := data.DefaultSceneCardConfig()
	config.Badges.TopLeft = zone
	return config
}

func twelveBadges() []string {
	return []string{"liked", "rating", "duration", "resolution", "views", "jizz_count",
		"watched", "file_size", "added_at", "frame_rate", "tags", "actors"}
}

func TestValidateSceneCardConfig_BadgeZoneGrid(t *testing.T) {
	tests := []struct {
		name    string
		zone    data.BadgeZone
		wantErr string
	}{
		{"old shape, no columns", data.BadgeZone{Items: []string{"rating", "liked"}, Direction: "vertical"}, ""},
		{"old shape, horizontal", data.BadgeZone{Items: []string{"duration"}, Direction: "horizontal"}, ""},
		{"empty zone with columns", data.BadgeZone{Items: []string{}, Direction: "vertical", Columns: 3}, ""},
		{"one column", data.BadgeZone{Items: []string{"rating"}, Direction: "vertical", Columns: 1}, ""},
		{"four columns", data.BadgeZone{Items: twelveBadges(), Direction: "horizontal", Columns: 4}, ""},
		{"twelve items, no columns", data.BadgeZone{Items: twelveBadges(), Direction: "vertical"}, ""},
		{"same field twice", data.BadgeZone{Items: []string{"rating", "rating"}, Direction: "vertical", Columns: 2}, ""},
		{"five columns", data.BadgeZone{Items: []string{"rating"}, Direction: "vertical", Columns: 5}, "columns must be between 1 and 4"},
		{"negative columns", data.BadgeZone{Items: []string{"rating"}, Direction: "vertical", Columns: -1}, "columns must be between 1 and 4"},
		{"thirteen items", data.BadgeZone{Items: append(twelveBadges(), "rating"), Direction: "horizontal", Columns: 4}, "maximum of 12 items"},
		{"bad field", data.BadgeZone{Items: []string{"bogus"}, Direction: "vertical", Columns: 2}, "invalid badge field"},
		{"bad direction", data.BadgeZone{Items: []string{}, Direction: "diagonal", Columns: 2}, "invalid direction"},
	}
	svc := &SettingsService{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := cardConfigWithTopLeft(tt.zone)
			err := svc.validateSceneCardConfig(&config)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
			if !strings.Contains(err.Error(), "top_left") {
				t.Fatalf("error should name the zone: %v", err)
			}
		})
	}
}

func TestValidateSceneCardConfig_SameFieldInSeveralZones(t *testing.T) {
	config := data.DefaultSceneCardConfig()
	config.Badges.TopLeft = data.BadgeZone{Items: []string{"rating", "duration"}, Direction: "vertical", Columns: 2}
	config.Badges.BottomRight = data.BadgeZone{Items: []string{"duration", "rating"}, Direction: "horizontal", Columns: 2}
	if err := (&SettingsService{}).validateSceneCardConfig(&config); err != nil {
		t.Fatalf("a field may appear in several zones: %v", err)
	}
}

func TestWithSceneCardConfig_StoresColumns(t *testing.T) {
	svc := &SettingsService{}
	config := cardConfigWithTopLeft(data.BadgeZone{Items: []string{"rating", "liked", "views"}, Direction: "horizontal", Columns: 2})
	settings := &data.UserSettings{}
	if err := svc.WithSceneCardConfig(config)(settings); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if settings.SceneCardConfig.Badges.TopLeft.Columns != 2 || settings.SceneCardConfig.Badges.TopRight.Columns != 0 {
		t.Fatalf("columns not stored: %+v", settings.SceneCardConfig.Badges)
	}
	if err := svc.WithSceneCardConfig(cardConfigWithTopLeft(data.BadgeZone{Items: []string{}, Direction: "vertical", Columns: 9}))(settings); err == nil {
		t.Fatal("expected an error for 9 columns")
	}
}
