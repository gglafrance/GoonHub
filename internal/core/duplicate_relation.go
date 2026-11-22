package core

import (
	"fmt"
	"math"
	"strings"

	"goonhub/internal/data"
)

// Thresholds deciding whether a matching segment covers "most" of a scene.
//
// A fingerprint match spans from the first to the last matching position, so two copies of the
// same video rarely match over 100 % of their length: a studio intro, an outro, end credits or
// a few seconds trimmed at either end stay outside the match. A scene is therefore considered
// mostly covered when either:
//   - the match spans at least duplicateCoveredRatio (90 %) of it, or
//   - at most duplicateCoverageSlackSeconds (45 s, a typical intro plus outro) of it is left
//     outside the match, as long as the match still spans duplicateSlackMinRatio (75 %) of it
//     (the absolute slack alone would let a short clip "cover" another short clip).
//
// Examples: a 30 min scene matched over 27 min is covered (90 %); a 4 min scene matched over
// 3 min 20 s is covered (40 s left, 83 %); a 1 min scene matched over 30 s is not (50 %).
const (
	duplicateCoveredRatio         = 0.90
	duplicateCoverageSlackSeconds = 45.0
	duplicateSlackMinRatio        = 0.75
)

// DuplicateRelationInput is what the relation of a group member to its reference (the scene it
// was matched against) is derived from: the matching segment in each scene, in seconds, and the
// durations of both scenes, in seconds.
type DuplicateRelationInput struct {
	MatchStart  *float64
	MatchEnd    *float64
	RefStart    *float64
	RefEnd      *float64
	Duration    int
	RefDuration int
}

// ClassifyDuplicateRelation tells how a group member relates to its reference:
//   - identical: the match covers most of both scenes (see the thresholds above);
//   - contained: it covers most of this scene but not of the reference (this scene is a clip
//     of the reference, e.g. one scene of a compilation);
//   - contains: it covers most of the reference but not of this scene (this scene is the
//     longer one, e.g. the compilation);
//   - partial: it covers a minor part of both scenes;
//   - unknown: a segment or a duration is missing, or a segment is empty (zero length, or
//     outside the scene). Unknown is never considered safe for automatic resolution.
func ClassifyDuplicateRelation(in DuplicateRelationInput) string {
	covered, ok := segmentCoverage(in.MatchStart, in.MatchEnd, in.Duration)
	if !ok {
		return data.DuplicateRelationUnknown
	}
	refCovered, ok := segmentCoverage(in.RefStart, in.RefEnd, in.RefDuration)
	if !ok {
		return data.DuplicateRelationUnknown
	}
	member := mostlyCovered(covered, in.Duration)
	ref := mostlyCovered(refCovered, in.RefDuration)
	switch {
	case member && ref:
		return data.DuplicateRelationIdentical
	case member:
		return data.DuplicateRelationContained
	case ref:
		return data.DuplicateRelationContains
	default:
		return data.DuplicateRelationPartial
	}
}

// segmentCoverage returns how many seconds of a scene of the given duration the segment
// covers, clamped to the scene. ok is false when the segment or the duration is missing, or
// when the segment is empty once clamped.
func segmentCoverage(start, end *float64, duration int) (float64, bool) {
	if start == nil || end == nil || duration <= 0 {
		return 0, false
	}
	s, e := *start, *end
	if math.IsNaN(s) || math.IsNaN(e) {
		return 0, false
	}
	s = math.Max(s, 0)
	e = math.Min(e, float64(duration))
	if e <= s {
		return 0, false
	}
	return e - s, true
}

// mostlyCovered tells whether covered seconds are "most" of a scene of the given duration.
func mostlyCovered(covered float64, duration int) bool {
	d := float64(duration)
	ratio := covered / d
	if ratio >= duplicateCoveredRatio {
		return true
	}
	return d-covered <= duplicateCoverageSlackSeconds && ratio >= duplicateSlackMinRatio
}

// relationOfMember classifies a stored member given the durations of the scenes (by ID).
func relationOfMember(m *data.DuplicateGroupMember, durations map[uint]int) string {
	in := DuplicateRelationInput{
		MatchStart: m.MatchStart, MatchEnd: m.MatchEnd,
		RefStart: m.RefStart, RefEnd: m.RefEnd,
		Duration: durations[m.SceneID],
	}
	if m.RefSceneID != nil {
		in.RefDuration = durations[*m.RefSceneID]
	}
	return ClassifyDuplicateRelation(in)
}

// DuplicateGroupKind summarizes the relations of a group's members: identical when every
// member is identical to its reference, else containment when one is contained in or contains
// its reference, else partial when one only partly matches, else unverified (some members have
// no relation yet). Only identical groups can be resolved automatically.
func DuplicateGroupKind(relations []string) string {
	var containment, partial, unknown bool
	for _, r := range relations {
		switch r {
		case data.DuplicateRelationIdentical:
		case data.DuplicateRelationContained, data.DuplicateRelationContains:
			containment = true
		case data.DuplicateRelationPartial:
			partial = true
		default:
			unknown = true
		}
	}
	switch {
	case containment:
		return data.DuplicateKindContainment
	case partial:
		return data.DuplicateKindPartial
	case unknown || len(relations) == 0:
		return data.DuplicateKindUnverified
	default:
		return data.DuplicateKindIdentical
	}
}

// IsValidDuplicateKind tells whether kind is a group kind accepted as a list filter.
func IsValidDuplicateKind(kind string) bool {
	switch kind {
	case data.DuplicateKindIdentical, data.DuplicateKindContainment, data.DuplicateKindPartial, data.DuplicateKindUnverified:
		return true
	}
	return false
}

// DuplicateSkippedMember is a member that prevents the automatic resolution of its group.
type DuplicateSkippedMember struct {
	SceneID    uint   `json:"scene_id"`
	Relation   string `json:"relation"`
	RefSceneID *uint  `json:"ref_scene_id,omitempty"`
}

// DuplicateSkippedGroup is a group left untouched by an automatic action because not all its
// scenes are identical. Reason lists the members that are not, e.g.
// "#4446 contains #4448; #4448 is a clip of #4446".
type DuplicateSkippedGroup struct {
	GroupID uint                     `json:"group_id"`
	Kind    string                   `json:"kind"`
	Reason  string                   `json:"reason"`
	Members []DuplicateSkippedMember `json:"members"`
}

// automaticResolutionBlocker returns why a group must not be resolved automatically (keeping
// one scene and trashing the others), or nil when every member is identical to its reference.
// Every member counts, trashed ones included: resolving schedules their deletion too, and a
// non-identical kept scene would mean losing content only the others have.
func automaticResolutionBlocker(group *data.DuplicateGroup) *DuplicateSkippedGroup {
	relations := make([]string, len(group.Members))
	for i, m := range group.Members {
		relations[i] = m.Relation
	}
	kind := DuplicateGroupKind(relations)
	if kind == data.DuplicateKindIdentical {
		return nil
	}
	skipped := &DuplicateSkippedGroup{GroupID: group.ID, Kind: kind, Members: []DuplicateSkippedMember{}}
	details := make([]string, 0, len(group.Members))
	for _, m := range group.Members {
		if m.Relation == data.DuplicateRelationIdentical {
			continue
		}
		relation := data.DuplicateRelationOrUnknown(m.Relation)
		skipped.Members = append(skipped.Members, DuplicateSkippedMember{SceneID: m.SceneID, Relation: relation, RefSceneID: m.RefSceneID})
		details = append(details, describeRelation(m.SceneID, relation, m.RefSceneID))
	}
	if len(details) == 0 {
		skipped.Reason = "group has no members"
	} else {
		skipped.Reason = strings.Join(details, "; ")
	}
	return skipped
}

// describeRelation explains a non-identical relation in a few words.
func describeRelation(sceneID uint, relation string, refID *uint) string {
	ref := "its match"
	if refID != nil {
		ref = fmt.Sprintf("#%d", *refID)
	}
	switch relation {
	case data.DuplicateRelationContained:
		return fmt.Sprintf("#%d is a clip of %s", sceneID, ref)
	case data.DuplicateRelationContains:
		return fmt.Sprintf("#%d contains %s", sceneID, ref)
	case data.DuplicateRelationPartial:
		return fmt.Sprintf("#%d only partly matches %s", sceneID, ref)
	default:
		return fmt.Sprintf("#%d is unverified (no matching segment yet)", sceneID)
	}
}
