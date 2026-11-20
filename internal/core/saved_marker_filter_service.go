package core

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	savedMarkerFilterMaxName = 100
	savedMarkerFilterMaxList = 50
)

var savedMarkerFilterSorts = map[string]bool{
	"": true, "label_asc": true, "label_desc": true, "count_desc": true, "count_asc": true,
	"recent": true, "oldest": true, "random": true,
}

// SavedMarkerFilterService manages the saved filters of the marker pages.
type SavedMarkerFilterService struct {
	repo   data.SavedMarkerFilterRepository
	logger *zap.Logger
}

func NewSavedMarkerFilterService(repo data.SavedMarkerFilterRepository, logger *zap.Logger) *SavedMarkerFilterService {
	return &SavedMarkerFilterService{repo: repo, logger: logger}
}

func cleanStringList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// normalizeMarkerFilterValues trims and validates the stored filters.
func normalizeMarkerFilterValues(v data.MarkerFilterValues) (data.MarkerFilterValues, error) {
	v.Labels = cleanStringList(v.Labels)
	v.MarkerTags = cleanStringList(v.MarkerTags)
	v.Tags = cleanStringList(v.Tags)
	v.Actors = cleanStringList(v.Actors)
	v.Studio = strings.TrimSpace(v.Studio)
	for name, list := range map[string][]string{"labels": v.Labels, "marker_tags": v.MarkerTags, "tags": v.Tags, "actors": v.Actors} {
		if len(list) > savedMarkerFilterMaxList {
			return v, apperrors.NewValidationError(fmt.Sprintf("%s: at most %d values", name, savedMarkerFilterMaxList))
		}
	}
	if !savedMarkerFilterSorts[v.Sort] {
		return v, apperrors.NewValidationError(fmt.Sprintf("invalid sort: %s", v.Sort))
	}
	if v.View != "" && v.View != "grouped" && v.View != "all" {
		return v, apperrors.NewValidationError("view must be grouped or all")
	}
	return v, nil
}

func validateMarkerFilterName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", apperrors.NewValidationError("name is required")
	}
	if utf8.RuneCountInString(name) > savedMarkerFilterMaxName {
		return "", apperrors.NewValidationError(fmt.Sprintf("name must be at most %d characters", savedMarkerFilterMaxName))
	}
	return name, nil
}

// List returns the user's saved marker filters.
func (s *SavedMarkerFilterService) List(userID uint) ([]data.SavedMarkerFilter, error) {
	filters, err := s.repo.ListByUserID(userID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list saved marker filters", err)
	}
	if filters == nil {
		filters = []data.SavedMarkerFilter{}
	}
	return filters, nil
}

// Create saves a new marker filter for the user.
func (s *SavedMarkerFilterService) Create(userID uint, name string, values data.MarkerFilterValues) (*data.SavedMarkerFilter, error) {
	name, err := validateMarkerFilterName(name)
	if err != nil {
		return nil, err
	}
	values, err = normalizeMarkerFilterValues(values)
	if err != nil {
		return nil, err
	}
	filter := &data.SavedMarkerFilter{UserID: userID, Name: name, Filters: values}
	if err := s.repo.Create(filter); err != nil {
		return nil, apperrors.NewInternalError("failed to create saved marker filter", err)
	}
	s.logger.Info("Saved marker filter created", zap.Uint("user_id", userID), zap.String("name", name))
	return filter, nil
}

func (s *SavedMarkerFilterService) getOwned(userID uint, id string) (*data.SavedMarkerFilter, error) {
	filter, err := s.repo.GetByUUID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.NewNotFoundError("saved marker filter", id)
		}
		return nil, apperrors.NewInternalError("failed to get saved marker filter", err)
	}
	if filter.UserID != userID {
		return nil, apperrors.NewNotFoundError("saved marker filter", id)
	}
	return filter, nil
}

// Update renames and/or replaces the filters of a saved marker filter.
func (s *SavedMarkerFilterService) Update(userID uint, id string, name *string, values *data.MarkerFilterValues) (*data.SavedMarkerFilter, error) {
	filter, err := s.getOwned(userID, id)
	if err != nil {
		return nil, err
	}
	if name != nil {
		if filter.Name, err = validateMarkerFilterName(*name); err != nil {
			return nil, err
		}
	}
	if values != nil {
		if filter.Filters, err = normalizeMarkerFilterValues(*values); err != nil {
			return nil, err
		}
	}
	if err := s.repo.Update(filter); err != nil {
		return nil, apperrors.NewInternalError("failed to update saved marker filter", err)
	}
	return filter, nil
}

// Delete removes a saved marker filter.
func (s *SavedMarkerFilterService) Delete(userID uint, id string) error {
	filter, err := s.getOwned(userID, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(filter.ID); err != nil {
		return apperrors.NewInternalError("failed to delete saved marker filter", err)
	}
	return nil
}
