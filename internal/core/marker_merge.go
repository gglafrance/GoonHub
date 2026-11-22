package core

import (
	"math"
	"sort"
	"strings"

	"goonhub/internal/data"
)

// Moving the markers of a duplicate scene to the scene kept when its group is resolved.
//
// A time t in the trashed scene is the same moment as t - OtherStart + ThisStart in the kept
// scene, using the segment mapped between both scenes (SimilarSegment, This being the kept
// scene and Other the trashed one): a clip that starts 70 s into a compilation has its markers
// moved 70 s later. Only the part of the trashed scene that matches the kept one can be placed
// reliably: a marker outside it is left on the trashed scene (still there if it is restored),
// unless both scenes are identical and the shifted time still lands in the kept scene.

// markerSegmentEdgeTolerance (seconds) widens the matching segment when deciding whether a
// marker falls inside it: segment edges come from fingerprint windows (about a second) and
// marker times are whole seconds, so a marker set on the very first frame of the match could
// otherwise fall just outside it.
const markerSegmentEdgeTolerance = 1.0

// markerDuplicateToleranceSeconds is how far apart (seconds) the starts of two markers of a
// user can be for them to be the same marker. Marker times are whole seconds: a marker moved
// with a fractional offset rounds to either neighbouring second, and the same moment marked by
// hand on two copies of a scene usually differs by a second or two. 2 s catches those while
// two distinct moments with the same label are rarely that close.
const markerDuplicateToleranceSeconds = 2

// MarkerMergeSummary counts what happened to the markers when a group was resolved.
type MarkerMergeSummary struct {
	// Moved markers went to the kept scene (Shifted included)
	Moved int `json:"moved"`
	// Shifted markers were moved with their times changed (offset, or end clamped)
	Shifted int `json:"shifted"`
	// Deduplicated markers were deleted from the kept scene as duplicates of another one
	Deduplicated int `json:"deduplicated"`
	// LeftBehind markers could not be placed in the kept scene: they stay on the trashed scene
	LeftBehind int `json:"left_behind"`
}

// Add accumulates another summary (bulk resolution).
func (s *MarkerMergeSummary) Add(o MarkerMergeSummary) {
	s.Moved += o.Moved
	s.Shifted += o.Shifted
	s.Deduplicated += o.Deduplicated
	s.LeftBehind += o.LeftBehind
}

// markerShift maps the times of a trashed duplicate onto the kept scene.
type markerShift struct {
	// segment maps the kept scene (This) to the trashed one (Other); nil when not recorded
	segment *SimilarSegment
	// identical: both scenes have the same content
	identical bool
	// keptDuration is the duration of the kept scene (seconds), 0 when unknown
	keptDuration int
}

// markerPlacement is where a marker of the trashed scene goes in the kept scene.
type markerPlacement struct {
	keep      bool
	timestamp int
	end       *int
	// shifted: the times changed (offset, or end clamped)
	shifted bool
	// reason the marker is left behind (keep false)
	reason string
}

// place tells where a marker (start, optional end, in the trashed scene) lands in the kept
// scene:
//   - no segment: identical scenes keep the same times (offset 0), others can't be placed;
//   - start inside the matching segment: shifted by the offset, an end past the segment
//     clamped to it (to the kept scene's end for identical scenes);
//   - start outside: kept only for identical scenes when the shifted time is inside the kept
//     scene.
//
// The kept scene's duration, when known, bounds every time; an end that does not stay after
// the start once clamped is dropped (the marker becomes a point).
func (s markerShift) place(start int, end *int) markerPlacement {
	if s.segment == nil {
		if !s.identical {
			return markerPlacement{reason: "no matching segment between the scenes"}
		}
		return s.placeWith(start, end, 0, false)
	}
	t := float64(start)
	inside := t >= s.segment.OtherStart-markerSegmentEdgeTolerance &&
		t <= s.segment.OtherEnd+markerSegmentEdgeTolerance
	if !inside && !s.identical {
		return markerPlacement{reason: "outside the part matching the kept scene"}
	}
	return s.placeWith(start, end, s.segment.ThisStart-s.segment.OtherStart, inside)
}

func (s markerShift) placeWith(start int, end *int, offset float64, inside bool) markerPlacement {
	ts := int(math.Round(float64(start) + offset))
	if ts < 0 || (s.keptDuration > 0 && ts > s.keptDuration) {
		if !inside {
			return markerPlacement{reason: "outside the kept scene once shifted"}
		}
		// Inside the segment, only the edge tolerance can push it out: clamp
		ts = min(max(ts, 0), max(s.keptDuration, 0))
	}
	p := markerPlacement{keep: true, timestamp: ts, shifted: ts != start}
	if end == nil {
		return p
	}
	e := int(math.Round(float64(*end) + offset))
	limit := -1
	if s.segment != nil && !s.identical {
		limit = int(math.Round(s.segment.ThisEnd))
	}
	if s.keptDuration > 0 && (limit < 0 || s.keptDuration < limit) {
		limit = s.keptDuration
	}
	if limit >= 0 && e > limit {
		e = limit
	}
	if e > ts {
		p.end = &e
	}
	if p.end == nil || *p.end != *end {
		p.shifted = true
	}
	return p
}

// markerShiftBetween builds the mapping of the trashed member's times onto the kept member,
// from the segments stored in the group (similarMatch: direct, inverted, or through a shared
// reference). The pair is identical when that relation is (relative to the kept scene) or when
// every member of the group is identical to its reference.
func markerShiftBetween(group *data.DuplicateGroup, keptID, trashedID uint, scenes map[uint]data.Scene) markerShift {
	shift := markerShift{keptDuration: scenes[keptID].Duration}
	if group == nil {
		return shift
	}
	var kept, trashed *data.DuplicateMemberScene
	relations := make([]string, len(group.Members))
	for i := range group.Members {
		m := &group.Members[i]
		relations[i] = m.Relation
		row := &data.DuplicateMemberScene{
			GroupID: group.ID, SceneID: m.SceneID, MatchStart: m.MatchStart, MatchEnd: m.MatchEnd,
			RefSceneID: m.RefSceneID, RefStart: m.RefStart, RefEnd: m.RefEnd, Relation: m.Relation,
			Duration: scenes[m.SceneID].Duration,
		}
		switch m.SceneID {
		case keptID:
			kept = row
		case trashedID:
			trashed = row
		}
	}
	if kept == nil || trashed == nil {
		return shift
	}
	relation, refID, segment := similarMatch(kept, trashed)
	shift.segment = segment
	shift.identical = DuplicateGroupKind(relations) == data.DuplicateKindIdentical ||
		(refID != nil && *refID == keptID && relation == data.DuplicateRelationIdentical)
	return shift
}

// markerDedupGroup is a set of markers of a user that are the same marker: keep stays (with
// label and the tags addTags it lacks), remove are deleted.
type markerDedupGroup struct {
	keep    data.MarkerWithTags
	remove  []data.MarkerWithTags
	label   string
	addTags []data.MarkerDedupTag
}

// normalizeMarkerLabel compares labels regardless of case and spacing.
func normalizeMarkerLabel(label string) string {
	return strings.ToLower(strings.Join(strings.Fields(label), " "))
}

// markersAlike tells whether two markers can be the same: their labels match (case and
// spacing aside, an empty label matching any) and their starts are close enough.
func markersAlike(label string, anchor int, m data.MarkerWithTags) bool {
	if d := m.Timestamp - anchor; d > markerDuplicateToleranceSeconds || d < -markerDuplicateToleranceSeconds {
		return false
	}
	other := normalizeMarkerLabel(m.Label)
	return label == "" || other == "" || label == other
}

// betterMarkerToKeep orders the markers of a duplicate set: the one with an end, then with
// more tags, then a labelled one, then the earliest created (then the lowest ID).
func betterMarkerToKeep(a, b data.MarkerWithTags) bool {
	if (a.EndTimestamp != nil) != (b.EndTimestamp != nil) {
		return a.EndTimestamp != nil
	}
	if len(a.Tags) != len(b.Tags) {
		return len(a.Tags) > len(b.Tags)
	}
	if (a.Label != "") != (b.Label != "") {
		return a.Label != ""
	}
	if !a.CreatedAt.Equal(b.CreatedAt) {
		return a.CreatedAt.Before(b.CreatedAt)
	}
	return a.ID < b.ID
}

// planMarkerDedup finds, per user, the markers that are the same marker (markersAlike) and
// tells which one of each set to keep. A set is anchored on its earliest marker (by time):
// a marker joins the first set whose anchor is within the tolerance and whose label matches
// (the first non-empty label of the set), so a chain of markers 2 s apart is not folded into
// one. The kept marker gets the set's label when it has none, the tags of the others, and
// keeps its end (the ordering prefers a marker with an end).
func planMarkerDedup(markers []data.MarkerWithTags) []markerDedupGroup {
	sorted := make([]data.MarkerWithTags, len(markers))
	copy(sorted, markers)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].UserID != sorted[j].UserID {
			return sorted[i].UserID < sorted[j].UserID
		}
		if sorted[i].Timestamp != sorted[j].Timestamp {
			return sorted[i].Timestamp < sorted[j].Timestamp
		}
		return sorted[i].ID < sorted[j].ID
	})

	type set struct {
		anchor  int
		label   string // normalized
		display string // first non-empty label, as written
		members []data.MarkerWithTags
	}
	var groups []markerDedupGroup
	flush := func(sets []*set) {
		for _, st := range sets {
			if len(st.members) < 2 {
				continue
			}
			best := 0
			for i := 1; i < len(st.members); i++ {
				if betterMarkerToKeep(st.members[i], st.members[best]) {
					best = i
				}
			}
			g := markerDedupGroup{keep: st.members[best], label: st.members[best].Label}
			if g.label == "" {
				g.label = st.display
			}
			has := make(map[uint]bool, len(g.keep.Tags))
			for _, t := range g.keep.Tags {
				has[t.ID] = true
			}
			for i, m := range st.members {
				if i == best {
					continue
				}
				g.remove = append(g.remove, m)
				for _, t := range m.Tags {
					if !has[t.ID] {
						has[t.ID] = true
						g.addTags = append(g.addTags, data.MarkerDedupTag{TagID: t.ID, IsFromLabel: t.IsFromLabel})
					}
				}
			}
			groups = append(groups, g)
		}
	}

	var sets []*set
	var user uint
	for i, m := range sorted {
		if i == 0 || m.UserID != user {
			flush(sets)
			sets = nil
			user = m.UserID
		}
		var joined *set
		for _, st := range sets {
			if markersAlike(st.label, st.anchor, m) {
				joined = st
				break
			}
		}
		if joined == nil {
			sets = append(sets, &set{anchor: m.Timestamp, label: normalizeMarkerLabel(m.Label), display: m.Label, members: []data.MarkerWithTags{m}})
			continue
		}
		joined.members = append(joined.members, m)
		if joined.label == "" && m.Label != "" {
			joined.label = normalizeMarkerLabel(m.Label)
			joined.display = m.Label
		}
	}
	flush(sets)
	return groups
}
