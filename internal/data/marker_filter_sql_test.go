package data

import (
	"strings"
	"testing"
)

func TestMarkerFilterSQL(t *testing.T) {
	tests := []struct {
		name       string
		opts       MarkerListOptions
		withLabels bool
		wantParts  []string
		wantArgs   int
	}{
		{"no filter", MarkerListOptions{}, true, nil, 0},
		{"ranges only", MarkerListOptions{RangesOnly: true}, true, []string{"m.end_timestamp IS NOT NULL"}, 0},
		{"labels", MarkerListOptions{Labels: []string{"a", "b"}}, true, []string{"m.label_id IN (SELECT id FROM marker_labels WHERE lower(name) IN ?)"}, 1},
		{"labels ignored", MarkerListOptions{Labels: []string{"a"}}, false, nil, 0},
		{"tags need all", MarkerListOptions{Tags: []string{"x", "y", "x"}}, true, []string{"scene_tags", "HAVING COUNT(DISTINCT t.name) = ?"}, 2},
		{"actors", MarkerListOptions{Actors: []string{"z"}}, true, []string{"scene_actors"}, 2},
		{"studio", MarkerListOptions{Studio: "S"}, true, []string{"studios"}, 2},
		{"marker tags need all", MarkerListOptions{MarkerTags: []string{"Cum", "Suck"}}, true, []string{"marker_tags", "m.id IN"}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args := markerFilterSQL(tt.opts, "m", tt.withLabels)
			if len(tt.wantParts) == 0 && sql != "" {
				t.Fatalf("expected no SQL, got %q", sql)
			}
			for _, part := range tt.wantParts {
				if !strings.Contains(sql, part) {
					t.Fatalf("expected %q in %q", part, sql)
				}
			}
			if len(args) != tt.wantArgs {
				t.Fatalf("expected %d args, got %d", tt.wantArgs, len(args))
			}
			if tt.name == "tags need all" && args[1] != 2 {
				t.Fatalf("expected distinct tag count 2, got %v", args[1])
			}
		})
	}
}
