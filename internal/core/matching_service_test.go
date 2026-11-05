package core

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	"goonhub/internal/core/matching"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

// memoryDuplicateGroups enforces the database's one-group-per-scene constraint
// and snapshots transactions to exercise rollback across grouping operations.
type memoryDuplicateGroups struct {
	data.DuplicateGroupRepository
	groups      map[uint]*data.DuplicateGroup
	transaction bool
	failAdd     uint
	failReopen  bool
}

func (repo *memoryDuplicateGroups) RunInTransaction(fn func(data.DuplicateGroupRepository) error) error {
	snapshot := make(map[uint]*data.DuplicateGroup, len(repo.groups))
	for id, group := range repo.groups {
		copy := *group
		copy.Members = append([]data.DuplicateGroupMember(nil), group.Members...)
		snapshot[id] = &copy
	}
	repo.transaction = true
	defer func() { repo.transaction = false }()
	if err := fn(repo); err != nil {
		repo.groups = snapshot
		return err
	}
	return nil
}

func (repo *memoryDuplicateGroups) GetGroupsBySceneIDs(ids []uint) (map[uint]uint, error) {
	if !repo.transaction {
		return nil, errors.New("membership lookup must use the transaction")
	}
	result := make(map[uint]uint)
	for _, id := range ids {
		for groupID, group := range repo.groups {
			for _, member := range group.Members {
				if member.SceneID == id {
					result[id] = groupID
				}
			}
		}
	}
	return result, nil
}

func (repo *memoryDuplicateGroups) GetByID(id uint) (*data.DuplicateGroup, error) {
	group, ok := repo.groups[id]
	if !ok {
		return nil, fmt.Errorf("group %d missing", id)
	}
	return group, nil
}

func (repo *memoryDuplicateGroups) GetByIDWithMembers(id uint) (*data.DuplicateGroup, error) {
	return repo.GetByID(id)
}

func (repo *memoryDuplicateGroups) Create(group *data.DuplicateGroup) error {
	group.ID = 1
	for repo.groups[group.ID] != nil {
		group.ID++
	}
	repo.groups[group.ID] = group
	return nil
}

func (repo *memoryDuplicateGroups) AddMember(member *data.DuplicateGroupMember) error {
	if member.SceneID == repo.failAdd {
		return errors.New("member write failed")
	}
	memberships, err := repo.GetGroupsBySceneIDs([]uint{member.SceneID})
	if err != nil {
		return err
	}
	if len(memberships) != 0 {
		return errors.New("unique scene membership violated")
	}
	group := repo.groups[member.GroupID]
	group.Members = append(group.Members, *member)
	return nil
}

func (repo *memoryDuplicateGroups) MergeGroups(target uint, sources []uint) error {
	for _, id := range sources {
		for _, member := range repo.groups[id].Members {
			member.GroupID = target
			repo.groups[target].Members = append(repo.groups[target].Members, member)
		}
		delete(repo.groups, id)
	}
	return nil
}

func (repo *memoryDuplicateGroups) UpdateSceneCount(id uint) error {
	repo.groups[id].SceneCount = len(repo.groups[id].Members)
	return nil
}

func (repo *memoryDuplicateGroups) SetBestScene(id, sceneID uint) error {
	repo.groups[id].BestSceneID = &sceneID
	return nil
}

func (repo *memoryDuplicateGroups) UpdateStatus(id uint, status string, resolvedAt *time.Time) error {
	if repo.failReopen {
		return errors.New("status write failed")
	}
	repo.groups[id].Status = status
	repo.groups[id].ResolvedAt = resolvedAt
	return nil
}

func matchingTestGroup(id uint, status string, sceneIDs ...uint) *data.DuplicateGroup {
	group := &data.DuplicateGroup{ID: id, Status: status, SceneCount: len(sceneIDs)}
	if status == "resolved" {
		now := time.Now()
		group.ResolvedAt = &now
	}
	for _, sceneID := range sceneIDs {
		group.Members = append(group.Members, data.DuplicateGroupMember{GroupID: id, SceneID: sceneID})
	}
	return group
}

func newMatchingTestService(t *testing.T, groups ...*data.DuplicateGroup) (*MatchingService, *memoryDuplicateGroups) {
	t.Helper()
	repo := &memoryDuplicateGroups{groups: make(map[uint]*data.DuplicateGroup)}
	for _, group := range groups {
		repo.groups[group.ID] = group
	}
	scenes := mocks.NewMockSceneRepository(gomock.NewController(t))
	scenes.EXPECT().GetByIDIncludingTrashed(gomock.Any()).DoAndReturn(func(id uint) (*data.Scene, error) {
		return &data.Scene{ID: id, Duration: int(id)}, nil
	}).AnyTimes()
	return NewMatchingService(nil, scenes, repo, nil, zap.NewNop()), repo
}

func TestProcessMatchesPreservesEveryMembership(t *testing.T) {
	tests := []struct {
		name   string
		groups []*data.DuplicateGroup
		wantID uint
	}{
		{"new group", nil, 1},
		{"existing group", []*data.DuplicateGroup{matchingTestGroup(7, "unresolved", 20)}, 7},
		{"query already grouped", []*data.DuplicateGroup{matchingTestGroup(7, "unresolved", 10, 20)}, 7},
		{"resolved group", []*data.DuplicateGroup{matchingTestGroup(7, "resolved", 20)}, 7},
		{"dismissed group", []*data.DuplicateGroup{matchingTestGroup(7, "dismissed", 20)}, 7},
		{"resolved query", []*data.DuplicateGroup{matchingTestGroup(7, "resolved", 10, 20)}, 7},
		{"bridge groups", []*data.DuplicateGroup{matchingTestGroup(7, "unresolved", 20), matchingTestGroup(3, "unresolved", 30)}, 3},
		{"bridge closed groups", []*data.DuplicateGroup{matchingTestGroup(7, "dismissed", 20), matchingTestGroup(3, "resolved", 30)}, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo := newMatchingTestService(t, tt.groups...)
			matches := []matching.MatchResult{
				{SceneID: 20, ConfidenceScore: 0.9, MatchType: "audio"},
				{SceneID: 30, ConfidenceScore: 0.8, MatchType: "visual"},
			}
			if err := svc.ProcessMatches(10, matches); err != nil {
				t.Fatal(err)
			}
			group := repo.groups[tt.wantID]
			if len(repo.groups) != 1 || group == nil {
				t.Fatalf("expected only group %d, got %v", tt.wantID, repo.groups)
			}
			ids := make([]uint, 0, len(group.Members))
			for _, member := range group.Members {
				ids = append(ids, member.SceneID)
			}
			sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
			if !reflect.DeepEqual(ids, []uint{10, 20, 30}) || group.SceneCount != 3 {
				t.Fatalf("missing or repeated members: %v; scene_count=%d", ids, group.SceneCount)
			}
			if group.Status != "unresolved" || group.ResolvedAt != nil {
				t.Fatalf("new duplicates must reopen the group and clear resolved_at: %+v", group)
			}
		})
	}
}

func TestProcessMatchesDoesNotReopenKnownClosedGroups(t *testing.T) {
	for _, status := range []string{"resolved", "dismissed"} {
		t.Run(status, func(t *testing.T) {
			group := matchingTestGroup(7, status, 10, 20)
			svc, repo := newMatchingTestService(t, group)
			if err := svc.ProcessMatches(10, []matching.MatchResult{{SceneID: 20}}); err != nil {
				t.Fatal(err)
			}
			if repo.groups[7].Status != status {
				t.Fatal("reprocessing existing members must preserve the closed status")
			}
		})
	}
}

func TestProcessMatchesRollsBackGroupChanges(t *testing.T) {
	for _, failure := range []string{"add member", "reopen", "merge then add"} {
		t.Run(failure, func(t *testing.T) {
			groups := []*data.DuplicateGroup{matchingTestGroup(7, "resolved", 20)}
			if failure == "merge then add" {
				groups = append(groups, matchingTestGroup(3, "unresolved", 30))
			}
			svc, repo := newMatchingTestService(t, groups...)
			if failure == "reopen" {
				repo.failReopen = true
			} else {
				repo.failAdd = 10
			}
			if err := svc.ProcessMatches(10, []matching.MatchResult{{SceneID: 20}, {SceneID: 30}}); err == nil {
				t.Fatal("expected transaction failure")
			}
			if len(repo.groups) != len(groups) || repo.groups[7].Status != "resolved" || len(repo.groups[7].Members) != 1 {
				t.Fatalf("transaction did not restore the original groups: %+v", repo.groups)
			}
		})
	}
}

func TestProcessMatchesIgnoresSelfAndDuplicateHits(t *testing.T) {
	svc, repo := newMatchingTestService(t)
	if err := svc.ProcessMatches(10, []matching.MatchResult{
		{SceneID: 10},
		{SceneID: 20, ConfidenceScore: 0.2, MatchType: "audio"},
		{SceneID: 20, ConfidenceScore: 0.9, MatchType: "visual"},
	}); err != nil {
		t.Fatal(err)
	}
	group := repo.groups[1]
	if len(group.Members) != 2 || group.Members[1].ConfidenceScore != 0.9 || group.Members[1].MatchType != "visual" {
		t.Fatalf("unexpected normalized members: %+v", group.Members)
	}
}

func TestReopenedGroupDoesNotChooseTrashedBestScene(t *testing.T) {
	group := matchingTestGroup(7, "resolved", 20, 30)
	repo := &memoryDuplicateGroups{groups: map[uint]*data.DuplicateGroup{7: group}}
	scenes := mocks.NewMockSceneRepository(gomock.NewController(t))
	now := time.Now()
	scenes.EXPECT().GetByIDIncludingTrashed(uint(20)).Return(&data.Scene{ID: 20, Duration: 1000, TrashedAt: &now}, nil)
	scenes.EXPECT().GetByIDIncludingTrashed(uint(30)).Return(&data.Scene{ID: 30, Duration: 30}, nil)
	scenes.EXPECT().GetByIDIncludingTrashed(uint(10)).Return(&data.Scene{ID: 10, Duration: 10}, nil)
	svc := NewMatchingService(nil, scenes, repo, nil, zap.NewNop())
	if err := svc.ProcessMatches(10, []matching.MatchResult{{SceneID: 30, MatchType: "audio"}}); err != nil {
		t.Fatal(err)
	}
	if group.BestSceneID == nil || *group.BestSceneID != 30 {
		t.Fatalf("expected live scene 30 as best, got %v", group.BestSceneID)
	}
}
