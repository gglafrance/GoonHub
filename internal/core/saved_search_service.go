package core

import (
	"errors"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type SavedSearchService struct {
	repo   data.SavedSearchRepository
	logger *zap.Logger
}

func NewSavedSearchService(repo data.SavedSearchRepository, logger *zap.Logger) *SavedSearchService {
	return &SavedSearchService{
		repo:   repo,
		logger: logger,
	}
}

type CreateSavedSearchInput struct {
	Name    string
	Filters data.Filters
}

type UpdateSavedSearchInput struct {
	Name    *string
	Filters *data.Filters
}

func (s *SavedSearchService) Create(userID uint, input CreateSavedSearchInput) (*data.SavedSearch, error) {
	if input.Name == "" {
		return nil, apperrors.ErrSavedSearchNameRequired
	}
	if len(input.Name) > 255 {
		return nil, apperrors.ErrSavedSearchNameTooLong
	}

	search := &data.SavedSearch{
		UserID:  userID,
		Name:    input.Name,
		Filters: input.Filters,
	}

	if err := s.repo.Create(search); err != nil {
		return nil, apperrors.NewInternalError("failed to create saved search", err)
	}

	s.logger.Info("Saved search created",
		zap.Uint("user_id", userID),
		zap.String("name", input.Name),
		zap.String("uuid", search.UUID.String()),
	)

	return search, nil
}

func (s *SavedSearchService) GetByUUID(userID uint, uuid string) (*data.SavedSearch, error) {
	search, err := s.repo.GetByUUID(uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrSavedSearchNotFound(uuid)
		}
		return nil, apperrors.NewInternalError("failed to find saved search", err)
	}

	if search.UserID != userID {
		return nil, apperrors.ErrSavedSearchForbidden
	}

	return search, nil
}

func (s *SavedSearchService) List(userID uint) ([]data.SavedSearch, error) {
	searches, err := s.repo.ListByUserID(userID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list saved searches", err)
	}
	return searches, nil
}

func (s *SavedSearchService) Update(userID uint, uuid string, input UpdateSavedSearchInput) (*data.SavedSearch, error) {
	search, err := s.repo.GetByUUID(uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrSavedSearchNotFound(uuid)
		}
		return nil, apperrors.NewInternalError("failed to find saved search", err)
	}

	if search.UserID != userID {
		return nil, apperrors.ErrSavedSearchForbidden
	}

	if input.Name != nil {
		if *input.Name == "" {
			return nil, apperrors.ErrSavedSearchNameRequired
		}
		if len(*input.Name) > 255 {
			return nil, apperrors.ErrSavedSearchNameTooLong
		}
		search.Name = *input.Name
	}

	if input.Filters != nil {
		search.Filters = *input.Filters
	}

	if err := s.repo.Update(search); err != nil {
		return nil, apperrors.NewInternalError("failed to update saved search", err)
	}

	s.logger.Info("Saved search updated",
		zap.Uint("user_id", userID),
		zap.String("uuid", uuid),
	)

	return search, nil
}

func (s *SavedSearchService) Delete(userID uint, uuid string) error {
	search, err := s.repo.GetByUUID(uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrSavedSearchNotFound(uuid)
		}
		return apperrors.NewInternalError("failed to find saved search", err)
	}

	if search.UserID != userID {
		return apperrors.ErrSavedSearchForbidden
	}

	if err := s.repo.Delete(search.ID); err != nil {
		return apperrors.NewInternalError("failed to delete saved search", err)
	}

	s.logger.Info("Saved search deleted",
		zap.Uint("user_id", userID),
		zap.String("uuid", uuid),
	)

	return nil
}
