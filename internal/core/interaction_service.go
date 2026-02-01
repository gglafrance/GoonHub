package core

import (
	"fmt"
	"math"

	"goonhub/internal/data"

	"go.uber.org/zap"
)

type InteractionService struct {
	repo   data.InteractionRepository
	logger *zap.Logger
}

func NewInteractionService(repo data.InteractionRepository, logger *zap.Logger) *InteractionService {
	return &InteractionService{
		repo:   repo,
		logger: logger,
	}
}

func (s *InteractionService) SetRating(userID, sceneID uint, rating float64) error {
	if rating < 0.5 || rating > 5.0 {
		return fmt.Errorf("rating must be between 0.5 and 5.0")
	}

	// Validate 0.5 increments: multiply by 2 and check if it's a whole number
	doubled := rating * 2
	if math.Abs(doubled-math.Round(doubled)) > 0.001 {
		return fmt.Errorf("rating must be in 0.5 increments")
	}

	if err := s.repo.UpsertRating(userID, sceneID, rating); err != nil {
		s.logger.Error("failed to set rating", zap.Uint("userID", userID), zap.Uint("sceneID", sceneID), zap.Error(err))
		return fmt.Errorf("failed to set rating: %w", err)
	}

	return nil
}

func (s *InteractionService) ClearRating(userID, sceneID uint) error {
	if err := s.repo.DeleteRating(userID, sceneID); err != nil {
		s.logger.Error("failed to clear rating", zap.Uint("userID", userID), zap.Uint("sceneID", sceneID), zap.Error(err))
		return fmt.Errorf("failed to clear rating: %w", err)
	}
	return nil
}

func (s *InteractionService) GetRating(userID, sceneID uint) (float64, error) {
	record, err := s.repo.GetRating(userID, sceneID)
	if err != nil {
		if data.IsNotFound(err) {
			return 0, nil
		}
		s.logger.Error("failed to get rating", zap.Uint("userID", userID), zap.Uint("sceneID", sceneID), zap.Error(err))
		return 0, fmt.Errorf("failed to get rating: %w", err)
	}
	return record.Rating, nil
}

func (s *InteractionService) ToggleLike(userID, sceneID uint) (bool, error) {
	liked, err := s.repo.IsLiked(userID, sceneID)
	if err != nil {
		s.logger.Error("failed to check like status", zap.Uint("userID", userID), zap.Uint("sceneID", sceneID), zap.Error(err))
		return false, fmt.Errorf("failed to check like status: %w", err)
	}

	if liked {
		if err := s.repo.DeleteLike(userID, sceneID); err != nil {
			s.logger.Error("failed to unlike scene", zap.Uint("userID", userID), zap.Uint("sceneID", sceneID), zap.Error(err))
			return false, fmt.Errorf("failed to unlike scene: %w", err)
		}
		return false, nil
	}

	if err := s.repo.SetLike(userID, sceneID); err != nil {
		s.logger.Error("failed to like scene", zap.Uint("userID", userID), zap.Uint("sceneID", sceneID), zap.Error(err))
		return false, fmt.Errorf("failed to like scene: %w", err)
	}
	return true, nil
}

func (s *InteractionService) IsLiked(userID, sceneID uint) (bool, error) {
	liked, err := s.repo.IsLiked(userID, sceneID)
	if err != nil {
		s.logger.Error("failed to check like status", zap.Uint("userID", userID), zap.Uint("sceneID", sceneID), zap.Error(err))
		return false, fmt.Errorf("failed to check like status: %w", err)
	}
	return liked, nil
}

func (s *InteractionService) IncrementJizzed(userID, sceneID uint) (int, error) {
	count, err := s.repo.IncrementJizzed(userID, sceneID)
	if err != nil {
		s.logger.Error("failed to increment jizzed", zap.Uint("userID", userID), zap.Uint("sceneID", sceneID), zap.Error(err))
		return 0, fmt.Errorf("failed to increment jizzed: %w", err)
	}
	return count, nil
}

func (s *InteractionService) GetJizzedCount(userID, sceneID uint) (int, error) {
	count, err := s.repo.GetJizzedCount(userID, sceneID)
	if err != nil {
		s.logger.Error("failed to get jizzed count", zap.Uint("userID", userID), zap.Uint("sceneID", sceneID), zap.Error(err))
		return 0, fmt.Errorf("failed to get jizzed count: %w", err)
	}
	return count, nil
}

func (s *InteractionService) GetAllInteractions(userID, sceneID uint) (*data.SceneInteractions, error) {
	interactions, err := s.repo.GetAllInteractions(userID, sceneID)
	if err != nil {
		s.logger.Error("failed to get interactions", zap.Uint("userID", userID), zap.Uint("sceneID", sceneID), zap.Error(err))
		return nil, fmt.Errorf("failed to get interactions: %w", err)
	}
	return interactions, nil
}
