package core

import (
	"fmt"
	"testing"

	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func newTestInteractionService(t *testing.T) (*InteractionService, *mocks.MockInteractionRepository) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockInteractionRepository(ctrl)
	logger := zap.NewNop()
	service := NewInteractionService(repo, logger)
	return service, repo
}

func TestSetRating_ValidWholeAndHalf(t *testing.T) {
	validRatings := []float64{0.5, 1.0, 1.5, 2.0, 2.5, 3.0, 3.5, 4.0, 4.5, 5.0}

	for _, rating := range validRatings {
		t.Run(fmt.Sprintf("rating_%.1f", rating), func(t *testing.T) {
			service, repo := newTestInteractionService(t)
			repo.EXPECT().UpsertRating(uint(1), uint(10), rating).Return(nil)

			err := service.SetRating(1, 10, rating)
			if err != nil {
				t.Fatalf("expected no error for valid rating %.1f, got: %v", rating, err)
			}
		})
	}
}

func TestSetRating_InvalidRange(t *testing.T) {
	invalidRatings := []float64{0, 0.0, -1.0, 5.5, 6.0, 10.0}

	for _, rating := range invalidRatings {
		t.Run(fmt.Sprintf("rating_%.1f", rating), func(t *testing.T) {
			service, _ := newTestInteractionService(t)

			err := service.SetRating(1, 10, rating)
			if err == nil {
				t.Fatalf("expected error for invalid rating %.1f, got nil", rating)
			}
		})
	}
}

func TestSetRating_InvalidStep(t *testing.T) {
	invalidSteps := []float64{0.1, 0.3, 0.7, 1.3, 2.7, 3.2, 4.9}

	for _, rating := range invalidSteps {
		t.Run(fmt.Sprintf("rating_%.1f", rating), func(t *testing.T) {
			service, _ := newTestInteractionService(t)

			err := service.SetRating(1, 10, rating)
			if err == nil {
				t.Fatalf("expected error for invalid step rating %.1f, got nil", rating)
			}
		})
	}
}

func TestClearRating(t *testing.T) {
	service, repo := newTestInteractionService(t)
	repo.EXPECT().DeleteRating(uint(1), uint(10)).Return(nil)

	err := service.ClearRating(1, 10)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestGetRating_NotFound(t *testing.T) {
	service, repo := newTestInteractionService(t)
	repo.EXPECT().GetRating(uint(1), uint(10)).Return(nil, gorm.ErrRecordNotFound)

	rating, err := service.GetRating(1, 10)
	if err != nil {
		t.Fatalf("expected no error for not found, got: %v", err)
	}
	if rating != 0 {
		t.Fatalf("expected rating 0 for not found, got: %f", rating)
	}
}

func TestToggleLike_LikeThenUnlike(t *testing.T) {
	t.Run("like when not liked", func(t *testing.T) {
		service, repo := newTestInteractionService(t)
		repo.EXPECT().IsLiked(uint(1), uint(10)).Return(false, nil)
		repo.EXPECT().SetLike(uint(1), uint(10)).Return(nil)

		liked, err := service.ToggleLike(1, 10)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if !liked {
			t.Fatal("expected liked to be true after liking")
		}
	})

	t.Run("unlike when liked", func(t *testing.T) {
		service, repo := newTestInteractionService(t)
		repo.EXPECT().IsLiked(uint(1), uint(10)).Return(true, nil)
		repo.EXPECT().DeleteLike(uint(1), uint(10)).Return(nil)

		liked, err := service.ToggleLike(1, 10)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if liked {
			t.Fatal("expected liked to be false after unliking")
		}
	})
}
