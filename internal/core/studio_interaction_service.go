package core

import (
	"fmt"
	"math"

	"goonhub/internal/data"

	"go.uber.org/zap"
)

type StudioInteractionService struct {
	repo   data.StudioInteractionRepository
	logger *zap.Logger
}

func NewStudioInteractionService(repo data.StudioInteractionRepository, logger *zap.Logger) *StudioInteractionService {
	return &StudioInteractionService{
		repo:   repo,
		logger: logger,
	}
}

func (s *StudioInteractionService) SetRating(userID, studioID uint, rating float64) error {
	if rating < 0.5 || rating > 5.0 {
		return fmt.Errorf("rating must be between 0.5 and 5.0")
	}

	// Validate 0.5 increments: multiply by 2 and check if it's a whole number
	doubled := rating * 2
	if math.Abs(doubled-math.Round(doubled)) > 0.001 {
		return fmt.Errorf("rating must be in 0.5 increments")
	}

	if err := s.repo.UpsertRating(userID, studioID, rating); err != nil {
		s.logger.Error("failed to set studio rating", zap.Uint("userID", userID), zap.Uint("studioID", studioID), zap.Error(err))
		return fmt.Errorf("failed to set rating: %w", err)
	}

	return nil
}

func (s *StudioInteractionService) ClearRating(userID, studioID uint) error {
	if err := s.repo.DeleteRating(userID, studioID); err != nil {
		s.logger.Error("failed to clear studio rating", zap.Uint("userID", userID), zap.Uint("studioID", studioID), zap.Error(err))
		return fmt.Errorf("failed to clear rating: %w", err)
	}
	return nil
}

func (s *StudioInteractionService) GetRating(userID, studioID uint) (float64, error) {
	record, err := s.repo.GetRating(userID, studioID)
	if err != nil {
		if data.IsNotFound(err) {
			return 0, nil
		}
		s.logger.Error("failed to get studio rating", zap.Uint("userID", userID), zap.Uint("studioID", studioID), zap.Error(err))
		return 0, fmt.Errorf("failed to get rating: %w", err)
	}
	return record.Rating, nil
}

func (s *StudioInteractionService) ToggleLike(userID, studioID uint) (bool, error) {
	liked, err := s.repo.IsLiked(userID, studioID)
	if err != nil {
		s.logger.Error("failed to check studio like status", zap.Uint("userID", userID), zap.Uint("studioID", studioID), zap.Error(err))
		return false, fmt.Errorf("failed to check like status: %w", err)
	}

	if liked {
		if err := s.repo.DeleteLike(userID, studioID); err != nil {
			s.logger.Error("failed to unlike studio", zap.Uint("userID", userID), zap.Uint("studioID", studioID), zap.Error(err))
			return false, fmt.Errorf("failed to unlike studio: %w", err)
		}
		return false, nil
	}

	if err := s.repo.SetLike(userID, studioID); err != nil {
		s.logger.Error("failed to like studio", zap.Uint("userID", userID), zap.Uint("studioID", studioID), zap.Error(err))
		return false, fmt.Errorf("failed to like studio: %w", err)
	}
	return true, nil
}

func (s *StudioInteractionService) IsLiked(userID, studioID uint) (bool, error) {
	liked, err := s.repo.IsLiked(userID, studioID)
	if err != nil {
		s.logger.Error("failed to check studio like status", zap.Uint("userID", userID), zap.Uint("studioID", studioID), zap.Error(err))
		return false, fmt.Errorf("failed to check like status: %w", err)
	}
	return liked, nil
}

func (s *StudioInteractionService) GetAllInteractions(userID, studioID uint) (*data.StudioInteractions, error) {
	interactions, err := s.repo.GetAllInteractions(userID, studioID)
	if err != nil {
		s.logger.Error("failed to get studio interactions", zap.Uint("userID", userID), zap.Uint("studioID", studioID), zap.Error(err))
		return nil, fmt.Errorf("failed to get interactions: %w", err)
	}
	return interactions, nil
}
