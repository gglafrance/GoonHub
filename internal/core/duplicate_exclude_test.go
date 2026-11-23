package core

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
)

// expectDuplicateTx runs RunInTransaction's callback against the same mock.
func expectDuplicateTx(groupRepo *mocks.MockDuplicateGroupRepository) {
	groupRepo.EXPECT().RunInTransaction(gomock.Any()).DoAndReturn(func(fn func(data.DuplicateGroupRepository) error) error {
		return fn(groupRepo)
	})
}

func excludeTestGroup(status string, best uint, sceneIDs ...uint) *data.DuplicateGroup {
	group := &data.DuplicateGroup{ID: 5, Status: status, SceneCount: len(sceneIDs), BestSceneID: &best}
	for _, id := range sceneIDs {
		group.Members = append(group.Members, data.DuplicateGroupMember{GroupID: 5, SceneID: id, IsBest: id == best})
	}
	return group
}

func TestExcludeMembers_RemovesAndRecordsAndPicksNewBest(t *testing.T) {
	svc, groupRepo, sceneRepo, _, _, _, _ := newTestDuplicateService(t)
	now := time.Now()
	group := excludeTestGroup("unresolved", 10, 10, 20, 30, 40)
	after := &data.DuplicateGroup{ID: 5, Status: "unresolved", SceneCount: 2, Members: []data.DuplicateGroupMember{
		{GroupID: 5, SceneID: 20}, {GroupID: 5, SceneID: 30, IsBest: true},
	}}
	gomock.InOrder(
		groupRepo.EXPECT().GetByIDWithMembers(uint(5)).Return(group, nil),
		groupRepo.EXPECT().GetByIDWithMembers(uint(5)).Return(after, nil),
	)
	// New best among 20 and 30 (20 is trashed): 30
	sceneRepo.EXPECT().GetByIDsIncludingTrashed([]uint{20, 30}).Return([]data.Scene{
		{ID: 20, Width: 3840, Height: 2160, TrashedAt: &now}, {ID: 30, Width: 1920, Height: 1080},
	}, nil).Times(2)
	expectDuplicateTx(groupRepo)
	groupRepo.EXPECT().AddExclusions([]uint{40, 10}, []uint{20, 30}).Return(nil)
	groupRepo.EXPECT().RemoveMembers(uint(5), []uint{40, 10}).Return(nil)
	groupRepo.EXPECT().UpdateSceneCount(uint(5)).Return(nil)
	groupRepo.EXPECT().UpdateMemberBest(uint(5), uint(30), true).Return(nil)
	groupRepo.EXPECT().SetBestScene(uint(5), uint(30)).Return(nil)
	// No trash, no status change, no file deletion: any other call fails the mock

	result, err := svc.ExcludeMembers(5, []uint{40, 10, 40})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(result.Excluded, []uint{40, 10}) || result.Dismissed {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Group == nil || result.Group.SceneCount != 2 || len(result.Group.Members) != 2 {
		t.Fatalf("unexpected group: %+v", result.Group)
	}
}

func TestExcludeMembers_KeepsBestWhenNotExcluded(t *testing.T) {
	svc, groupRepo, sceneRepo, _, _, _, _ := newTestDuplicateService(t)
	group := excludeTestGroup("unresolved", 10, 10, 20, 30)
	groupRepo.EXPECT().GetByIDWithMembers(uint(5)).Return(group, nil).Times(2)
	sceneRepo.EXPECT().GetByIDsIncludingTrashed(gomock.Any()).Return([]data.Scene{{ID: 10}, {ID: 20}, {ID: 30}}, nil)
	expectDuplicateTx(groupRepo)
	groupRepo.EXPECT().AddExclusions([]uint{30}, []uint{10, 20}).Return(nil)
	groupRepo.EXPECT().RemoveMembers(uint(5), []uint{30}).Return(nil)
	groupRepo.EXPECT().UpdateSceneCount(uint(5)).Return(nil)

	if _, err := svc.ExcludeMembers(5, []uint{30}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExcludeMembers_DismissesWhenOneMemberLeft(t *testing.T) {
	svc, groupRepo, sceneRepo, _, _, _, _ := newTestDuplicateService(t)
	group := excludeTestGroup("unresolved", 10, 10, 20)
	groupRepo.EXPECT().GetByIDWithMembers(uint(5)).Return(group, nil).Times(2)
	sceneRepo.EXPECT().GetByIDsIncludingTrashed(gomock.Any()).Return([]data.Scene{{ID: 10}, {ID: 20}}, nil)
	expectDuplicateTx(groupRepo)
	groupRepo.EXPECT().AddExclusions([]uint{20}, []uint{10}).Return(nil)
	groupRepo.EXPECT().RemoveMembers(uint(5), []uint{20}).Return(nil)
	groupRepo.EXPECT().UpdateSceneCount(uint(5)).Return(nil)
	groupRepo.EXPECT().UpdateStatus(uint(5), "dismissed", nil).Return(nil)

	result, err := svc.ExcludeMembers(5, []uint{20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Dismissed {
		t.Fatal("a group left with one member must be dismissed")
	}
}

func TestExcludeMembers_ClearsBestWhenNoSceneLeftToKeep(t *testing.T) {
	svc, groupRepo, sceneRepo, _, _, _, _ := newTestDuplicateService(t)
	now := time.Now()
	group := excludeTestGroup("unresolved", 10, 10, 20, 30)
	groupRepo.EXPECT().GetByIDWithMembers(uint(5)).Return(group, nil).Times(2)
	sceneRepo.EXPECT().GetByIDsIncludingTrashed([]uint{20, 30}).Return([]data.Scene{
		{ID: 20, TrashedAt: &now}, {ID: 30, TrashedAt: &now},
	}, nil)
	sceneRepo.EXPECT().GetByIDsIncludingTrashed([]uint{10, 20, 30}).Return(nil, nil)
	expectDuplicateTx(groupRepo)
	groupRepo.EXPECT().AddExclusions([]uint{10}, []uint{20, 30}).Return(nil)
	groupRepo.EXPECT().RemoveMembers(uint(5), []uint{10}).Return(nil)
	groupRepo.EXPECT().UpdateSceneCount(uint(5)).Return(nil)
	groupRepo.EXPECT().ClearBestScene(uint(5)).Return(nil)

	if _, err := svc.ExcludeMembers(5, []uint{10}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExcludeMembers_FailureInTransactionIsReturned(t *testing.T) {
	svc, groupRepo, _, _, _, _, _ := newTestDuplicateService(t)
	group := excludeTestGroup("unresolved", 10, 10, 20, 30)
	groupRepo.EXPECT().GetByIDWithMembers(uint(5)).Return(group, nil)
	expectDuplicateTx(groupRepo)
	groupRepo.EXPECT().AddExclusions([]uint{30}, []uint{10, 20}).Return(errors.New("db down"))

	if _, err := svc.ExcludeMembers(5, []uint{30}); err == nil {
		t.Fatal("expected an error")
	}
}

func TestExcludeMembers_Validation(t *testing.T) {
	tests := []struct {
		name     string
		group    *data.DuplicateGroup
		sceneIDs []uint
	}{
		{"no scenes", nil, nil},
		{"not a member", excludeTestGroup("unresolved", 10, 10, 20), []uint{99}},
		{"every member", excludeTestGroup("unresolved", 10, 10, 20), []uint{10, 20}},
		{"resolved group", excludeTestGroup("resolved", 10, 10, 20, 30), []uint{30}},
		{"dismissed group", excludeTestGroup("dismissed", 10, 10, 20, 30), []uint{30}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, groupRepo, _, _, _, _, _ := newTestDuplicateService(t)
			if tt.group != nil {
				groupRepo.EXPECT().GetByIDWithMembers(uint(5)).Return(tt.group, nil)
			}
			if _, err := svc.ExcludeMembers(5, tt.sceneIDs); !apperrors.IsValidation(err) {
				t.Fatalf("expected a validation error, got %v", err)
			}
		})
	}
}

func TestExcludeMembers_NotFound(t *testing.T) {
	svc, groupRepo, _, _, _, _, _ := newTestDuplicateService(t)
	groupRepo.EXPECT().GetByIDWithMembers(uint(5)).Return(nil, gorm.ErrRecordNotFound)
	if _, err := svc.ExcludeMembers(5, []uint{1}); !apperrors.IsNotFound(err) {
		t.Fatalf("expected a not found error, got %v", err)
	}
}
