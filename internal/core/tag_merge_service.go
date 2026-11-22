package core

import (
	"errors"
	"fmt"
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

// TagUpdate holds the fields of PUT /tags/:id; nil fields are left unchanged.
type TagUpdate struct {
	Name  *string
	Color *string
}

// UpdateTag changes the name and/or the color of a tag. A new name (trimmed) must not be
// used by another tag, exactly or ignoring case and extra spaces (an EntityConflictError
// carrying that tag, to merge into it instead); a different case of the tag's own name is
// allowed. A rename also renames the references to the old name (saved searches, saved
// marker filters, homepage widgets) and re-indexes the tag's scenes.
func (s *TagService) UpdateTag(id uint, upd TagUpdate) (*data.Tag, error) {
	if upd.Name == nil && upd.Color == nil {
		return nil, apperrors.NewValidationError("name or color is required")
	}
	var name string
	if upd.Name != nil {
		name = strings.TrimSpace(*upd.Name)
		if err := validateTagName(name); err != nil {
			return nil, err
		}
	}
	if upd.Color != nil && !colorRegex.MatchString(*upd.Color) {
		return nil, apperrors.NewValidationErrorWithField("color", "invalid color format, must be a hex color like #6B7280")
	}

	tag, err := s.getTag(id)
	if err != nil {
		return nil, err
	}
	oldName := tag.Name
	if upd.Color != nil {
		tag.Color = strings.ToUpper(*upd.Color)
	}

	if upd.Name == nil || name == oldName {
		if err := s.tagRepo.Update(tag); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, apperrors.ErrTagNotFound(id)
			}
			return nil, apperrors.NewInternalError("failed to update tag", err)
		}
		s.logger.Info("Tag updated", zap.Uint("id", id), zap.String("color", tag.Color))
		return tag, nil
	}

	if err := s.checkTagNameFree(id, name); err != nil {
		return nil, err
	}
	tag.Name = name
	refs, sceneIDs, err := s.tagRepo.RenameTag(*tag, oldName)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			return nil, apperrors.ErrTagNotFound(id)
		case errors.Is(err, data.ErrTagNameTaken):
			// created meanwhile: report the tag that has it when it can be found
			if conflict := s.checkTagNameFree(id, name); conflict != nil {
				return nil, conflict
			}
			return nil, apperrors.ErrTagAlreadyExists(name)
		}
		return nil, apperrors.NewInternalError("failed to rename tag", err)
	}
	reindexScenesInBatches(s.sceneRepo, s.indexer, s.logger, sceneIDs)
	s.logger.Info("Tag renamed",
		zap.Uint("id", id),
		zap.String("from", oldName),
		zap.String("to", name),
		zap.Int64("references_updated", refs),
		zap.Int("scenes_reindexed", len(sceneIDs)),
	)
	return tag, nil
}

// checkTagNameFree returns an EntityConflictError when a tag other than selfID has name,
// exactly or ignoring case and extra spaces (the duplicate detection key). An exact match is
// preferred as the reported tag.
func (s *TagService) checkTagNameFree(selfID uint, name string) error {
	tags, err := s.tagRepo.List()
	if err != nil {
		return apperrors.NewInternalError("failed to check tag name", err)
	}
	key := NormalizeTagName(name)
	var existing *data.Tag
	for i := range tags {
		t := &tags[i]
		if t.ID == selfID || NormalizeTagName(t.Name) != key {
			continue
		}
		if existing == nil || t.Name == name {
			existing = t
		}
	}
	if existing == nil {
		return nil
	}
	return newTagNameConflict(name, *existing)
}

// newTagNameConflict is the 409 of a rename to a name used by another tag: it suggests
// merging into that tag instead.
func newTagNameConflict(name string, existing data.Tag) *apperrors.EntityConflictError {
	msg := fmt.Sprintf("tag '%s' already exists: merge into it instead of renaming", existing.Name)
	if existing.Name != name {
		msg = fmt.Sprintf("'%s' matches the existing tag '%s' (ignoring case and spaces): merge into it instead of renaming", name, existing.Name)
	}
	return &apperrors.EntityConflictError{
		ConflictError: apperrors.NewConflictError("tag", msg),
		Field:         "name",
		Value:         name,
		Existing:      existing,
	}
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
