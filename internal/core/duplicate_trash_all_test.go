package core

import (
	"errors"
	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"goonhub/internal/mocks"
	"reflect"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
)

func TestTrashAllGroup_TrashesEveryScene(t *testing.T) {
	svc, groupRepo, sceneRepo, _, _, _, _ := newTestDuplicateService(t)
	// Deletion of resolved duplicates is on: trash-all must not schedule it (no Get expected)
	svc.SetConfigRepo(mocks.NewMockDuplicationConfigRepository(gomock.NewController(t)))
	events, unsubscribe := subscribeTrashEvents(svc)
	defer unsubscribe()

	best := uint(10)
	earlier := time.Now().Add(-time.Hour)
	group := &data.DuplicateGroup{ID: 1, Status: "unresolved", BestSceneID: &best, Members: []data.DuplicateGroupMember{
		{GroupID: 1, SceneID: 10, IsBest: true},
		{GroupID: 1, SceneID: 20},
		{GroupID: 1, SceneID: 30}, // already in the trash
		{GroupID: 1, SceneID: 40}, // missing
	}}
	now := time.Now()
	groupRepo.EXPECT().GetByIDWithMembers(uint(1)).Return(group, nil)
	sceneRepo.EXPECT().GetByIDsIncludingTrashed([]uint{10, 20, 30, 40}).Return([]data.Scene{
		{ID: 10}, {ID: 20}, {ID: 30, TrashedAt: &earlier},
	}, nil)
	sceneRepo.EXPECT().GetByIDIncludingTrashed(uint(10)).Return(&data.Scene{ID: 10, Title: "best"}, nil)
	sceneRepo.EXPECT().MoveToTrash(uint(10)).Return(&now, nil)
	sceneRepo.EXPECT().GetByIDIncludingTrashed(uint(20)).Return(&data.Scene{ID: 20, Title: "other"}, nil)
	sceneRepo.EXPECT().MoveToTrash(uint(20)).Return(&now, nil)
	groupRepo.EXPECT().UpdateMemberBest(uint(1), uint(10), false).Return(nil)
	groupRepo.EXPECT().ClearBestScene(uint(1)).Return(nil)
	groupRepo.EXPECT().UpdateStatus(uint(1), "resolved", gomock.Not(gomock.Nil())).Return(nil)

	result, err := svc.TrashAllGroup(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(result.Trashed, []uint{10, 20}) {
		t.Fatalf("expected scenes 10 and 20 trashed, got %v", result.Trashed)
	}
	if !reflect.DeepEqual(result.Skipped, []uint{30, 40}) {
		t.Fatalf("expected scenes 30 and 40 skipped, got %v", result.Skipped)
	}
	for _, want := range []uint{10, 20} {
		select {
		case ev := <-events:
			if ev.Type != "scene:trashed" || ev.SceneID != want {
				t.Fatalf("expected scene:trashed for %d, got %+v", want, ev)
			}
		case <-time.After(time.Second):
			t.Fatalf("no scene:trashed event for scene %d", want)
		}
	}
}

func TestTrashAllGroup_NotFound(t *testing.T) {
	svc, groupRepo, _, _, _, _, _ := newTestDuplicateService(t)
	groupRepo.EXPECT().GetByIDWithMembers(uint(7)).Return(nil, gorm.ErrRecordNotFound)

	if _, err := svc.TrashAllGroup(7); !apperrors.IsNotFound(err) {
		t.Fatalf("expected a not found error, got %v", err)
	}
}

func TestTrashAllGroup_TrashFailureLeavesGroupUnresolved(t *testing.T) {
	svc, groupRepo, sceneRepo, _, _, _, _ := newTestDuplicateService(t)
	group := &data.DuplicateGroup{ID: 1, Status: "unresolved", Members: []data.DuplicateGroupMember{
		{GroupID: 1, SceneID: 10}, {GroupID: 1, SceneID: 20},
	}}
	groupRepo.EXPECT().GetByIDWithMembers(uint(1)).Return(group, nil)
	sceneRepo.EXPECT().GetByIDsIncludingTrashed([]uint{10, 20}).Return([]data.Scene{{ID: 10}, {ID: 20}}, nil)
	sceneRepo.EXPECT().GetByIDIncludingTrashed(uint(10)).Return(&data.Scene{ID: 10}, nil)
	sceneRepo.EXPECT().MoveToTrash(uint(10)).Return(nil, errors.New("db down"))
	// No status update, no best change: the group stays unresolved for a retry

	if _, err := svc.TrashAllGroup(1); err == nil {
		t.Fatal("expected an error")
	}
}

func TestTrashAllGroup_SceneGoneMeanwhileIsSkipped(t *testing.T) {
	svc, groupRepo, sceneRepo, _, _, _, _ := newTestDuplicateService(t)
	group := &data.DuplicateGroup{ID: 2, Status: "unresolved", Members: []data.DuplicateGroupMember{{GroupID: 2, SceneID: 10}}}
	groupRepo.EXPECT().GetByIDWithMembers(uint(2)).Return(group, nil)
	sceneRepo.EXPECT().GetByIDsIncludingTrashed([]uint{10}).Return([]data.Scene{{ID: 10}}, nil)
	sceneRepo.EXPECT().GetByIDIncludingTrashed(uint(10)).Return(nil, gorm.ErrRecordNotFound)
	groupRepo.EXPECT().ClearBestScene(uint(2)).Return(nil)
	groupRepo.EXPECT().UpdateStatus(uint(2), "resolved", gomock.Any()).Return(nil)

	result, err := svc.TrashAllGroup(2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Trashed) != 0 || !reflect.DeepEqual(result.Skipped, []uint{10}) {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestTrashAllGroups_ReportsPerGroup(t *testing.T) {
	svc, groupRepo, sceneRepo, _, _, _, _ := newTestDuplicateService(t)
	now := time.Now()
	groupRepo.EXPECT().GetByIDWithMembers(uint(1)).Return(&data.DuplicateGroup{ID: 1, Status: "unresolved", Members: []data.DuplicateGroupMember{
		{GroupID: 1, SceneID: 10}, {GroupID: 1, SceneID: 20},
	}}, nil)
	sceneRepo.EXPECT().GetByIDsIncludingTrashed([]uint{10, 20}).Return([]data.Scene{{ID: 10}, {ID: 20, TrashedAt: &now}}, nil)
	sceneRepo.EXPECT().GetByIDIncludingTrashed(uint(10)).Return(&data.Scene{ID: 10}, nil)
	sceneRepo.EXPECT().MoveToTrash(uint(10)).Return(&now, nil)
	groupRepo.EXPECT().ClearBestScene(uint(1)).Return(nil)
	groupRepo.EXPECT().UpdateStatus(uint(1), "resolved", gomock.Any()).Return(nil)
	groupRepo.EXPECT().GetByIDWithMembers(uint(2)).Return(&data.DuplicateGroup{ID: 2, Status: "resolved"}, nil)
	groupRepo.EXPECT().GetByIDWithMembers(uint(3)).Return(nil, errors.New("boom"))

	result := svc.TrashAllGroups([]uint{1, 2, 3})
	if !reflect.DeepEqual(result.Resolved, []uint{1}) {
		t.Fatalf("expected group 1 resolved, got %v", result.Resolved)
	}
	if len(result.Failed) != 2 || result.Failed[0].GroupID != 2 || result.Failed[1].GroupID != 3 {
		t.Fatalf("expected groups 2 and 3 failed, got %+v", result.Failed)
	}
	if result.TrashedScenes != 1 || result.SkippedScenes != 1 {
		t.Fatalf("expected 1 trashed and 1 skipped scene, got %+v", result)
	}
}

// subscribeTrashEvents listens to the scene service's event bus.
func subscribeTrashEvents(svc *DuplicateService) (<-chan SceneEvent, func()) {
	id, ch := svc.sceneService.EventBus.Subscribe()
	return ch, func() { svc.sceneService.EventBus.Unsubscribe(id) }
}
