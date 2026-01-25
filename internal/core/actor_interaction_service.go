package core

import (
	"fmt"
	"math"

	"goonhub/internal/data"

	"go.uber.org/zap"
)

type ActorInteractionService struct {
	repo   data.ActorInteractionRepository
	logger *zap.Logger
}

func NewActorInteractionService(repo data.ActorInteractionRepository, logger *zap.Logger) *ActorInteractionService {
	return &ActorInteractionService{
		repo:   repo,
		logger: logger,
	}
}

func (s *ActorInteractionService) SetRating(userID, actorID uint, rating float64) error {
	if rating < 0.5 || rating > 5.0 {
		return fmt.Errorf("rating must be between 0.5 and 5.0")
	}

	// Validate 0.5 increments: multiply by 2 and check if it's a whole number
	doubled := rating * 2
	if math.Abs(doubled-math.Round(doubled)) > 0.001 {
		return fmt.Errorf("rating must be in 0.5 increments")
	}

	if err := s.repo.UpsertRating(userID, actorID, rating); err != nil {
		s.logger.Error("failed to set actor rating", zap.Uint("userID", userID), zap.Uint("actorID", actorID), zap.Error(err))
		return fmt.Errorf("failed to set rating: %w", err)
	}

	return nil
}

func (s *ActorInteractionService) ClearRating(userID, actorID uint) error {
	if err := s.repo.DeleteRating(userID, actorID); err != nil {
		s.logger.Error("failed to clear actor rating", zap.Uint("userID", userID), zap.Uint("actorID", actorID), zap.Error(err))
		return fmt.Errorf("failed to clear rating: %w", err)
	}
	return nil
}

func (s *ActorInteractionService) GetRating(userID, actorID uint) (float64, error) {
	record, err := s.repo.GetRating(userID, actorID)
	if err != nil {
		if data.IsNotFound(err) {
			return 0, nil
		}
		s.logger.Error("failed to get actor rating", zap.Uint("userID", userID), zap.Uint("actorID", actorID), zap.Error(err))
		return 0, fmt.Errorf("failed to get rating: %w", err)
	}
	return record.Rating, nil
}

func (s *ActorInteractionService) ToggleLike(userID, actorID uint) (bool, error) {
	liked, err := s.repo.IsLiked(userID, actorID)
	if err != nil {
		s.logger.Error("failed to check actor like status", zap.Uint("userID", userID), zap.Uint("actorID", actorID), zap.Error(err))
		return false, fmt.Errorf("failed to check like status: %w", err)
	}

	if liked {
		if err := s.repo.DeleteLike(userID, actorID); err != nil {
			s.logger.Error("failed to unlike actor", zap.Uint("userID", userID), zap.Uint("actorID", actorID), zap.Error(err))
			return false, fmt.Errorf("failed to unlike actor: %w", err)
		}
		return false, nil
	}

	if err := s.repo.SetLike(userID, actorID); err != nil {
		s.logger.Error("failed to like actor", zap.Uint("userID", userID), zap.Uint("actorID", actorID), zap.Error(err))
		return false, fmt.Errorf("failed to like actor: %w", err)
	}
	return true, nil
}

func (s *ActorInteractionService) IsLiked(userID, actorID uint) (bool, error) {
	liked, err := s.repo.IsLiked(userID, actorID)
	if err != nil {
		s.logger.Error("failed to check actor like status", zap.Uint("userID", userID), zap.Uint("actorID", actorID), zap.Error(err))
		return false, fmt.Errorf("failed to check like status: %w", err)
	}
	return liked, nil
}

func (s *ActorInteractionService) GetAllInteractions(userID, actorID uint) (*data.ActorInteractions, error) {
	interactions, err := s.repo.GetAllInteractions(userID, actorID)
	if err != nil {
		s.logger.Error("failed to get actor interactions", zap.Uint("userID", userID), zap.Uint("actorID", actorID), zap.Error(err))
		return nil, fmt.Errorf("failed to get interactions: %w", err)
	}
	return interactions, nil
}
