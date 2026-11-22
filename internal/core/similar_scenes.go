package core

import (
	"errors"
	"fmt"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"

	"gorm.io/gorm"
)

// SimilarSegment maps the matching part of the current scene to the other scene, in seconds:
// this scene from ThisStart to ThisEnd matches the other one from OtherStart to OtherEnd.
type SimilarSegment struct {
	ThisStart  float64 `json:"this_start"`
	ThisEnd    float64 `json:"this_end"`
	OtherStart float64 `json:"other_start"`
	OtherEnd   float64 `json:"other_end"`
}

// SimilarScene is a scene sharing the duplicate group (fingerprint match) of the current scene.
type SimilarScene struct {
	ID              uint    `json:"id"`
	Title           string  `json:"title"`
	Duration        int     `json:"duration"`
	Width           int     `json:"width"`
	Height          int     `json:"height"`
	Size            int64   `json:"size"`
	ThumbnailPath   string  `json:"thumbnail_path"`
	IsTrashed       bool    `json:"is_trashed"`
	IsBest          bool    `json:"is_best"`
	ConfidenceScore float64 `json:"confidence_score"`
	MatchType       string  `json:"match_type"`
	// Relation of this scene to RelationRefSceneID (data.DuplicateRelation*). When
	// RelationRefSceneID is the current scene, contained means "a clip of the current scene"
	// and contains "contains the current scene". Otherwise it is the relation as stored,
	// to the scene it was matched against (nil when that scene is not visible).
	Relation           string `json:"relation"`
	RelationRefSceneID *uint  `json:"relation_ref_scene_id"`
	// Segment is the matching part of both scenes, nil when not recorded
	Segment *SimilarSegment `json:"segment"`
}

// SimilarScenes lists the scenes of the duplicate group of a scene.
type SimilarScenes struct {
	SceneID uint `json:"scene_id"`
	// GroupID and GroupStatus (unresolved, resolved, dismissed) are unset when the scene is in
	// no duplicate group
	GroupID     *uint          `json:"group_id"`
	GroupStatus string         `json:"group_status,omitempty"`
	Scenes      []SimilarScene `json:"scenes"`
}

// GetSimilarScenes returns the other scenes of the duplicate group of a scene, with how each
// one relates to it and the matching segments. Trashed scenes are left out unless
// includeTrashed (they are flagged then). The scene itself must exist and not be trashed.
func (ds *DuplicateService) GetSimilarScenes(sceneID uint, includeTrashed bool) (*SimilarScenes, error) {
	if _, err := ds.sceneRepo.GetByID(sceneID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrSceneNotFound(sceneID)
		}
		return nil, apperrors.NewInternalError("failed to get scene", err)
	}
	rows, err := ds.groupRepo.ListSceneGroupMembers(sceneID, includeTrashed)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list similar scenes", fmt.Errorf("list group members of scene %d: %w", sceneID, err))
	}

	result := &SimilarScenes{SceneID: sceneID, Scenes: []SimilarScene{}}
	var current *data.DuplicateMemberScene
	visible := make(map[uint]bool, len(rows))
	for i := range rows {
		visible[rows[i].SceneID] = true
		if rows[i].SceneID == sceneID {
			current = &rows[i]
		}
	}
	if current == nil {
		return result, nil
	}
	groupID := current.GroupID
	result.GroupID = &groupID
	result.GroupStatus = current.GroupStatus

	for i := range rows {
		other := &rows[i]
		if other.SceneID == sceneID {
			continue
		}
		relation, refID, segment := similarMatch(current, other)
		if refID != nil && !visible[*refID] {
			refID = nil
		}
		result.Scenes = append(result.Scenes, SimilarScene{
			ID:                 other.SceneID,
			Title:              other.Title,
			Duration:           other.Duration,
			Width:              other.Width,
			Height:             other.Height,
			Size:               other.Size,
			ThumbnailPath:      other.ThumbnailPath,
			IsTrashed:          other.TrashedAt != nil,
			IsBest:             other.IsBest,
			ConfidenceScore:    other.ConfidenceScore,
			MatchType:          other.MatchType,
			Relation:           relation,
			RelationRefSceneID: refID,
			Segment:            segment,
		})
	}
	return result, nil
}

// similarMatch tells how other relates to current and which parts of them match, from the
// segments stored on both members (each one against its own reference scene):
//   - other was matched against current: its segment and relation apply as they are;
//   - current was matched against other: its segment applies swapped, its relation inverted;
//   - both were matched against the same third scene: the parts they both match in it are
//     mapped back to each scene, and the relation is derived from that segment;
//   - else the relation of other is returned as stored, to its own reference, without segment.
func similarMatch(current, other *data.DuplicateMemberScene) (string, *uint, *SimilarSegment) {
	thisID := current.SceneID
	otherRefsThis := other.RefSceneID != nil && *other.RefSceneID == current.SceneID
	thisRefsOther := current.RefSceneID != nil && *current.RefSceneID == other.SceneID

	switch {
	case otherRefsThis && hasMemberSegment(other):
		seg := &SimilarSegment{ThisStart: *other.RefStart, ThisEnd: *other.RefEnd, OtherStart: *other.MatchStart, OtherEnd: *other.MatchEnd}
		return refineRelation(other.Relation, seg, current, other), &thisID, seg
	case thisRefsOther && hasMemberSegment(current):
		seg := &SimilarSegment{ThisStart: *current.MatchStart, ThisEnd: *current.MatchEnd, OtherStart: *current.RefStart, OtherEnd: *current.RefEnd}
		return refineRelation(invertDuplicateRelation(current.Relation), seg, current, other), &thisID, seg
	case otherRefsThis:
		return data.DuplicateRelationOrUnknown(other.Relation), &thisID, nil
	case thisRefsOther:
		return invertDuplicateRelation(current.Relation), &thisID, nil
	}

	if current.RefSceneID != nil && other.RefSceneID != nil && *current.RefSceneID == *other.RefSceneID &&
		hasMemberSegment(current) && hasMemberSegment(other) {
		if seg := segmentThroughCommonRef(current, other); seg != nil {
			relation := classifySimilar(seg, current, other)
			if relation != data.DuplicateRelationUnknown {
				return relation, &thisID, seg
			}
			return data.DuplicateRelationOrUnknown(other.Relation), other.RefSceneID, seg
		}
	}
	return data.DuplicateRelationOrUnknown(other.Relation), other.RefSceneID, nil
}

// hasMemberSegment tells whether a member has a non-empty segment in itself and its reference.
func hasMemberSegment(m *data.DuplicateMemberScene) bool {
	return m.MatchStart != nil && m.MatchEnd != nil && m.RefStart != nil && m.RefEnd != nil &&
		*m.MatchEnd > *m.MatchStart && *m.RefEnd > *m.RefStart
}

// segmentThroughCommonRef maps the part of the common reference that both members match back
// to each of them (linearly, segments being time-aligned), or nil when they share no part of it.
func segmentThroughCommonRef(current, other *data.DuplicateMemberScene) *SimilarSegment {
	lo := max(*current.RefStart, *other.RefStart)
	hi := min(*current.RefEnd, *other.RefEnd)
	if hi <= lo {
		return nil
	}
	through := func(m *data.DuplicateMemberScene, t float64) float64 {
		scale := (*m.MatchEnd - *m.MatchStart) / (*m.RefEnd - *m.RefStart)
		return *m.MatchStart + (t-*m.RefStart)*scale
	}
	return &SimilarSegment{
		ThisStart: through(current, lo), ThisEnd: through(current, hi),
		OtherStart: through(other, lo), OtherEnd: through(other, hi),
	}
}

// classifySimilar derives the relation of other to current from their matching segment.
func classifySimilar(seg *SimilarSegment, current, other *data.DuplicateMemberScene) string {
	return ClassifyDuplicateRelation(DuplicateRelationInput{
		MatchStart: &seg.OtherStart, MatchEnd: &seg.OtherEnd,
		RefStart: &seg.ThisStart, RefEnd: &seg.ThisEnd,
		Duration: other.Duration, RefDuration: current.Duration,
	})
}

// refineRelation keeps a stored relation, deriving it from the segment when it is not set yet.
func refineRelation(relation string, seg *SimilarSegment, current, other *data.DuplicateMemberScene) string {
	relation = data.DuplicateRelationOrUnknown(relation)
	if relation != data.DuplicateRelationUnknown {
		return relation
	}
	return classifySimilar(seg, current, other)
}

// invertDuplicateRelation turns the relation of a scene to another into the relation of the
// other one to it (a clip of the other means the other contains it).
func invertDuplicateRelation(relation string) string {
	switch relation {
	case data.DuplicateRelationContained:
		return data.DuplicateRelationContains
	case data.DuplicateRelationContains:
		return data.DuplicateRelationContained
	default:
		return data.DuplicateRelationOrUnknown(relation)
	}
}
