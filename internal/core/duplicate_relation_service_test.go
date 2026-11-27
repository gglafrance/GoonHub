package core

import (
	"strings"
	"testing"
	"time"

	"goonhub/internal/data"

	"go.uber.org/mock/gomock"
)

// containmentGroup is group 64: scene 4446 (5,627 s compilation, 720p) contains scene 4448
// (1,642 s clip, 1080p).
func containmentGroup() *data.DuplicateGroup {
	id4446, id4448 := uint(4446), uint(4448)
	best := id4448
	return &data.DuplicateGroup{ID: 64, Status: "unresolved", BestSceneID: &best, Members: []data.DuplicateGroupMember{
		{GroupID: 64, SceneID: 4446, RefSceneID: &id4448, Relation: data.DuplicateRelationContains},
		{GroupID: 64, SceneID: 4448, RefSceneID: &id4446, Relation: data.DuplicateRelationContained, IsBest: true},
	}}
}

func TestAutoSelectBest_SkipsNonIdenticalGroups(t *testing.T) {
	svc, groupRepo, _, _, _, _, _ := newTestDuplicateService(t)
	unverified := &data.DuplicateGroup{ID: 65, Status: "unresolved", Members: []data.DuplicateGroupMember{
		{GroupID: 65, SceneID: 1, Relation: data.DuplicateRelationIdentical},
		{GroupID: 65, SceneID: 2, Relation: data.DuplicateRelationUnknown},
	}}
	groupRepo.EXPECT().GetByIDWithMembers(uint(64)).Return(containmentGroup(), nil)
	groupRepo.EXPECT().GetByIDWithMembers(uint(65)).Return(unverified, nil)
	// No scene lookup, no UpdateMemberBest nor SetBestScene: the mocks fail on any other call

	result, err := svc.AutoSelectBest([]uint{64, 65})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Selected) != 0 || len(result.Skipped) != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if s := result.Skipped[0]; s.GroupID != 64 || s.Kind != data.DuplicateKindContainment || !strings.Contains(s.Reason, "#4448 is a clip of #4446") {
		t.Fatalf("unexpected skipped containment group: %+v", s)
	}
	if s := result.Skipped[1]; s.GroupID != 65 || s.Kind != data.DuplicateKindUnverified || len(s.Members) != 1 || s.Members[0].SceneID != 2 {
		t.Fatalf("unexpected skipped unverified group: %+v", s)
	}
}

func TestResolveGroups_SkipsNonIdenticalGroups(t *testing.T) {
	for _, autoSelect := range []bool{true, false} {
		svc, groupRepo, sceneRepo, _, _, _, _ := newTestDuplicateService(t)
		best := uint(10)
		identical := &data.DuplicateGroup{ID: 1, Status: "unresolved", BestSceneID: &best, Members: []data.DuplicateGroupMember{
			{GroupID: 1, SceneID: 10, Relation: data.DuplicateRelationIdentical},
			{GroupID: 1, SceneID: 20, Relation: data.DuplicateRelationIdentical},
		}}
		partial := &data.DuplicateGroup{ID: 2, Status: "unresolved", Members: []data.DuplicateGroupMember{
			{GroupID: 2, SceneID: 30, Relation: data.DuplicateRelationPartial},
			{GroupID: 2, SceneID: 40, Relation: data.DuplicateRelationPartial},
		}}

		groupRepo.EXPECT().GetByIDWithMembers(uint(64)).Return(containmentGroup(), nil)
		groupRepo.EXPECT().GetByIDWithMembers(uint(2)).Return(partial, nil)
		// The identical group is resolved: looked up by ResolveGroups, then by ResolveGroup
		groupRepo.EXPECT().GetByIDWithMembers(uint(1)).Return(identical, nil).Times(2)
		if autoSelect {
			sceneRepo.EXPECT().GetByIDsIncludingTrashed([]uint{10, 20}).Return([]data.Scene{
				{ID: 10, Width: 1920, Height: 1080}, {ID: 20, Width: 1280, Height: 720},
			}, nil)
		}
		groupRepo.EXPECT().UpdateMemberBest(uint(1), uint(10), true).Return(nil)
		groupRepo.EXPECT().UpdateMemberBest(uint(1), uint(20), false).Return(nil)
		now := time.Now()
		// Only scene 20 goes to the trash: neither 4446, 4448, 30 nor 40
		sceneRepo.EXPECT().GetByIDIncludingTrashed(uint(20)).Return(&data.Scene{ID: 20}, nil)
		sceneRepo.EXPECT().MoveToTrash(uint(20)).Return(&now, nil)
		groupRepo.EXPECT().SetBestScene(uint(1), uint(10)).Return(nil)
		groupRepo.EXPECT().UpdateStatus(uint(1), "resolved", gomock.Any()).Return(nil)

		result := svc.ResolveGroups([]uint{64, 1, 2}, autoSelect, false)
		if len(result.Resolved) != 1 || result.Resolved[0] != 1 || len(result.Failed) != 0 {
			t.Fatalf("auto_select=%t: unexpected result: %+v", autoSelect, result)
		}
		if len(result.Skipped) != 2 || result.Skipped[0].GroupID != 64 || result.Skipped[1].GroupID != 2 ||
			result.Skipped[1].Kind != data.DuplicateKindPartial {
			t.Fatalf("auto_select=%t: unexpected skipped groups: %+v", autoSelect, result.Skipped)
		}
	}
}

func TestResolveGroup_ManualResolutionAllowedForContainment(t *testing.T) {
	svc, groupRepo, sceneRepo, _, _, _, _ := newTestDuplicateService(t)
	groupRepo.EXPECT().GetByIDWithMembers(uint(64)).Return(containmentGroup(), nil)
	groupRepo.EXPECT().UpdateMemberBest(uint(64), uint(4446), true).Return(nil)
	groupRepo.EXPECT().UpdateMemberBest(uint(64), uint(4448), false).Return(nil)
	now := time.Now()
	sceneRepo.EXPECT().GetByIDIncludingTrashed(uint(4448)).Return(&data.Scene{ID: 4448}, nil)
	sceneRepo.EXPECT().MoveToTrash(uint(4448)).Return(&now, nil)
	groupRepo.EXPECT().SetBestScene(uint(64), uint(4446)).Return(nil)
	groupRepo.EXPECT().UpdateStatus(uint(64), "resolved", gomock.Any()).Return(nil)

	// The user chose to keep the compilation: allowed whatever the relation
	if _, err := svc.ResolveGroup(64, 4446, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnrichGroup_ExposesRelationAndKind(t *testing.T) {
	svc, _, sceneRepo, _, _, _, _ := newTestDuplicateService(t)
	sceneRepo.EXPECT().GetByIDsIncludingTrashed([]uint{4446, 4448}).Return([]data.Scene{
		{ID: 4446, Duration: 5627}, {ID: 4448, Duration: 1642},
	}, nil)
	group := containmentGroup()
	group.Members[0].Relation = "" // never classified: reported as unknown
	enriched, err := svc.enrichGroup(group)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if enriched.Kind != data.DuplicateKindContainment {
		t.Fatalf("unexpected kind: %q", enriched.Kind)
	}
	if enriched.Members[0].Relation != data.DuplicateRelationUnknown || enriched.Members[1].Relation != data.DuplicateRelationContained {
		t.Fatalf("unexpected relations: %+v", enriched.Members)
	}
}

func TestBackfillMemberRelations(t *testing.T) {
	svc, groupRepo, _, _, _, _, _ := newTestDuplicateService(t)
	// A full first batch of identical members, then one batch with a clip and a member whose
	// scene duration is unknown (left unknown, not written)
	first := make([]data.DuplicateMemberRelationInput, relationBackfillBatch)
	for i := range first {
		first[i] = data.DuplicateMemberRelationInput{
			ID: uint(i + 1), MatchStart: seg(0), MatchEnd: seg(600), RefStart: seg(0), RefEnd: seg(600),
			Duration: 600, RefDuration: 600,
		}
	}
	last := []data.DuplicateMemberRelationInput{
		{ID: 1001, MatchStart: seg(1.5), MatchEnd: seg(1640), RefStart: seg(2101.5), RefEnd: seg(3740), Duration: 1642, RefDuration: 5627},
		{ID: 1002, MatchStart: seg(0), MatchEnd: seg(600), RefStart: seg(0), RefEnd: seg(600), Duration: 0, RefDuration: 600},
	}
	groupRepo.EXPECT().ListMembersForRelationBackfill(uint(0), relationBackfillBatch).Return(first, nil)
	groupRepo.EXPECT().ListMembersForRelationBackfill(uint(relationBackfillBatch), relationBackfillBatch).Return(last, nil)
	groupRepo.EXPECT().SetMemberRelation(gomock.Any(), data.DuplicateRelationIdentical).Return(nil).Times(relationBackfillBatch)
	groupRepo.EXPECT().SetMemberRelation(uint(1001), data.DuplicateRelationContained).Return(nil)

	classified, err := svc.BackfillMemberRelations()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if classified != relationBackfillBatch+1 {
		t.Fatalf("expected %d members classified, got %d", relationBackfillBatch+1, classified)
	}
}
