package core

import (
	"testing"

	"goonhub/internal/data"
)

func TestGetAllMarkers_AttachesTags(t *testing.T) {
	svc, markerRepo, _ := newTestMarkerService(t)
	markers := []data.MarkerWithScene{
		{UserSceneMarker: data.UserSceneMarker{ID: 1, Label: "Cum"}},
		{UserSceneMarker: data.UserSceneMarker{ID: 2, Label: "Kiss"}},
	}
	markerRepo.EXPECT().GetAllMarkersForUser(uint(1), 0, 20, data.MarkerListOptions{SortBy: "label_asc"}).Return(markers, int64(2), nil)
	markerRepo.EXPECT().GetMarkerTagsMultiple([]uint{1, 2}).Return(map[uint][]data.MarkerTagInfo{
		1: {{ID: 9, Name: "Cum"}, {ID: 10, Name: "Suck"}},
	}, nil)

	got, _, err := svc.GetAllMarkers(1, 1, 20, data.MarkerListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got[0].Tags) != 2 || len(got[1].Tags) != 0 {
		t.Fatalf("tags not attached: %+v", got)
	}
}
