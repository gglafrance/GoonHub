package core

import (
	"fmt"
	"goonhub/internal/core/matching"
	"goonhub/internal/data"
	"sort"
)

// duplicateExclusionSet holds pairs of scenes marked as not duplicates of each other
// (data.DuplicateExclusion): fingerprint matching never links them again.
type duplicateExclusionSet map[[2]uint]struct{}

func newDuplicateExclusionSet(rows []data.DuplicateExclusion) duplicateExclusionSet {
	set := make(duplicateExclusionSet, len(rows))
	for _, r := range rows {
		set[exclusionKey(r.SceneIDA, r.SceneIDB)] = struct{}{}
	}
	return set
}

func exclusionKey(a, b uint) [2]uint {
	if a > b {
		a, b = b, a
	}
	return [2]uint{a, b}
}

// has reports whether scenes a and b are excluded from each other.
func (s duplicateExclusionSet) has(a, b uint) bool {
	if len(s) == 0 {
		return false
	}
	_, ok := s[exclusionKey(a, b)]
	return ok
}

// excludedWithAny reports whether scene id is excluded with one of the scenes of others.
func (s duplicateExclusionSet) excludedWithAny(id uint, others map[uint]bool) bool {
	if len(s) == 0 {
		return false
	}
	for other := range others {
		if s.has(id, other) {
			return true
		}
	}
	return false
}

// loadDuplicateExclusions returns the exclusions among the given scenes.
func loadDuplicateExclusions(repo data.DuplicateGroupRepository, sceneIDs []uint) (duplicateExclusionSet, error) {
	rows, err := repo.ListExclusionsAmong(sceneIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to load duplicate exclusions: %w", err)
	}
	return newDuplicateExclusionSet(rows), nil
}

// withoutExcludedMatches drops from matchMap the scenes excluded with the query scene and
// returns the remaining matches and matched IDs, sorted by scene ID.
func withoutExcludedMatches(sceneID uint, matchMap map[uint]matching.MatchResult, excluded duplicateExclusionSet) (map[uint]matching.MatchResult, []matching.MatchResult, []uint) {
	kept := make(map[uint]matching.MatchResult, len(matchMap))
	ids := make([]uint, 0, len(matchMap))
	for id, m := range matchMap {
		if excluded.has(sceneID, id) {
			continue
		}
		kept[id] = m
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	matches := make([]matching.MatchResult, 0, len(ids))
	for _, id := range ids {
		matches = append(matches, kept[id])
	}
	return kept, matches, ids
}

// bestMatchInGroup returns the match with the highest confidence among the other scenes of
// the member's group, leaving out the scenes excluded with the member.
func bestMatchInGroup(sceneID uint, matches []matching.MatchResult, groupScenes map[uint]bool, excluded duplicateExclusionSet) (matching.MatchResult, bool) {
	var best matching.MatchResult
	for _, m := range matches {
		if m.SceneID == sceneID || !groupScenes[m.SceneID] || excluded.has(sceneID, m.SceneID) {
			continue
		}
		if m.ConfidenceScore > best.ConfidenceScore {
			best = m
		}
	}
	return best, best.SceneID != 0
}
