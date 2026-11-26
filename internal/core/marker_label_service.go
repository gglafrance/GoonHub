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

// maxMarkerLabelNameLength is the length limit of a marker label name (bytes,
// marker_labels.name is VARCHAR(100)).
const maxMarkerLabelNameLength = 100

// maxMarkerLabelDescriptionLength bounds the description of a label.
const maxMarkerLabelDescriptionLength = 500

// defaultMarkerLabelColor is the color of a label created without one.
const defaultMarkerLabelColor = "#FFFFFF"

// MarkerLabelService manages the server-wide marker labels (docs/MARKER_LABELS.md): the
// referential and the default tags of each label. Every write needs marker_labels:manage
// (checked by the routes).
type MarkerLabelService struct {
	repo    data.MarkerLabelRepository
	tagRepo data.TagRepository
	logger  *zap.Logger
}

func NewMarkerLabelService(repo data.MarkerLabelRepository, tagRepo data.TagRepository, logger *zap.Logger) *MarkerLabelService {
	return &MarkerLabelService{repo: repo, tagRepo: tagRepo, logger: logger}
}

// MarkerLabelInput holds the fields of a label creation or update; nil fields are left
// unchanged on update (and defaulted on creation).
type MarkerLabelInput struct {
	Name        *string
	Color       *string
	Description *string
}

// MarkerLabelMergeResult describes the merge of source labels into a target label (done, or
// to do for a dry run).
type MarkerLabelMergeResult struct {
	Target  data.MarkerLabel   `json:"target"`
	Sources []data.MarkerLabel `json:"sources"`
	DryRun  bool               `json:"dry_run"`
	data.MarkerLabelMergeStats
}

// MarkerLabelDeleteResult tells what a deletion did.
type MarkerLabelDeleteResult struct {
	Deleted int `json:"deleted"`
	// MarkersCleared are the markers that lost their label (they stay, without label).
	MarkersCleared int64 `json:"markers_cleared"`
}

// validateMarkerLabelName trims and checks a label name.
func validateMarkerLabelName(name string) (string, error) {
	name = data.NormalizeMarkerLabelName(name)
	if name == "" {
		return "", apperrors.NewValidationErrorWithField("name", "label name is required")
	}
	if len(name) > maxMarkerLabelNameLength {
		return "", apperrors.NewValidationErrorWithField("name", "label name must be 100 characters or less")
	}
	return name, nil
}

func validateMarkerLabelColor(color string) (string, error) {
	if !colorRegex.MatchString(color) {
		return "", apperrors.NewValidationErrorWithField("color", "invalid color format, must be a hex color like #FF4D4D")
	}
	return strings.ToUpper(color), nil
}

func validateMarkerLabelDescription(description string) (string, error) {
	description = strings.TrimSpace(description)
	if len(description) > maxMarkerLabelDescriptionLength {
		return "", apperrors.NewValidationErrorWithField("description", "description must be 500 characters or less")
	}
	return description, nil
}

// List returns every label with its usage (all users) and its default tags.
func (s *MarkerLabelService) List() ([]data.MarkerLabelWithUsage, error) {
	labels, err := s.repo.ListWithUsage()
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list marker labels", err)
	}
	if labels == nil {
		labels = []data.MarkerLabelWithUsage{}
	}
	return labels, nil
}

// Create adds a label. A name used by another label (ignoring case) is a 409 carrying it.
func (s *MarkerLabelService) Create(in MarkerLabelInput) (*data.MarkerLabel, error) {
	if in.Name == nil {
		return nil, apperrors.NewValidationErrorWithField("name", "label name is required")
	}
	name, err := validateMarkerLabelName(*in.Name)
	if err != nil {
		return nil, err
	}
	label := &data.MarkerLabel{Name: name, Color: defaultMarkerLabelColor}
	if in.Color != nil && *in.Color != "" {
		if label.Color, err = validateMarkerLabelColor(*in.Color); err != nil {
			return nil, err
		}
	}
	if in.Description != nil {
		if label.Description, err = validateMarkerLabelDescription(*in.Description); err != nil {
			return nil, err
		}
	}
	if err := s.checkNameFree(0, name); err != nil {
		return nil, err
	}
	if err := s.repo.Create(label); err != nil {
		if errors.Is(err, data.ErrMarkerLabelNameTaken) {
			if conflict := s.checkNameFree(0, name); conflict != nil {
				return nil, conflict
			}
			return nil, apperrors.NewConflictError("marker label", fmt.Sprintf("label '%s' already exists", name))
		}
		return nil, apperrors.NewInternalError("failed to create marker label", err)
	}
	s.logger.Info("Marker label created", zap.Uint("id", label.ID), zap.String("name", name))
	return label, nil
}

// Update renames, recolors or describes a label. A new name used by another label (ignoring
// case) is a 409 carrying that label, to merge into it instead; a different case of the
// label's own name is allowed. A rename renames the name on the markers and in the saved
// filters and homepage widgets.
func (s *MarkerLabelService) Update(id uint, in MarkerLabelInput) (*data.MarkerLabel, error) {
	if in.Name == nil && in.Color == nil && in.Description == nil {
		return nil, apperrors.NewValidationError("name, color or description is required")
	}
	label, err := s.get(id)
	if err != nil {
		return nil, err
	}
	oldName := label.Name
	if in.Name != nil {
		name, err := validateMarkerLabelName(*in.Name)
		if err != nil {
			return nil, err
		}
		if name != oldName {
			if err := s.checkNameFree(id, name); err != nil {
				return nil, err
			}
		}
		label.Name = name
	}
	if in.Color != nil {
		if label.Color, err = validateMarkerLabelColor(*in.Color); err != nil {
			return nil, err
		}
	}
	if in.Description != nil {
		if label.Description, err = validateMarkerLabelDescription(*in.Description); err != nil {
			return nil, err
		}
	}
	refs, err := s.repo.Update(label, oldName)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			return nil, apperrors.NewNotFoundError("marker label", id)
		case errors.Is(err, data.ErrMarkerLabelNameTaken):
			if conflict := s.checkNameFree(id, label.Name); conflict != nil {
				return nil, conflict
			}
			return nil, apperrors.NewConflictError("marker label", fmt.Sprintf("label '%s' already exists", label.Name))
		}
		return nil, apperrors.NewInternalError("failed to update marker label", err)
	}
	if oldName != label.Name {
		s.logger.Info("Marker label renamed", zap.Uint("id", id), zap.String("from", oldName),
			zap.String("to", label.Name), zap.Int64("references_updated", refs))
	}
	return label, nil
}

// Delete deletes labels (unknown IDs are a 404). Their markers stay, without label.
func (s *MarkerLabelService) Delete(ids []uint) (*MarkerLabelDeleteResult, error) {
	unique := uniqueLabelIDs(ids)
	if len(unique) == 0 {
		return nil, apperrors.NewValidationErrorWithField("ids", "at least one label is required")
	}
	if _, err := s.getAll(unique); err != nil {
		return nil, err
	}
	cleared, err := s.repo.Delete(unique)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to delete marker labels", err)
	}
	s.logger.Info("Marker labels deleted", zap.Uints("ids", unique), zap.Int64("markers_cleared", cleared))
	return &MarkerLabelDeleteResult{Deleted: len(unique), MarkersCleared: cleared}, nil
}

// Merge merges source labels into a target: their markers get the target, the target gets
// their default tags (union), the references to their names name the target, and they are
// deleted (irreversible). With dryRun nothing changes and the result tells what it would do.
func (s *MarkerLabelService) Merge(targetID uint, sourceIDs []uint, dryRun bool) (*MarkerLabelMergeResult, error) {
	if targetID == 0 {
		return nil, apperrors.NewValidationErrorWithField("target_id", "target label is required")
	}
	unique := uniqueLabelIDs(sourceIDs)
	for _, id := range unique {
		if id == targetID {
			return nil, apperrors.NewValidationErrorWithField("source_ids", "the target label cannot be one of the sources")
		}
	}
	if len(unique) == 0 {
		return nil, apperrors.NewValidationErrorWithField("source_ids", "at least one source label is required")
	}
	target, err := s.get(targetID)
	if err != nil {
		return nil, err
	}
	sources, err := s.getAll(unique)
	if err != nil {
		return nil, err
	}
	stats, err := s.repo.Merge(*target, sources, dryRun)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to merge marker labels", err)
	}
	if !dryRun {
		s.logger.Info("Marker labels merged", zap.Uint("target_id", targetID), zap.Uints("source_ids", unique),
			zap.Int64("markers_moved", stats.MarkersMoved))
	}
	return &MarkerLabelMergeResult{Target: *target, Sources: sources, DryRun: dryRun, MarkerLabelMergeStats: *stats}, nil
}

// FindByName returns the label of that name (ignoring case), NotFound when there is none.
func (s *MarkerLabelService) FindByName(name string) (*data.MarkerLabel, error) {
	label, err := s.repo.FindByName(name)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to find marker label", err)
	}
	if label == nil {
		return nil, apperrors.NewNotFoundError("marker label", strings.TrimSpace(name))
	}
	return label, nil
}

// GetDefaultTags returns the default tags of a label.
func (s *MarkerLabelService) GetDefaultTags(labelID uint) ([]data.Tag, error) {
	if _, err := s.get(labelID); err != nil {
		return nil, err
	}
	tags, err := s.repo.GetDefaultTags(labelID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get label tags", err)
	}
	if tags == nil {
		tags = []data.Tag{}
	}
	return tags, nil
}

// SetDefaultTags replaces the default tags of a label; every marker of the label (all users)
// gets them as tags from label, and loses the previous ones it had from the label.
func (s *MarkerLabelService) SetDefaultTags(labelID uint, tagIDs []uint) ([]data.Tag, error) {
	if _, err := s.get(labelID); err != nil {
		return nil, err
	}
	unique := uniqueLabelIDs(tagIDs)
	if len(unique) > 0 {
		tags, err := s.tagRepo.GetByIDs(unique)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to validate tags", err)
		}
		if len(tags) != len(unique) {
			return nil, apperrors.NewValidationError("one or more tags do not exist")
		}
	}
	if err := s.repo.SetDefaultTags(labelID, unique); err != nil {
		return nil, apperrors.NewInternalError("failed to set label tags", err)
	}
	s.logger.Info("Marker label default tags set", zap.Uint("label_id", labelID), zap.Int("tags", len(unique)))
	return s.GetDefaultTags(labelID)
}

// checkNameFree returns an EntityConflictError when a label other than selfID has name,
// ignoring case and surrounding spaces.
func (s *MarkerLabelService) checkNameFree(selfID uint, name string) error {
	existing, err := s.repo.FindByName(name)
	if err != nil {
		return apperrors.NewInternalError("failed to check label name", err)
	}
	if existing == nil || existing.ID == selfID {
		return nil
	}
	msg := fmt.Sprintf("label '%s' already exists: merge into it instead", existing.Name)
	if existing.Name != name {
		msg = fmt.Sprintf("'%s' matches the existing label '%s' (ignoring case): merge into it instead", name, existing.Name)
	}
	return &apperrors.EntityConflictError{
		ConflictError: apperrors.NewConflictError("marker label", msg),
		Field:         "name",
		Value:         name,
		Existing:      *existing,
	}
}

func (s *MarkerLabelService) get(id uint) (*data.MarkerLabel, error) {
	label, err := s.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.NewNotFoundError("marker label", id)
		}
		return nil, apperrors.NewInternalError("failed to find marker label", err)
	}
	return label, nil
}

// getAll loads labels by ID (sorted by ID); a missing one is a 404.
func (s *MarkerLabelService) getAll(ids []uint) ([]data.MarkerLabel, error) {
	labels, err := s.repo.GetByIDs(ids)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to load marker labels", err)
	}
	if len(labels) != len(ids) {
		found := make(map[uint]bool, len(labels))
		for _, l := range labels {
			found[l.ID] = true
		}
		for _, id := range ids {
			if !found[id] {
				return nil, apperrors.NewNotFoundError("marker label", id)
			}
		}
	}
	sort.Slice(labels, func(i, j int) bool { return labels[i].ID < labels[j].ID })
	return labels, nil
}

// uniqueLabelIDs drops zero and repeated IDs, keeping the order.
func uniqueLabelIDs(ids []uint) []uint {
	out := make([]uint, 0, len(ids))
	seen := make(map[uint]bool, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
