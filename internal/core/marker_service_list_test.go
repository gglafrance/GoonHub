package core

import (
	"testing"

	"goonhub/internal/data"
)

func TestNormalizeMarkerSort(t *testing.T) {
	valid := map[string]bool{"label_asc": true, "recent": true}
	tests := []struct {
		name     string
		in       data.MarkerListOptions
		wantSort string
		wantSeed bool
	}{
		{"valid sort kept", data.MarkerListOptions{SortBy: "recent"}, "recent", false},
		{"invalid sort falls back", data.MarkerListOptions{SortBy: "nope"}, "label_asc", false},
		{"random gets a seed", data.MarkerListOptions{SortBy: "random"}, "random", true},
		{"random keeps its seed", data.MarkerListOptions{SortBy: "random", Seed: 42}, "random", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeMarkerSort(tt.in, valid, "label_asc")
			if got.SortBy != tt.wantSort {
				t.Fatalf("sort = %q, want %q", got.SortBy, tt.wantSort)
			}
			if tt.wantSeed && got.Seed <= 0 {
				t.Fatalf("expected a positive seed, got %d", got.Seed)
			}
			if !tt.wantSeed && got.Seed != 0 {
				t.Fatalf("expected no seed, got %d", got.Seed)
			}
			if tt.in.Seed != 0 && got.Seed != tt.in.Seed {
				t.Fatalf("seed changed from %d to %d", tt.in.Seed, got.Seed)
			}
		})
	}
}

func TestGetAllMarkers_PassesFilters(t *testing.T) {
	svc, markerRepo, _ := newTestMarkerService(t)
	markerRepo.EXPECT().
		GetAllMarkersForUser(uint(1), 20, 10, data.MarkerListOptions{SortBy: "random", RangesOnly: true, Seed: 7}).
		Return([]data.MarkerWithScene{}, int64(0), nil)

	if _, _, err := svc.GetAllMarkers(1, 3, 10, data.MarkerListOptions{SortBy: "random", RangesOnly: true, Seed: 7}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetLabelGroups_RangesOnlyThumbnails(t *testing.T) {
	svc, markerRepo, _ := newTestMarkerService(t)
	markerRepo.EXPECT().
		GetLabelGroupsForUser(uint(1), 0, 20, data.MarkerListOptions{SortBy: "count_desc", RangesOnly: true}).
		Return([]data.MarkerLabelGroup{{Label: "Kiss", Count: 2}}, int64(1), nil)
	markerRepo.EXPECT().
		GetRandomThumbnailsForLabels(uint(1), []string{"Kiss"}, 10, data.MarkerListOptions{SortBy: "count_desc", RangesOnly: true}).
		Return(map[string][]uint{"Kiss": {4}}, nil)

	groups, _, err := svc.GetLabelGroups(1, 1, 20, data.MarkerListOptions{SortBy: "bogus", RangesOnly: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(groups) != 1 || len(groups[0].ThumbnailMarkerIDs) != 1 {
		t.Fatalf("unexpected groups: %+v", groups)
	}
}

func TestGetMarkersByLabel_PassesFilters(t *testing.T) {
	svc, markerRepo, _ := newTestMarkerService(t)
	markerRepo.EXPECT().
		GetMarkersByLabelForUser(uint(1), "Kiss", 0, 20, data.MarkerListOptions{RangesOnly: true, Tags: []string{"Outdoor"}}).
		Return([]data.MarkerWithScene{}, int64(0), nil)

	if _, _, err := svc.GetMarkersByLabel(1, "Kiss", 1, 20, data.MarkerListOptions{RangesOnly: true, Tags: []string{"Outdoor"}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetMarkersForScenes_GroupsByScene(t *testing.T) {
	svc, markerRepo, _ := newTestMarkerService(t)
	markerRepo.EXPECT().GetByUserAndScenes(uint(1), []uint{2, 3}).Return([]data.UserSceneMarker{
		{ID: 1, SceneID: 2, Timestamp: 5},
		{ID: 2, SceneID: 2, Timestamp: 9},
		{ID: 3, SceneID: 3, Timestamp: 1},
	}, nil)

	got, err := svc.GetMarkersForScenes(1, []uint{2, 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got[2]) != 2 || len(got[3]) != 1 {
		t.Fatalf("unexpected grouping: %+v", got)
	}
}

func TestGetMarkersForScenes_AnonymousSkipsQuery(t *testing.T) {
	svc, _, _ := newTestMarkerService(t)
	got, err := svc.GetMarkersForScenes(0, []uint{2})
	if err != nil || len(got) != 0 {
		t.Fatalf("expected empty result without query, got %v, %v", got, err)
	}
}
