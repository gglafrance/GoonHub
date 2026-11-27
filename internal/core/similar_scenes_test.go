package core

import (
	"errors"
	"testing"
	"time"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"

	"gorm.io/gorm"
)

func simF64(v float64) *float64 { return &v }
func simUint(v uint) *uint      { return &v }

// simMember builds a member of group 7 matched against ref over [ms, me] here and [rs, re] there.
func simMember(sceneID uint, duration int, ref uint, relation string, ms, me, rs, re float64) data.DuplicateMemberScene {
	return data.DuplicateMemberScene{
		GroupID: 7, GroupStatus: "unresolved", SceneID: sceneID, Duration: duration, MatchType: "audio",
		Title: "scene", RefSceneID: simUint(ref), Relation: relation,
		MatchStart: simF64(ms), MatchEnd: simF64(me), RefStart: simF64(rs), RefEnd: simF64(re),
	}
}

func TestGetSimilarScenes_NotFound(t *testing.T) {
	svc, _, sceneRepo, _, _, _, _ := newTestDuplicateService(t)
	sceneRepo.EXPECT().GetByID(uint(5)).Return(nil, gorm.ErrRecordNotFound)

	_, err := svc.GetSimilarScenes(5, false)
	if !apperrors.IsNotFound(err) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestGetSimilarScenes_NoGroup(t *testing.T) {
	svc, groupRepo, sceneRepo, _, _, _, _ := newTestDuplicateService(t)
	sceneRepo.EXPECT().GetByID(uint(5)).Return(&data.Scene{ID: 5}, nil)
	groupRepo.EXPECT().ListSceneGroupMembers(uint(5), false).Return(nil, nil)

	got, err := svc.GetSimilarScenes(5, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GroupID != nil || got.GroupStatus != "" || got.Scenes == nil || len(got.Scenes) != 0 {
		t.Fatalf("expected an empty list without group, got %+v", got)
	}
}

func TestGetSimilarScenes_RepoError(t *testing.T) {
	svc, groupRepo, sceneRepo, _, _, _, _ := newTestDuplicateService(t)
	sceneRepo.EXPECT().GetByID(uint(5)).Return(&data.Scene{ID: 5}, nil)
	groupRepo.EXPECT().ListSceneGroupMembers(uint(5), true).Return(nil, errors.New("db down"))

	if _, err := svc.GetSimilarScenes(5, true); err == nil {
		t.Fatal("expected an error")
	}
}

func TestGetSimilarScenes_MapsSegmentsAndRelations(t *testing.T) {
	svc, groupRepo, sceneRepo, _, _, _, _ := newTestDuplicateService(t)
	sceneRepo.EXPECT().GetByID(uint(1)).Return(&data.Scene{ID: 1}, nil)

	// 1 (current, 600 s) was matched against 3; 2 is a clip matched against 1; 3 contains 1;
	// 4 was matched against 3 too (shares 3 with 1); 5 against 9, a scene not visible here
	current := simMember(1, 600, 3, data.DuplicateRelationContained, 0, 600, 1000, 1600)
	clip := simMember(2, 90, 1, data.DuplicateRelationContained, 0, 90, 70, 160)
	comp := data.DuplicateMemberScene{GroupID: 7, GroupStatus: "unresolved", SceneID: 3, Duration: 3600, Relation: data.DuplicateRelationUnknown}
	sibling := simMember(4, 300, 3, data.DuplicateRelationContained, 0, 300, 1300, 1600)
	trashedAt := time.Now()
	sibling.TrashedAt = &trashedAt
	far := simMember(5, 120, 9, data.DuplicateRelationPartial, 0, 30, 0, 30)

	groupRepo.EXPECT().ListSceneGroupMembers(uint(1), true).Return(
		[]data.DuplicateMemberScene{current, clip, comp, sibling, far}, nil)

	got, err := svc.GetSimilarScenes(1, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GroupID == nil || *got.GroupID != 7 || got.GroupStatus != "unresolved" {
		t.Fatalf("group not reported: %+v", got)
	}
	if len(got.Scenes) != 4 {
		t.Fatalf("expected 4 similar scenes (current excluded), got %d", len(got.Scenes))
	}
	byID := map[uint]SimilarScene{}
	for _, s := range got.Scenes {
		byID[s.ID] = s
	}
	if _, ok := byID[1]; ok {
		t.Fatal("current scene listed as similar")
	}

	// Matched against the current scene: as stored
	assertSimilar(t, byID[2], data.DuplicateRelationContained, simUint(1), &SimilarSegment{70, 160, 0, 90})
	// The current scene was matched against it: swapped and inverted
	assertSimilar(t, byID[3], data.DuplicateRelationContains, simUint(1), &SimilarSegment{0, 600, 1000, 1600})
	// Through the common reference 3: overlap [1300, 1600] of 3 is 300-600 here, 0-300 there;
	// 4 is wholly covered, 1 only half: 4 is a clip of 1
	assertSimilar(t, byID[4], data.DuplicateRelationContained, simUint(1), &SimilarSegment{300, 600, 0, 300})
	if !byID[4].IsTrashed {
		t.Fatal("trashed scene not flagged")
	}
	// Unrelated reference not visible: relation as stored, no reference, no segment
	assertSimilar(t, byID[5], data.DuplicateRelationPartial, nil, nil)
}

func TestSimilarMatch_StoredRelationWithoutSegment(t *testing.T) {
	current := data.DuplicateMemberScene{SceneID: 1, Duration: 600, RefSceneID: simUint(2), Relation: data.DuplicateRelationContains}
	other := data.DuplicateMemberScene{SceneID: 2, Duration: 60}

	relation, ref, seg := similarMatch(&current, &other)
	if relation != data.DuplicateRelationContained || ref == nil || *ref != 1 || seg != nil {
		t.Fatalf("expected contained relative to 1 without segment, got %s %v %+v", relation, ref, seg)
	}
}

func TestSimilarMatch_DerivesUnknownRelationFromSegment(t *testing.T) {
	current := data.DuplicateMemberScene{SceneID: 1, Duration: 600}
	other := simMember(2, 600, 1, data.DuplicateRelationUnknown, 2, 598, 0, 596)

	relation, _, seg := similarMatch(&current, &other)
	if relation != data.DuplicateRelationIdentical || seg == nil {
		t.Fatalf("expected identical with segment, got %s %+v", relation, seg)
	}
}

func TestSimilarMatch_CommonReferenceWithoutOverlap(t *testing.T) {
	current := simMember(1, 100, 3, data.DuplicateRelationContained, 0, 100, 0, 100)
	other := simMember(2, 100, 3, data.DuplicateRelationContained, 0, 100, 500, 600)

	relation, ref, seg := similarMatch(&current, &other)
	if relation != data.DuplicateRelationContained || ref == nil || *ref != 3 || seg != nil {
		t.Fatalf("expected the stored relation to 3 without segment, got %s %v %+v", relation, ref, seg)
	}
}

func assertSimilar(t *testing.T, s SimilarScene, relation string, ref *uint, seg *SimilarSegment) {
	t.Helper()
	if s.Relation != relation {
		t.Fatalf("scene %d: relation %q, want %q", s.ID, s.Relation, relation)
	}
	if (ref == nil) != (s.RelationRefSceneID == nil) || (ref != nil && *ref != *s.RelationRefSceneID) {
		t.Fatalf("scene %d: relation ref %v, want %v", s.ID, s.RelationRefSceneID, ref)
	}
	if (seg == nil) != (s.Segment == nil) || (seg != nil && *seg != *s.Segment) {
		t.Fatalf("scene %d: segment %+v, want %+v", s.ID, s.Segment, seg)
	}
}
