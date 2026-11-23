package core

import (
	"reflect"
	"sort"
	"testing"

	"goonhub/internal/core/matching"
	"goonhub/internal/data"
)

func groupSceneIDs(group *data.DuplicateGroup) []uint {
	ids := make([]uint, 0, len(group.Members))
	for _, m := range group.Members {
		ids = append(ids, m.SceneID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func TestProcessMatchesIgnoresExcludedPair(t *testing.T) {
	svc, repo := newMatchingTestService(t)
	repo.exclusions = []data.DuplicateExclusion{data.NewDuplicateExclusion(20, 10)}
	if err := svc.ProcessMatches(10, []matching.MatchResult{
		{SceneID: 20, ConfidenceScore: 0.9, MatchType: "audio"},
		{SceneID: 30, ConfidenceScore: 0.8, MatchType: "audio"},
	}); err != nil {
		t.Fatal(err)
	}
	if len(repo.groups) != 1 {
		t.Fatalf("expected one group, got %v", repo.groups)
	}
	group := repo.groups[1]
	if got := groupSceneIDs(group); !reflect.DeepEqual(got, []uint{10, 30}) || group.SceneCount != 2 {
		t.Fatalf("excluded scene 20 must be left out: members %v, scene_count %d", got, group.SceneCount)
	}
}

func TestProcessMatchesOnlyExcludedMatchesCreatesNoGroup(t *testing.T) {
	svc, repo := newMatchingTestService(t)
	repo.exclusions = []data.DuplicateExclusion{data.NewDuplicateExclusion(10, 20)}
	if err := svc.ProcessMatches(10, []matching.MatchResult{{SceneID: 20, ConfidenceScore: 0.9, MatchType: "audio"}}); err != nil {
		t.Fatal(err)
	}
	if len(repo.groups) != 0 {
		t.Fatalf("a match between excluded scenes must not create a group: %v", repo.groups)
	}
}

func TestProcessMatchesDoesNotRegroupExcludedSceneWithFormerGroup(t *testing.T) {
	// Scene 10 was removed from group 7 (20, 30): it is not a duplicate of 20 nor 30.
	// A later fingerprint of 10 matching 20 and 30 changes nothing, and the dismissed group
	// is not reopened.
	group := matchingTestGroup(7, "dismissed", 20, 30)
	svc, repo := newMatchingTestService(t, group)
	repo.exclusions = []data.DuplicateExclusion{data.NewDuplicateExclusion(10, 20), data.NewDuplicateExclusion(10, 30)}
	if err := svc.ProcessMatches(10, []matching.MatchResult{
		{SceneID: 20, ConfidenceScore: 0.9, MatchType: "audio"},
		{SceneID: 30, ConfidenceScore: 0.9, MatchType: "audio"},
	}); err != nil {
		t.Fatal(err)
	}
	if got := groupSceneIDs(repo.groups[7]); !reflect.DeepEqual(got, []uint{20, 30}) || repo.groups[7].Status != "dismissed" {
		t.Fatalf("group changed: members %v, status %s", got, repo.groups[7].Status)
	}
}

func TestProcessMatchesDoesNotAddQuerySceneExcludedWithAMember(t *testing.T) {
	// 10 is excluded with 20 only; it matches 30, a member of 20's group: joining would put it
	// back with 20. Scene 40, new and only linked through 10, is not added either.
	svc, repo := newMatchingTestService(t, matchingTestGroup(7, "unresolved", 20, 30))
	repo.exclusions = []data.DuplicateExclusion{data.NewDuplicateExclusion(10, 20)}
	if err := svc.ProcessMatches(10, []matching.MatchResult{
		{SceneID: 30, ConfidenceScore: 0.9, MatchType: "audio"},
		{SceneID: 40, ConfidenceScore: 0.9, MatchType: "audio"},
	}); err != nil {
		t.Fatal(err)
	}
	if got := groupSceneIDs(repo.groups[7]); !reflect.DeepEqual(got, []uint{20, 30}) {
		t.Fatalf("expected group 7 unchanged, got %v", got)
	}
}

func TestProcessMatchesThirdSceneDoesNotBridgeExcludedScenes(t *testing.T) {
	// 10 was removed from group 7 (20). A new scene 40 matching both joins group 7, 10 does not.
	svc, repo := newMatchingTestService(t, matchingTestGroup(7, "unresolved", 20))
	repo.exclusions = []data.DuplicateExclusion{data.NewDuplicateExclusion(10, 20)}
	if err := svc.ProcessMatches(40, []matching.MatchResult{
		{SceneID: 10, ConfidenceScore: 0.9, MatchType: "audio"},
		{SceneID: 20, ConfidenceScore: 0.9, MatchType: "audio"},
	}); err != nil {
		t.Fatal(err)
	}
	group := repo.groups[7]
	if got := groupSceneIDs(group); !reflect.DeepEqual(got, []uint{20, 40}) || group.SceneCount != 2 {
		t.Fatalf("expected members 20 and 40, got %v (scene_count %d)", got, group.SceneCount)
	}
}

func TestProcessMatchesNewGroupLeavesOutMatchesExcludedFromEachOther(t *testing.T) {
	svc, repo := newMatchingTestService(t)
	repo.exclusions = []data.DuplicateExclusion{data.NewDuplicateExclusion(30, 20)}
	if err := svc.ProcessMatches(10, []matching.MatchResult{
		{SceneID: 20, ConfidenceScore: 0.9, MatchType: "audio"},
		{SceneID: 30, ConfidenceScore: 0.9, MatchType: "audio"},
	}); err != nil {
		t.Fatal(err)
	}
	group := repo.groups[1]
	if got := groupSceneIDs(group); !reflect.DeepEqual(got, []uint{10, 20}) || group.SceneCount != 2 {
		t.Fatalf("expected members 10 and 20, got %v (scene_count %d)", got, group.SceneCount)
	}
}

func TestBestMatchInGroupSkipsExcludedScenes(t *testing.T) {
	matches := []matching.MatchResult{
		{SceneID: 10, ConfidenceScore: 1},   // itself
		{SceneID: 20, ConfidenceScore: 0.9}, // excluded
		{SceneID: 30, ConfidenceScore: 0.7},
		{SceneID: 40, ConfidenceScore: 0.95}, // not in the group
	}
	group := map[uint]bool{10: true, 20: true, 30: true}
	excluded := newDuplicateExclusionSet([]data.DuplicateExclusion{data.NewDuplicateExclusion(20, 10)})

	best, found := bestMatchInGroup(10, matches, group, excluded)
	if !found || best.SceneID != 30 {
		t.Fatalf("expected scene 30, got %+v (found %v)", best, found)
	}
	if _, found := bestMatchInGroup(10, matches[:2], group, excluded); found {
		t.Fatal("an excluded scene must never give the segment")
	}
	if best, _ := bestMatchInGroup(10, matches, group, nil); best.SceneID != 20 {
		t.Fatalf("without exclusions the best match is 20, got %d", best.SceneID)
	}
}

func TestDuplicateExclusionSetIsSymmetric(t *testing.T) {
	set := newDuplicateExclusionSet([]data.DuplicateExclusion{data.NewDuplicateExclusion(5, 3)})
	if !set.has(3, 5) || !set.has(5, 3) || set.has(3, 4) {
		t.Fatal("unexpected exclusion lookups")
	}
	if !set.excludedWithAny(5, map[uint]bool{1: true, 3: true}) || set.excludedWithAny(5, map[uint]bool{1: true}) {
		t.Fatal("unexpected excludedWithAny")
	}
	if e := data.NewDuplicateExclusion(9, 2); e.SceneIDA != 2 || e.SceneIDB != 9 {
		t.Fatalf("pair not ordered: %+v", e)
	}
}
