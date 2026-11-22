package data

import (
	"strings"
	"testing"
)

// A template saved before the grid layout (items and direction only) reads back unchanged and
// is stored again without a columns key.
func TestSceneCardConfig_OldShapeRoundTrip(t *testing.T) {
	stored := []byte(`{"badges":{"top_left":{"items":["rating"],"direction":"vertical"},` +
		`"top_right":{"items":["watched"],"direction":"vertical"},"bottom_left":{"items":[],"direction":"vertical"},` +
		`"bottom_right":{"items":["duration"],"direction":"horizontal"}},"content_rows":[]}`)
	var config SceneCardConfig
	if err := config.Scan(stored); err != nil {
		t.Fatalf("scan: %v", err)
	}
	zones := []BadgeZone{config.Badges.TopLeft, config.Badges.TopRight, config.Badges.BottomLeft, config.Badges.BottomRight}
	for i, zone := range zones {
		if zone.Columns != 0 {
			t.Fatalf("zone %d: columns = %d, want 0", i, zone.Columns)
		}
	}
	value, err := config.Value()
	if err != nil {
		t.Fatalf("value: %v", err)
	}
	if string(value.([]byte)) != string(stored) {
		t.Fatalf("old shape changed:\n got %s\nwant %s", value, stored)
	}
}

func TestSceneCardConfig_ColumnsRoundTrip(t *testing.T) {
	config := DefaultSceneCardConfig()
	config.Badges.TopRight = BadgeZone{Items: []string{"watched", "views", "resolution"}, Direction: "horizontal", Columns: 2}
	value, err := config.Value()
	if err != nil {
		t.Fatalf("value: %v", err)
	}
	raw := value.([]byte)
	if !strings.Contains(string(raw), `"top_right":{"items":["watched","views","resolution"],"direction":"horizontal","columns":2}`) {
		t.Fatalf("columns not stored: %s", raw)
	}
	if strings.Count(string(raw), `"columns"`) != 1 {
		t.Fatalf("zones without columns should not store the key: %s", raw)
	}
	var back SceneCardConfig
	if err := back.Scan(raw); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if back.Badges.TopRight.Columns != 2 || len(back.Badges.TopRight.Items) != 3 || back.Badges.TopLeft.Columns != 0 {
		t.Fatalf("round trip lost the grid: %+v", back.Badges)
	}
}
