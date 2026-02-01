package core

import (
	"testing"

	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestRelatedScenesService_GetRelatedScenes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSceneRepo := mocks.NewMockSceneRepository(ctrl)
	mockTagRepo := mocks.NewMockTagRepository(ctrl)
	mockActorRepo := mocks.NewMockActorRepository(ctrl)
	mockStudioRepo := mocks.NewMockStudioRepository(ctrl)

	service := NewRelatedScenesService(
		mockSceneRepo,
		mockTagRepo,
		mockActorRepo,
		mockStudioRepo,
		zap.NewNop(),
	)

	t.Run("returns scenes from actors", func(t *testing.T) {
		sceneID := uint(1)
		actor := data.Actor{ID: 10, Name: "Actor 1"}
		relatedScene := data.Scene{ID: 2, Title: "Related Scene"}

		mockActorRepo.EXPECT().GetSceneActors(sceneID).Return([]data.Actor{actor}, nil)
		mockActorRepo.EXPECT().GetActorScenes(actor.ID, 1, gomock.Any()).Return([]data.Scene{relatedScene}, int64(1), nil)
		mockTagRepo.EXPECT().GetSceneTags(sceneID).Return([]data.Tag{}, nil)
		mockSceneRepo.EXPECT().GetByID(sceneID).Return(&data.Scene{ID: sceneID, StudioID: nil}, nil)
		mockSceneRepo.EXPECT().List(1, gomock.Any()).Return([]data.Scene{}, int64(0), nil)

		scenes, err := service.GetRelatedScenes(sceneID, 12)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(scenes) != 1 {
			t.Fatalf("expected 1 scene, got %d", len(scenes))
		}
		if scenes[0].ID != relatedScene.ID {
			t.Errorf("expected scene ID %d, got %d", relatedScene.ID, scenes[0].ID)
		}
	})

	t.Run("excludes source scene from results", func(t *testing.T) {
		sceneID := uint(1)
		actor := data.Actor{ID: 10, Name: "Actor 1"}
		sourceScene := data.Scene{ID: sceneID, Title: "Source Scene"}
		relatedScene := data.Scene{ID: 2, Title: "Related Scene"}

		mockActorRepo.EXPECT().GetSceneActors(sceneID).Return([]data.Actor{actor}, nil)
		mockActorRepo.EXPECT().GetActorScenes(actor.ID, 1, gomock.Any()).Return([]data.Scene{sourceScene, relatedScene}, int64(2), nil)
		mockTagRepo.EXPECT().GetSceneTags(sceneID).Return([]data.Tag{}, nil)
		mockSceneRepo.EXPECT().GetByID(sceneID).Return(&data.Scene{ID: sceneID, StudioID: nil}, nil)
		mockSceneRepo.EXPECT().List(1, gomock.Any()).Return([]data.Scene{}, int64(0), nil)

		scenes, err := service.GetRelatedScenes(sceneID, 12)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		for _, s := range scenes {
			if s.ID == sceneID {
				t.Errorf("source scene should be excluded from results")
			}
		}
	})

	t.Run("falls back to recent scenes when no matches", func(t *testing.T) {
		sceneID := uint(1)
		fallbackScene := data.Scene{ID: 3, Title: "Fallback Scene"}

		mockActorRepo.EXPECT().GetSceneActors(sceneID).Return([]data.Actor{}, nil)
		mockTagRepo.EXPECT().GetSceneTags(sceneID).Return([]data.Tag{}, nil)
		mockSceneRepo.EXPECT().GetByID(sceneID).Return(&data.Scene{ID: sceneID, StudioID: nil}, nil)
		mockSceneRepo.EXPECT().List(1, gomock.Any()).Return([]data.Scene{fallbackScene}, int64(1), nil)

		scenes, err := service.GetRelatedScenes(sceneID, 12)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(scenes) != 1 {
			t.Fatalf("expected 1 fallback scene, got %d", len(scenes))
		}
		if scenes[0].ID != fallbackScene.ID {
			t.Errorf("expected fallback scene ID %d, got %d", fallbackScene.ID, scenes[0].ID)
		}
	})

	t.Run("returns scenes from studio", func(t *testing.T) {
		sceneID := uint(1)
		studioID := uint(5)
		studioScene := data.Scene{ID: 4, Title: "Studio Scene"}

		mockActorRepo.EXPECT().GetSceneActors(sceneID).Return([]data.Actor{}, nil)
		mockTagRepo.EXPECT().GetSceneTags(sceneID).Return([]data.Tag{}, nil)
		mockSceneRepo.EXPECT().GetByID(sceneID).Return(&data.Scene{ID: sceneID, StudioID: &studioID}, nil)
		mockStudioRepo.EXPECT().GetStudioScenes(studioID, 1, gomock.Any()).Return([]data.Scene{studioScene}, int64(1), nil)
		mockSceneRepo.EXPECT().List(1, gomock.Any()).Return([]data.Scene{}, int64(0), nil)

		scenes, err := service.GetRelatedScenes(sceneID, 12)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		found := false
		for _, s := range scenes {
			if s.ID == studioScene.ID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected studio scene in results")
		}
	})

	t.Run("respects limit parameter", func(t *testing.T) {
		sceneID := uint(1)
		limit := 5
		fallbackScenes := make([]data.Scene, 10)
		for i := range fallbackScenes {
			fallbackScenes[i] = data.Scene{ID: uint(i + 2), Title: "Scene"}
		}

		mockActorRepo.EXPECT().GetSceneActors(sceneID).Return([]data.Actor{}, nil)
		mockTagRepo.EXPECT().GetSceneTags(sceneID).Return([]data.Tag{}, nil)
		mockSceneRepo.EXPECT().GetByID(sceneID).Return(&data.Scene{ID: sceneID, StudioID: nil}, nil)
		mockSceneRepo.EXPECT().List(1, gomock.Any()).Return(fallbackScenes, int64(10), nil)

		scenes, err := service.GetRelatedScenes(sceneID, limit)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(scenes) > limit {
			t.Errorf("expected at most %d scenes, got %d", limit, len(scenes))
		}
	})

	t.Run("caps limit at 50", func(t *testing.T) {
		sceneID := uint(1)

		mockActorRepo.EXPECT().GetSceneActors(sceneID).Return([]data.Actor{}, nil)
		mockTagRepo.EXPECT().GetSceneTags(sceneID).Return([]data.Tag{}, nil)
		mockSceneRepo.EXPECT().GetByID(sceneID).Return(&data.Scene{ID: sceneID, StudioID: nil}, nil)
		mockSceneRepo.EXPECT().List(1, gomock.Any()).Return([]data.Scene{}, int64(0), nil)

		scenes, err := service.GetRelatedScenes(sceneID, 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Just verify it doesn't crash with large limit
		if scenes == nil {
			t.Errorf("expected non-nil slice")
		}
	})

	t.Run("returns scenes from tags", func(t *testing.T) {
		sceneID := uint(1)
		tag := data.Tag{ID: 20, Name: "Test Tag"}
		taggedSceneID := uint(5)
		taggedScene := data.Scene{ID: taggedSceneID, Title: "Tagged Scene"}

		mockActorRepo.EXPECT().GetSceneActors(sceneID).Return([]data.Actor{}, nil)
		mockTagRepo.EXPECT().GetSceneTags(sceneID).Return([]data.Tag{tag}, nil)
		mockTagRepo.EXPECT().GetSceneIDsByTag(tag.ID, gomock.Any()).Return([]uint{taggedSceneID}, nil)
		mockSceneRepo.EXPECT().GetByIDs([]uint{taggedSceneID}).Return([]data.Scene{taggedScene}, nil)
		mockSceneRepo.EXPECT().GetByID(sceneID).Return(&data.Scene{ID: sceneID, StudioID: nil}, nil)
		mockSceneRepo.EXPECT().List(1, gomock.Any()).Return([]data.Scene{}, int64(0), nil)

		scenes, err := service.GetRelatedScenes(sceneID, 12)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		found := false
		for _, s := range scenes {
			if s.ID == taggedScene.ID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected tagged scene in results")
		}
	})
}
