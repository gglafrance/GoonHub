package core

import (
	"errors"
	"sort"
	"strings"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// TagMergeResult describes the merge of source tags into a target tag (done, or to do for a
// dry run).
type TagMergeResult struct {
	Target  data.Tag   `json:"target"`
	Sources []data.Tag `json:"sources"`
	DryRun  bool       `json:"dry_run"`
	data.TagMergeStats
}

// TagDuplicateGroup is a set of tags whose names are equal ignoring case and extra spaces.
type TagDuplicateGroup struct {
	// Key is the normalized name shared by the group.
	Key string `json:"key"`
	// SuggestedTargetID is the tag to keep: the one with the most scenes (oldest on a tie).
	SuggestedTargetID uint `json:"suggested_target_id"`
	// Tags of the group, suggested target first, then by scene count (desc) and id.
	Tags []data.TagWithCount `json:"tags"`
}

// NormalizeTagName folds a tag name for duplicate detection: lower case, surrounding spaces
// removed and inner runs of whitespace collapsed to one space.
func NormalizeTagName(name string) string {
	return strings.ToLower(strings.Join(strings.Fields(name), " "))
}

// GroupDuplicateTags returns the groups of at least two tags sharing a normalized name,
// sorted by key.
func GroupDuplicateTags(tags []data.TagWithCount) []TagDuplicateGroup {
	byKey := make(map[string][]data.TagWithCount)
	for _, t := range tags {
		key := NormalizeTagName(t.Name)
		if key == "" {
			continue
		}
		byKey[key] = append(byKey[key], t)
	}
	groups := make([]TagDuplicateGroup, 0)
	for key, members := range byKey {
		if len(members) < 2 {
			continue
		}
		sort.Slice(members, func(i, j int) bool {
			if members[i].SceneCount != members[j].SceneCount {
				return members[i].SceneCount > members[j].SceneCount
			}
			return members[i].ID < members[j].ID
		})
		groups = append(groups, TagDuplicateGroup{Key: key, SuggestedTargetID: members[0].ID, Tags: members})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Key < groups[j].Key })
	return groups
}

// FindDuplicates returns the tags whose names are equal ignoring case and extra spaces.
func (s *TagService) FindDuplicates() ([]TagDuplicateGroup, error) {
	tags, err := s.tagRepo.ListWithCounts()
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list tags", err)
	}
	return GroupDuplicateTags(tags), nil
}

// UpdateTagColor changes the color of a tag.
func (s *TagService) UpdateTagColor(id uint, color string) (*data.Tag, error) {
	if !colorRegex.MatchString(color) {
		return nil, apperrors.NewValidationErrorWithField("color", "invalid color format, must be a hex color like #6B7280")
	}
	tag, err := s.getTag(id)
	if err != nil {
		return nil, err
	}
	tag.Color = strings.ToUpper(color)
	if err := s.tagRepo.Update(tag); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrTagNotFound(id)
		}
		return nil, apperrors.NewInternalError("failed to update tag", err)
	}
	s.logger.Info("Tag color updated", zap.Uint("id", id), zap.String("color", tag.Color))
	return tag, nil
}

// MergeTags merges the source tags into the target tag: scenes, markers, marker labels,
// playlists, saved filters and homepage sections move to the target and the sources are
// deleted (irreversible). The scenes whose tags changed are re-indexed. With dryRun nothing
// changes and the result tells what the merge would do.
func (s *TagService) MergeTags(targetID uint, sourceIDs []uint, dryRun bool) (*TagMergeResult, error) {
	if targetID == 0 {
		return nil, apperrors.NewValidationErrorWithField("target_id", "target tag is required")
	}
	unique := make([]uint, 0, len(sourceIDs))
	seen := make(map[uint]bool, len(sourceIDs))
	for _, id := range sourceIDs {
		if id == 0 || seen[id] {
			continue
		}
		if id == targetID {
			return nil, apperrors.NewValidationErrorWithField("source_ids", "the target tag cannot be one of the sources")
		}
		seen[id] = true
		unique = append(unique, id)
	}
	if len(unique) == 0 {
		return nil, apperrors.NewValidationErrorWithField("source_ids", "at least one source tag is required")
	}

	target, err := s.getTag(targetID)
	if err != nil {
		return nil, err
	}
	sources, err := s.tagRepo.GetByIDs(unique)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to load source tags", err)
	}
	if len(sources) != len(unique) {
		found := make(map[uint]bool, len(sources))
		for _, t := range sources {
			found[t.ID] = true
		}
		for _, id := range unique {
			if !found[id] {
				return nil, apperrors.ErrTagNotFound(id)
			}
		}
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].ID < sources[j].ID })

	stats, sceneIDs, err := s.tagRepo.MergeTags(*target, sources, dryRun)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to merge tags", err)
	}
	if !dryRun {
		reindexScenesInBatches(s.sceneRepo, s.indexer, s.logger, sceneIDs)
		s.logger.Info("Tags merged",
			zap.Uint("target_id", target.ID),
			zap.Uints("source_ids", unique),
			zap.Int64("scenes_retagged", stats.ScenesRetagged),
			zap.Int64("markers_updated", stats.MarkersUpdated),
			zap.Int64("references_updated", stats.ReferencesUpdated),
		)
	}
	return &TagMergeResult{Target: *target, Sources: sources, DryRun: dryRun, TagMergeStats: *stats}, nil
}

func (s *TagService) getTag(id uint) (*data.Tag, error) {
	tag, err := s.tagRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrTagNotFound(id)
		}
		return nil, apperrors.NewInternalError("failed to find tag", err)
	}
	return tag, nil
}
