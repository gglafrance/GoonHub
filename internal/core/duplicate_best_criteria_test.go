package core

import (
	"testing"
	"time"

	"goonhub/internal/data"
)

func TestParseBestCriteria(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    int
		wantErr bool
	}{
		{"empty uses default", "", len(DefaultBestCriteria), false},
		{"custom order", "duration, resolution", 2, false},
		{"unknown", "resolution,shiny", 0, true},
		{"repeated", "duration,duration", 0, true},
		{"contradiction size", "size_small,size_large", 0, true},
		{"contradiction age", "oldest,newest", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseBestCriteria(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil || len(got) != tt.want {
				t.Fatalf("got %v, %v; want %d criteria", got, err, tt.want)
			}
		})
	}
}

func TestBetterSceneBy_CustomOrder(t *testing.T) {
	old := time.Now().Add(-time.Hour)
	a := &data.Scene{ID: 1, Width: 1920, Height: 1080, Duration: 100, VideoCodec: "h264", CreatedAt: old}
	b := &data.Scene{ID: 2, Width: 1280, Height: 720, Duration: 200, VideoCodec: "av1", CreatedAt: time.Now()}

	if !betterSceneBy(a, b, []string{BestCriterionResolution}) {
		t.Fatal("resolution first: a should win")
	}
	if !betterSceneBy(b, a, []string{BestCriterionDuration, BestCriterionResolution}) {
		t.Fatal("duration first: b should win")
	}
	if !betterSceneBy(b, a, []string{BestCriterionCodec}) {
		t.Fatal("codec: av1 should win")
	}
	if !betterSceneBy(a, b, []string{BestCriterionOldest}) {
		t.Fatal("oldest: a should win")
	}
	if !betterSceneBy(b, a, []string{BestCriterionNewest}) {
		t.Fatal("newest: b should win")
	}
}

func TestBestCriteriaFromConfig_InvalidFallsBack(t *testing.T) {
	got := bestCriteriaFromConfig(&data.DuplicationConfigRecord{BestSelectionCriteria: "nope"})
	if len(got) != len(DefaultBestCriteria) || got[0] != BestCriterionResolution {
		t.Fatalf("expected default criteria, got %v", got)
	}
}
