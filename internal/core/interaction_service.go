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

func (s *InteractionService) SetRating(userID, videoID uint, rating float64) error {
	if rating < 0.5 || rating > 5.0 {
		return fmt.Errorf("rating must be between 0.5 and 5.0")
	}

	// Validate 0.5 increments: multiply by 2 and check if it's a whole number
	doubled := rating * 2
	if math.Abs(doubled-math.Round(doubled)) > 0.001 {
		return fmt.Errorf("rating must be in 0.5 increments")
	}

	if err := s.repo.UpsertRating(userID, videoID, rating); err != nil {
		s.logger.Error("failed to set rating", zap.Uint("userID", userID), zap.Uint("videoID", videoID), zap.Error(err))
		return fmt.Errorf("failed to set rating: %w", err)
	}

	return nil
}

func (s *InteractionService) ClearRating(userID, videoID uint) error {
	if err := s.repo.DeleteRating(userID, videoID); err != nil {
		s.logger.Error("failed to clear rating", zap.Uint("userID", userID), zap.Uint("videoID", videoID), zap.Error(err))
		return fmt.Errorf("failed to clear rating: %w", err)
	}
	return nil
}

func (s *InteractionService) GetRating(userID, videoID uint) (float64, error) {
	record, err := s.repo.GetRating(userID, videoID)
	if err != nil {
		if data.IsNotFound(err) {
			return 0, nil
		}
		s.logger.Error("failed to get rating", zap.Uint("userID", userID), zap.Uint("videoID", videoID), zap.Error(err))
		return 0, fmt.Errorf("failed to get rating: %w", err)
	}
	return record.Rating, nil
}

func (s *InteractionService) ToggleLike(userID, videoID uint) (bool, error) {
	liked, err := s.repo.IsLiked(userID, videoID)
	if err != nil {
		s.logger.Error("failed to check like status", zap.Uint("userID", userID), zap.Uint("videoID", videoID), zap.Error(err))
		return false, fmt.Errorf("failed to check like status: %w", err)
	}

	if liked {
		if err := s.repo.DeleteLike(userID, videoID); err != nil {
			s.logger.Error("failed to unlike video", zap.Uint("userID", userID), zap.Uint("videoID", videoID), zap.Error(err))
			return false, fmt.Errorf("failed to unlike video: %w", err)
		}
		return false, nil
	}

	if err := s.repo.SetLike(userID, videoID); err != nil {
		s.logger.Error("failed to like video", zap.Uint("userID", userID), zap.Uint("videoID", videoID), zap.Error(err))
		return false, fmt.Errorf("failed to like video: %w", err)
	}
	return true, nil
}

func (s *InteractionService) IsLiked(userID, videoID uint) (bool, error) {
	liked, err := s.repo.IsLiked(userID, videoID)
	if err != nil {
		s.logger.Error("failed to check like status", zap.Uint("userID", userID), zap.Uint("videoID", videoID), zap.Error(err))
		return false, fmt.Errorf("failed to check like status: %w", err)
	}
	return liked, nil
}

func (s *InteractionService) IncrementJizzed(userID, videoID uint) (int, error) {
	count, err := s.repo.IncrementJizzed(userID, videoID)
	if err != nil {
		s.logger.Error("failed to increment jizzed", zap.Uint("userID", userID), zap.Uint("videoID", videoID), zap.Error(err))
		return 0, fmt.Errorf("failed to increment jizzed: %w", err)
	}
	return count, nil
}

func (s *InteractionService) GetJizzedCount(userID, videoID uint) (int, error) {
	count, err := s.repo.GetJizzedCount(userID, videoID)
	if err != nil {
		s.logger.Error("failed to get jizzed count", zap.Uint("userID", userID), zap.Uint("videoID", videoID), zap.Error(err))
		return 0, fmt.Errorf("failed to get jizzed count: %w", err)
	}
	return count, nil
}

func (s *InteractionService) GetAllInteractions(userID, videoID uint) (*data.VideoInteractions, error) {
	interactions, err := s.repo.GetAllInteractions(userID, videoID)
	if err != nil {
		s.logger.Error("failed to get interactions", zap.Uint("userID", userID), zap.Uint("videoID", videoID), zap.Error(err))
		return nil, fmt.Errorf("failed to get interactions: %w", err)
	}
	return interactions, nil
}
