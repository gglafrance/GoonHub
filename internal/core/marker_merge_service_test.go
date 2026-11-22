package core

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"goonhub/internal/data"

	"go.uber.org/mock/gomock"
)

// fakeMarkerHelper records what resolution asks of the marker service.
type fakeMarkerHelper struct {
	dedupScenes [][]uint
	dedupDryRun []bool
	dedupResult *MarkerDedupResult
	dedupErr    error
	queued      []uint
}

func (f *fakeMarkerHelper) DeduplicateMarkers(sceneIDs []uint, dryRun bool) (*MarkerDedupResult, error) {
	f.dedupScenes = append(f.dedupScenes, sceneIDs)
	f.dedupDryRun = append(f.dedupDryRun, dryRun)
	return f.dedupResult, f.dedupErr
}

func (f *fakeMarkerHelper) QueueMarkerThumbnails(markerIDs []uint) {
	f.queued = append(f.queued, markerIDs...)
}

// clipGroup: scene 20 (90 s) is a clip found 70 s into scene 10 (600 s).
func clipGroup() (*data.DuplicateGroup, map[uint]data.Scene) {
	ref := uint(10)
	group := &data.DuplicateGroup{ID: 1, Members: []data.DuplicateGroupMember{
		{GroupID: 1, SceneID: 10},
		{GroupID: 1, SceneID: 20, RefSceneID: &ref, MatchStart: ptr[float64](0), MatchEnd: ptr[float64](90),
			RefStart: ptr[float64](70), RefEnd: ptr[float64](160), Relation: data.DuplicateRelationContained},
	}}
	return group, map[uint]data.Scene{10: {ID: 10, Duration: 600}, 20: {ID: 20, Duration: 90}}
}

func TestMergeMarkers_ShiftsMovesAndLeavesBehind(t *testing.T) {
	svc, _, _, _, _, markerRepo, _ := newTestDuplicateService(t)
	helper := &fakeMarkerHelper{dedupResult: &MarkerDedupResult{Removed: 1}}
	svc.SetMarkerService(helper)
	group, scenes := clipGroup()

	markerRepo.EXPECT().GetAllByScene(uint(20)).Return([]data.UserSceneMarker{
		{ID: 1, UserID: 1, SceneID: 20, Timestamp: 10, Label: "Kiss"},
		{ID: 2, UserID: 1, SceneID: 20, Timestamp: 95, Label: "Credits"},
		{ID: 3, UserID: 2, SceneID: 20, Timestamp: 0, EndTimestamp: ptr(5), Label: "Intro"},
	}, nil)
	markerRepo.EXPECT().MoveMarkers([]data.MarkerMove{
		{ID: 1, SceneID: 10, Timestamp: 80},
		{ID: 3, SceneID: 10, Timestamp: 70, EndTimestamp: ptr(75)},
	}).Return(nil)

	got, err := svc.mergeMarkers(group, 10, []uint{20}, scenes)
	if err != nil {
		t.Fatal(err)
	}
	want := MarkerMergeSummary{Moved: 2, Shifted: 2, Deduplicated: 1, LeftBehind: 1}
	if got != want {
		t.Fatalf("summary = %+v, want %+v", got, want)
	}
	if !reflect.DeepEqual(helper.dedupScenes, [][]uint{{10}}) || helper.dedupDryRun[0] {
		t.Fatalf("expected one applied dedup of the kept scene, got %v %v", helper.dedupScenes, helper.dedupDryRun)
	}
	if !slices.Equal(helper.queued, []uint{1, 3}) {
		t.Fatalf("thumbnails queued for %v, want [1 3]", helper.queued)
	}
}

func TestMergeMarkers_IdenticalWithoutSegmentKeepsTimes(t *testing.T) {
	svc, _, _, _, _, markerRepo, _ := newTestDuplicateService(t)
	helper := &fakeMarkerHelper{dedupResult: &MarkerDedupResult{}}
	svc.SetMarkerService(helper)
	group := &data.DuplicateGroup{ID: 1, Members: []data.DuplicateGroupMember{
		{SceneID: 10, Relation: data.DuplicateRelationIdentical},
		{SceneID: 20, Relation: data.DuplicateRelationIdentical},
	}}
	scenes := map[uint]data.Scene{10: {ID: 10, Duration: 300}, 20: {ID: 20, Duration: 301}}

	markerRepo.EXPECT().GetAllByScene(uint(20)).Return([]data.UserSceneMarker{
		{ID: 1, Timestamp: 10}, {ID: 2, Timestamp: 301},
	}, nil)
	markerRepo.EXPECT().MoveMarkers([]data.MarkerMove{{ID: 1, SceneID: 10, Timestamp: 10}}).Return(nil)

	got, err := svc.mergeMarkers(group, 10, []uint{20}, scenes)
	if err != nil {
		t.Fatal(err)
	}
	if got != (MarkerMergeSummary{Moved: 1, LeftBehind: 1}) {
		t.Fatalf("unexpected summary %+v", got)
	}
	if len(helper.queued) != 0 {
		t.Fatalf("no thumbnail to remake when times did not change, got %v", helper.queued)
	}
}

func TestMergeMarkers_NothingToMove(t *testing.T) {
	svc, _, _, _, _, markerRepo, _ := newTestDuplicateService(t)
	helper := &fakeMarkerHelper{}
	svc.SetMarkerService(helper)
	group, scenes := clipGroup()
	markerRepo.EXPECT().GetAllByScene(uint(20)).Return(nil, nil)

	got, err := svc.mergeMarkers(group, 10, []uint{20}, scenes)
	if err != nil || got != (MarkerMergeSummary{}) {
		t.Fatalf("got %+v, %v", got, err)
	}
	if len(helper.dedupScenes) != 0 {
		t.Fatal("no dedup expected when nothing moved")
	}
}

func TestMergeMarkers_DedupFailureDoesNotFailResolution(t *testing.T) {
	svc, _, _, _, _, markerRepo, _ := newTestDuplicateService(t)
	svc.SetMarkerService(&fakeMarkerHelper{dedupErr: errors.New("db down")})
	group, scenes := clipGroup()
	markerRepo.EXPECT().GetAllByScene(uint(20)).Return([]data.UserSceneMarker{{ID: 1, Timestamp: 10}}, nil)
	markerRepo.EXPECT().MoveMarkers(gomock.Any()).Return(nil)

	got, err := svc.mergeMarkers(group, 10, []uint{20}, scenes)
	if err != nil {
		t.Fatal(err)
	}
	if got.Moved != 1 || got.Deduplicated != 0 {
		t.Fatalf("unexpected summary %+v", got)
	}
}

func TestMergeMarkers_MoveFailureFails(t *testing.T) {
	svc, _, _, _, _, markerRepo, _ := newTestDuplicateService(t)
	group, scenes := clipGroup()
	cause := errors.New("db down")
	markerRepo.EXPECT().GetAllByScene(uint(20)).Return([]data.UserSceneMarker{{ID: 1, Timestamp: 10}}, nil)
	markerRepo.EXPECT().MoveMarkers(gomock.Any()).Return(cause)

	if _, err := svc.mergeMarkers(group, 10, []uint{20}, scenes); !errors.Is(err, cause) {
		t.Fatalf("expected the move failure, got %v", err)
	}
}

func TestDeduplicateMarkers_DryRunChangesNothing(t *testing.T) {
	svc, markerRepo, _ := newTestMarkerService(t)
	markerRepo.EXPECT().GetAllByScene(uint(7)).Return([]data.UserSceneMarker{
		{ID: 1, UserID: 1, SceneID: 7, Timestamp: 10, Label: "Kiss"},
		{ID: 2, UserID: 1, SceneID: 7, Timestamp: 11, Label: "kiss"},
		{ID: 3, UserID: 1, SceneID: 7, Timestamp: 200, Label: "Kiss"},
	}, nil)
	markerRepo.EXPECT().GetMarkerTagsMultiple([]uint{1, 2, 3}).Return(map[uint][]data.MarkerTagInfo{
		2: {{ID: 9}},
	}, nil)

	res, err := svc.DeduplicateMarkers([]uint{7}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !res.DryRun || res.ScenesScanned != 1 || res.Scenes != 1 || res.Groups != 1 || res.Removed != 1 || len(res.Merges) != 1 {
		t.Fatalf("unexpected result %+v", res)
	}
	m := res.Merges[0]
	if m.Kept.ID != 2 || len(m.Removed) != 1 || m.Removed[0].ID != 1 || m.Label != "kiss" || len(m.AddedTagIDs) != 0 {
		t.Fatalf("unexpected merge %+v", m)
	}
}

func TestDeduplicateMarkers_AppliesAndRemovesThumbnails(t *testing.T) {
	svc, markerRepo, _ := newTestMarkerService(t)
	svc.markerThumbnailDir = t.TempDir()
	thumb := filepath.Join(svc.markerThumbnailDir, "marker_1.webp")
	if err := os.WriteFile(thumb, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	markerRepo.EXPECT().ListSceneIDsWithNearbyMarkers(markerDuplicateToleranceSeconds).Return([]uint{7, 8}, nil)
	markerRepo.EXPECT().GetAllByScene(uint(7)).Return([]data.UserSceneMarker{
		{ID: 1, UserID: 1, SceneID: 7, Timestamp: 10, Label: "Kiss", ThumbnailPath: "marker_1.webp"},
		{ID: 2, UserID: 1, SceneID: 7, Timestamp: 11, EndTimestamp: ptr(40)},
	}, nil)
	markerRepo.EXPECT().GetMarkerTagsMultiple([]uint{1, 2}).Return(map[uint][]data.MarkerTagInfo{
		1: {{ID: 9, IsFromLabel: true}},
	}, nil)
	label := "Kiss"
	markerRepo.EXPECT().ApplyMarkerDedup([]data.MarkerDedupMerge{{
		KeepID: 2, Label: &label, AddTags: []data.MarkerDedupTag{{TagID: 9, IsFromLabel: true}}, RemoveIDs: []uint{1},
	}}).Return(nil)
	// scene 8 has a single marker left: nothing to fold
	markerRepo.EXPECT().GetAllByScene(uint(8)).Return([]data.UserSceneMarker{{ID: 5, SceneID: 8}}, nil)

	res, err := svc.DeduplicateMarkers(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.DryRun || res.ScenesScanned != 2 || res.Scenes != 1 || res.Removed != 1 {
		t.Fatalf("unexpected result %+v", res)
	}
	if _, err := os.Stat(thumb); !os.IsNotExist(err) {
		t.Fatalf("thumbnail of the removed marker should be gone, stat err = %v", err)
	}
}

func TestDeduplicateMarkers_ApplyFailure(t *testing.T) {
	svc, markerRepo, _ := newTestMarkerService(t)
	cause := errors.New("db down")
	markerRepo.EXPECT().GetAllByScene(uint(7)).Return([]data.UserSceneMarker{
		{ID: 1, UserID: 1, Timestamp: 10}, {ID: 2, UserID: 1, Timestamp: 10},
	}, nil)
	markerRepo.EXPECT().GetMarkerTagsMultiple([]uint{1, 2}).Return(map[uint][]data.MarkerTagInfo{}, nil)
	markerRepo.EXPECT().ApplyMarkerDedup(gomock.Any()).Return(cause)

	if _, err := svc.DeduplicateMarkers([]uint{7}, false); !errors.Is(err, cause) {
		t.Fatalf("expected the apply failure, got %v", err)
	}
}

func TestQueueMarkerThumbnails_SkipsDeletedMarkers(t *testing.T) {
	svc, markerRepo, sceneRepo := newTestMarkerService(t)
	svc.markerThumbnailDir = t.TempDir()
	old := filepath.Join(svc.markerThumbnailDir, "marker_1.webp")
	if err := os.WriteFile(old, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	marker := &data.UserSceneMarker{ID: 1, SceneID: 10, Timestamp: 80, ThumbnailPath: "marker_1.webp"}
	markerRepo.EXPECT().GetByID(uint(1)).Return(marker, nil)
	markerRepo.EXPECT().GetByID(uint(2)).Return(nil, errors.New("record not found"))
	// no stored path: the inline thumbnail fails (logged), the old file is gone already
	sceneRepo.EXPECT().GetByID(uint(10)).Return(&data.Scene{ID: 10}, nil)

	svc.QueueMarkerThumbnails([]uint{1, 2})
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("old thumbnail made at the old time should be removed, stat err = %v", err)
	}
}
