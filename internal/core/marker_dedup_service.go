package core

import (
	"fmt"
	"os"
	"path/filepath"

	"goonhub/internal/data"

	"go.uber.org/zap"
)

// markerDedupMaxDetails caps the merges listed in a MarkerDedupResult (the counts cover all).
const markerDedupMaxDetails = 200

// MarkerDedupMarker is a marker of a duplicate set, as shown in a MarkerDedupResult.
type MarkerDedupMarker struct {
	ID           uint   `json:"id"`
	Timestamp    int    `json:"timestamp"`
	EndTimestamp *int   `json:"end_timestamp,omitempty"`
	Label        string `json:"label"`
}

// MarkerDedupMerge is a set of markers of a user folded into one.
type MarkerDedupMerge struct {
	SceneID uint              `json:"scene_id"`
	UserID  uint              `json:"user_id"`
	Kept    MarkerDedupMarker `json:"kept"`
	// Label is the kept marker's label after the merge (taken from another one when it had none)
	Label   string              `json:"label"`
	Removed []MarkerDedupMarker `json:"removed"`
	// AddedTagIDs are the tags of the removed markers the kept one gets
	AddedTagIDs []uint `json:"added_tag_ids"`
}

// MarkerDedupResult is the outcome of DeduplicateMarkers.
type MarkerDedupResult struct {
	// DryRun: nothing was changed, the result tells what would be
	DryRun bool `json:"dry_run"`
	// ScenesScanned is how many scenes were looked at
	ScenesScanned int `json:"scenes_scanned"`
	// Scenes is how many scenes have duplicate markers
	Scenes int `json:"scenes"`
	// Groups is how many sets of duplicates were (or would be) folded
	Groups int `json:"groups"`
	// Removed is how many markers were (or would be) deleted
	Removed int `json:"removed"`
	// Merges details the sets, at most markerDedupMaxDetails of them (Truncated then)
	Merges    []MarkerDedupMerge `json:"merges"`
	Truncated bool               `json:"truncated"`
}

// DeduplicateMarkers folds, scene by scene and per user, the markers that are the same marker
// (same label, or no label, and starts at most markerDuplicateToleranceSeconds apart; see
// planMarkerDedup): one is kept with the tags of all of them, the others are deleted with their
// thumbnails. With dryRun nothing changes and the result tells what would. A nil sceneIDs
// means every scene that has candidates.
func (s *MarkerService) DeduplicateMarkers(sceneIDs []uint, dryRun bool) (*MarkerDedupResult, error) {
	if sceneIDs == nil {
		ids, err := s.markerRepo.ListSceneIDsWithNearbyMarkers(markerDuplicateToleranceSeconds)
		if err != nil {
			return nil, fmt.Errorf("failed to list scenes with duplicate markers: %w", err)
		}
		sceneIDs = ids
	}
	result := &MarkerDedupResult{DryRun: dryRun, Merges: []MarkerDedupMerge{}}
	for _, sceneID := range sceneIDs {
		result.ScenesScanned++
		groups, err := s.planSceneMarkerDedup(sceneID)
		if err != nil {
			return result, err
		}
		if len(groups) == 0 {
			continue
		}
		result.Scenes++
		merges := make([]data.MarkerDedupMerge, 0, len(groups))
		var removed []data.UserSceneMarker
		for _, g := range groups {
			merge := data.MarkerDedupMerge{KeepID: g.keep.ID, AddTags: g.addTags}
			if g.label != g.keep.Label {
				label := g.label
				merge.Label = &label
			}
			detail := MarkerDedupMerge{
				SceneID: sceneID, UserID: g.keep.UserID, Kept: dedupMarker(g.keep), Label: g.label,
				Removed: make([]MarkerDedupMarker, 0, len(g.remove)), AddedTagIDs: make([]uint, 0, len(g.addTags)),
			}
			for _, m := range g.remove {
				merge.RemoveIDs = append(merge.RemoveIDs, m.ID)
				detail.Removed = append(detail.Removed, dedupMarker(m))
				removed = append(removed, m.UserSceneMarker)
			}
			for _, t := range g.addTags {
				detail.AddedTagIDs = append(detail.AddedTagIDs, t.TagID)
			}
			merges = append(merges, merge)
			result.Groups++
			result.Removed += len(g.remove)
			if len(result.Merges) < markerDedupMaxDetails {
				result.Merges = append(result.Merges, detail)
			} else {
				result.Truncated = true
			}
		}
		if dryRun {
			continue
		}
		if err := s.markerRepo.ApplyMarkerDedup(merges); err != nil {
			return result, fmt.Errorf("failed to fold duplicate markers of scene %d: %w", sceneID, err)
		}
		for i := range removed {
			s.removeMarkerThumbnailFiles(&removed[i])
		}
		s.logger.Info("Folded duplicate markers",
			zap.Uint("scene_id", sceneID),
			zap.Int("groups", len(merges)),
			zap.Int("removed", len(removed)),
		)
	}
	return result, nil
}

// planSceneMarkerDedup loads the markers of a scene (every user) with their tags and plans
// their deduplication.
func (s *MarkerService) planSceneMarkerDedup(sceneID uint) ([]markerDedupGroup, error) {
	markers, err := s.markerRepo.GetAllByScene(sceneID)
	if err != nil {
		return nil, fmt.Errorf("failed to get markers of scene %d: %w", sceneID, err)
	}
	if len(markers) < 2 {
		return nil, nil
	}
	ids := make([]uint, len(markers))
	for i, m := range markers {
		ids[i] = m.ID
	}
	tags, err := s.markerRepo.GetMarkerTagsMultiple(ids)
	if err != nil {
		return nil, fmt.Errorf("failed to get marker tags of scene %d: %w", sceneID, err)
	}
	withTags := make([]data.MarkerWithTags, len(markers))
	for i, m := range markers {
		withTags[i] = data.MarkerWithTags{UserSceneMarker: m, Tags: tags[m.ID]}
	}
	return planMarkerDedup(withTags), nil
}

func dedupMarker(m data.MarkerWithTags) MarkerDedupMarker {
	return MarkerDedupMarker{ID: m.ID, Timestamp: m.Timestamp, EndTimestamp: m.EndTimestamp, Label: m.Label}
}

// QueueMarkerThumbnails has new thumbnails made for markers whose time changed (moved to the
// kept scene of a duplicate group): the old files, made at the old time, are removed first.
// Markers deleted meanwhile are skipped.
func (s *MarkerService) QueueMarkerThumbnails(markerIDs []uint) {
	for _, id := range markerIDs {
		marker, err := s.markerRepo.GetByID(id)
		if err != nil || marker == nil {
			continue
		}
		scene, err := s.sceneRepo.GetByID(marker.SceneID)
		if err != nil {
			s.logger.Warn("failed to get scene for marker thumbnail", zap.Uint("markerID", id), zap.Error(err))
			continue
		}
		s.removeMarkerThumbnailFiles(marker)
		s.queueThumbnail(marker, scene)
	}
}

// removeMarkerThumbnailFiles deletes the thumbnail files of a marker (best effort). They are
// GoonHub's generated files, in the marker thumbnail directory, never media.
func (s *MarkerService) removeMarkerThumbnailFiles(marker *data.UserSceneMarker) {
	for _, name := range []string{marker.ThumbnailPath, marker.AnimatedThumbnailPath} {
		if name == "" {
			continue
		}
		path := filepath.Join(s.markerThumbnailDir, name)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			s.logger.Warn("failed to delete marker thumbnail",
				zap.Uint("markerID", marker.ID),
				zap.String("path", path),
				zap.Error(err))
		}
	}
}
