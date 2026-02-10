package core

import (
	"testing"

	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func setupRelatedScenesService(ctrl *gomock.Controller) (
	*RelatedScenesService,
	*mocks.MockSceneRepository,
	*mocks.MockTagRepository,
	*mocks.MockActorRepository,
	*mocks.MockStudioRepository,
	*mocks.MockActorInteractionRepository,
	*mocks.MockStudioInteractionRepository,
	*mocks.MockWatchHistoryRepository,
) {
	mockSceneRepo := mocks.NewMockSceneRepository(ctrl)
	mockTagRepo := mocks.NewMockTagRepository(ctrl)
	mockActorRepo := mocks.NewMockActorRepository(ctrl)
	mockStudioRepo := mocks.NewMockStudioRepository(ctrl)
	mockActorInteractionRepo := mocks.NewMockActorInteractionRepository(ctrl)
	mockStudioInteractionRepo := mocks.NewMockStudioInteractionRepository(ctrl)
	mockWatchHistoryRepo := mocks.NewMockWatchHistoryRepository(ctrl)

	service := NewRelatedScenesService(
		mockSceneRepo,
		mockTagRepo,
		mockActorRepo,
		mockStudioRepo,
		mockActorInteractionRepo,
		mockStudioInteractionRepo,
		mockWatchHistoryRepo,
		zap.NewNop(),
	)

	return service, mockSceneRepo, mockTagRepo, mockActorRepo, mockStudioRepo,
		mockActorInteractionRepo, mockStudioInteractionRepo, mockWatchHistoryRepo
}

func TestRelatedScenesService_GetRelatedScenes(t *testing.T) {
	t.Run("returns scenes from shared actors", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, mockSceneRepo, mockTagRepo, mockActorRepo, _,
			_, _, _ := setupRelatedScenesService(ctrl)

		sceneID := uint(1)
		actor := data.Actor{ID: 10, Name: "Actor 1"}
		relatedScene := data.Scene{ID: 2, Title: "Related Scene"}

		mockSceneRepo.EXPECT().GetByID(sceneID).Return(&data.Scene{ID: sceneID, StudioID: nil}, nil)
		mockActorRepo.EXPECT().GetSceneActors(sceneID).Return([]data.Actor{actor}, nil)
		mockTagRepo.EXPECT().GetSceneTags(sceneID).Return([]data.Tag{}, nil)

		mockActorRepo.EXPECT().GetActorSceneIDs(actor.ID).Return([]uint{2}, nil)

		mockSceneRepo.EXPECT().GetByIDs(gomock.Any()).Return([]data.Scene{relatedScene}, nil)
		mockTagRepo.EXPECT().GetSceneTagsMultiple(gomock.Any()).Return(map[uint][]data.Tag{}, nil)
		mockActorRepo.EXPECT().GetSceneActorsMultiple(gomock.Any()).Return(
			map[uint][]data.Actor{2: {{ID: 10, Name: "Actor 1"}}}, nil)

		// fillWithPopular called because 1 result < limit (12)
		mockSceneRepo.EXPECT().ListPopular(gomock.Any()).Return([]data.Scene{}, nil)

		scenes, err := service.GetRelatedScenes(sceneID, 0, 12)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(scenes) == 0 {
			t.Fatal("expected at least 1 scene")
		}
		if scenes[0].ID != relatedScene.ID {
			t.Errorf("expected scene ID %d, got %d", relatedScene.ID, scenes[0].ID)
		}
	})

	t.Run("excludes source scene from results", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, mockSceneRepo, mockTagRepo, mockActorRepo, _,
			_, _, _ := setupRelatedScenesService(ctrl)

		sceneID := uint(1)
		actor := data.Actor{ID: 10, Name: "Actor 1"}
		relatedScene := data.Scene{ID: 2, Title: "Related Scene"}

		mockSceneRepo.EXPECT().GetByID(sceneID).Return(&data.Scene{ID: sceneID, StudioID: nil}, nil)
		mockActorRepo.EXPECT().GetSceneActors(sceneID).Return([]data.Actor{actor}, nil)
		mockTagRepo.EXPECT().GetSceneTags(sceneID).Return([]data.Tag{}, nil)

		mockActorRepo.EXPECT().GetActorSceneIDs(actor.ID).Return([]uint{sceneID, 2}, nil)

		mockSceneRepo.EXPECT().GetByIDs(gomock.Any()).Return([]data.Scene{relatedScene}, nil)
		mockTagRepo.EXPECT().GetSceneTagsMultiple(gomock.Any()).Return(map[uint][]data.Tag{}, nil)
		mockActorRepo.EXPECT().GetSceneActorsMultiple(gomock.Any()).Return(
			map[uint][]data.Actor{2: {{ID: 10, Name: "Actor 1"}}}, nil)

		mockSceneRepo.EXPECT().ListPopular(gomock.Any()).Return([]data.Scene{}, nil)

		scenes, err := service.GetRelatedScenes(sceneID, 0, 12)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		for _, s := range scenes {
			if s.ID == sceneID {
				t.Errorf("source scene should be excluded from results")
			}
		}
	})

	t.Run("falls back to popular scenes when no matches", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, mockSceneRepo, mockTagRepo, mockActorRepo, _,
			_, _, _ := setupRelatedScenesService(ctrl)

		sceneID := uint(1)
		popularScene := data.Scene{ID: 3, Title: "Popular Scene", ViewCount: 100}

		mockSceneRepo.EXPECT().GetByID(sceneID).Return(&data.Scene{ID: sceneID, StudioID: nil}, nil)
		mockActorRepo.EXPECT().GetSceneActors(sceneID).Return([]data.Actor{}, nil)
		mockTagRepo.EXPECT().GetSceneTags(sceneID).Return([]data.Tag{}, nil)

		mockSceneRepo.EXPECT().ListPopular(gomock.Any()).Return([]data.Scene{popularScene}, nil)

		scenes, err := service.GetRelatedScenes(sceneID, 0, 12)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(scenes) != 1 {
			t.Fatalf("expected 1 scene, got %d", len(scenes))
		}
		if scenes[0].ID != popularScene.ID {
			t.Errorf("expected popular scene ID %d, got %d", popularScene.ID, scenes[0].ID)
		}
	})

	t.Run("returns scenes from studio", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, mockSceneRepo, mockTagRepo, mockActorRepo, mockStudioRepo,
			_, _, _ := setupRelatedScenesService(ctrl)

		sceneID := uint(1)
		studioID := uint(5)
		studioScene := data.Scene{ID: 4, Title: "Studio Scene", StudioID: &studioID}

		mockSceneRepo.EXPECT().GetByID(sceneID).Return(&data.Scene{ID: sceneID, StudioID: &studioID}, nil)
		mockActorRepo.EXPECT().GetSceneActors(sceneID).Return([]data.Actor{}, nil)
		mockTagRepo.EXPECT().GetSceneTags(sceneID).Return([]data.Tag{}, nil)

		mockStudioRepo.EXPECT().GetStudioSceneIDs(studioID, candidateCapStudio).Return([]uint{4}, nil)

		mockSceneRepo.EXPECT().GetByIDs(gomock.Any()).Return([]data.Scene{studioScene}, nil)
		mockTagRepo.EXPECT().GetSceneTagsMultiple(gomock.Any()).Return(map[uint][]data.Tag{}, nil)
		mockActorRepo.EXPECT().GetSceneActorsMultiple(gomock.Any()).Return(map[uint][]data.Actor{}, nil)

		mockSceneRepo.EXPECT().ListPopular(gomock.Any()).Return([]data.Scene{}, nil)

		scenes, err := service.GetRelatedScenes(sceneID, 0, 12)
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

	t.Run("returns scenes from tags", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, mockSceneRepo, mockTagRepo, mockActorRepo, _,
			_, _, _ := setupRelatedScenesService(ctrl)

		sceneID := uint(1)
		tag := data.Tag{ID: 20, Name: "Test Tag"}
		taggedScene := data.Scene{ID: 5, Title: "Tagged Scene"}

		mockSceneRepo.EXPECT().GetByID(sceneID).Return(&data.Scene{ID: sceneID, StudioID: nil}, nil)
		mockActorRepo.EXPECT().GetSceneActors(sceneID).Return([]data.Actor{}, nil)
		mockTagRepo.EXPECT().GetSceneTags(sceneID).Return([]data.Tag{tag}, nil)

		mockTagRepo.EXPECT().GetSceneIDsByTag(tag.ID, candidateCapTags).Return([]uint{5}, nil)

		mockSceneRepo.EXPECT().GetByIDs(gomock.Any()).Return([]data.Scene{taggedScene}, nil)
		mockTagRepo.EXPECT().GetSceneTagsMultiple(gomock.Any()).Return(
			map[uint][]data.Tag{5: {{ID: 20, Name: "Test Tag"}}}, nil)
		mockActorRepo.EXPECT().GetSceneActorsMultiple(gomock.Any()).Return(map[uint][]data.Actor{}, nil)

		mockSceneRepo.EXPECT().ListPopular(gomock.Any()).Return([]data.Scene{}, nil)

		scenes, err := service.GetRelatedScenes(sceneID, 0, 12)
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

	t.Run("respects limit parameter", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, mockSceneRepo, mockTagRepo, mockActorRepo, _,
			_, _, _ := setupRelatedScenesService(ctrl)

		sceneID := uint(1)
		limit := 3

		tag := data.Tag{ID: 20, Name: "Test Tag"}
		tagSceneIDs := []uint{2, 3, 4, 5, 6, 7, 8, 9, 10}
		taggedScenes := make([]data.Scene, len(tagSceneIDs))
		tagsByScene := make(map[uint][]data.Tag)
		for i, id := range tagSceneIDs {
			taggedScenes[i] = data.Scene{ID: id, Title: "Scene"}
			tagsByScene[id] = []data.Tag{{ID: 20, Name: "Test Tag"}}
		}

		mockSceneRepo.EXPECT().GetByID(sceneID).Return(&data.Scene{ID: sceneID, StudioID: nil}, nil)
		mockActorRepo.EXPECT().GetSceneActors(sceneID).Return([]data.Actor{}, nil)
		mockTagRepo.EXPECT().GetSceneTags(sceneID).Return([]data.Tag{tag}, nil)

		mockTagRepo.EXPECT().GetSceneIDsByTag(tag.ID, candidateCapTags).Return(tagSceneIDs, nil)

		mockSceneRepo.EXPECT().GetByIDs(gomock.Any()).Return(taggedScenes, nil)
		mockTagRepo.EXPECT().GetSceneTagsMultiple(gomock.Any()).Return(tagsByScene, nil)
		mockActorRepo.EXPECT().GetSceneActorsMultiple(gomock.Any()).Return(map[uint][]data.Actor{}, nil)

		scenes, err := service.GetRelatedScenes(sceneID, 0, limit)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(scenes) > limit {
			t.Errorf("expected at most %d scenes, got %d", limit, len(scenes))
		}
	})

	t.Run("caps limit at 50", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, mockSceneRepo, mockTagRepo, mockActorRepo, _,
			_, _, _ := setupRelatedScenesService(ctrl)

		sceneID := uint(1)

		mockSceneRepo.EXPECT().GetByID(sceneID).Return(&data.Scene{ID: sceneID, StudioID: nil}, nil)
		mockActorRepo.EXPECT().GetSceneActors(sceneID).Return([]data.Actor{}, nil)
		mockTagRepo.EXPECT().GetSceneTags(sceneID).Return([]data.Tag{}, nil)
		mockSceneRepo.EXPECT().ListPopular(gomock.Any()).Return([]data.Scene{}, nil)

		scenes, err := service.GetRelatedScenes(sceneID, 0, 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if scenes == nil {
			t.Errorf("expected non-nil slice")
		}
	})

	t.Run("accumulates scores across actors and tags", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, mockSceneRepo, mockTagRepo, mockActorRepo, _,
			_, _, _ := setupRelatedScenesService(ctrl)

		sceneID := uint(1)
		actor := data.Actor{ID: 10, Name: "Actor 1"}
		tag := data.Tag{ID: 20, Name: "Tag 1"}

		scene2 := data.Scene{ID: 2, Title: "Both Match"}
		scene3 := data.Scene{ID: 3, Title: "Tag Only"}

		mockSceneRepo.EXPECT().GetByID(sceneID).Return(&data.Scene{ID: sceneID, StudioID: nil}, nil)
		mockActorRepo.EXPECT().GetSceneActors(sceneID).Return([]data.Actor{actor}, nil)
		mockTagRepo.EXPECT().GetSceneTags(sceneID).Return([]data.Tag{tag}, nil)

		mockActorRepo.EXPECT().GetActorSceneIDs(actor.ID).Return([]uint{2}, nil)
		mockTagRepo.EXPECT().GetSceneIDsByTag(tag.ID, candidateCapTags).Return([]uint{2, 3}, nil)

		mockSceneRepo.EXPECT().GetByIDs(gomock.Any()).Return([]data.Scene{scene2, scene3}, nil)
		mockTagRepo.EXPECT().GetSceneTagsMultiple(gomock.Any()).Return(map[uint][]data.Tag{
			2: {{ID: 20}},
			3: {{ID: 20}},
		}, nil)
		mockActorRepo.EXPECT().GetSceneActorsMultiple(gomock.Any()).Return(map[uint][]data.Actor{
			2: {{ID: 10}},
		}, nil)

		mockSceneRepo.EXPECT().ListPopular(gomock.Any()).Return([]data.Scene{}, nil)

		scenes, err := service.GetRelatedScenes(sceneID, 0, 12)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(scenes) < 2 {
			t.Fatalf("expected at least 2 scenes, got %d", len(scenes))
		}
		// Scene with both actor+tag should rank first (40+8=48 vs 8)
		if scenes[0].ID != 2 {
			t.Errorf("expected scene 2 (actor+tag) to rank first, got scene %d", scenes[0].ID)
		}
	})

	t.Run("applies watched penalty for logged in user", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, mockSceneRepo, mockTagRepo, mockActorRepo, _,
			mockActorInteractionRepo, mockStudioInteractionRepo, mockWatchHistoryRepo := setupRelatedScenesService(ctrl)

		sceneID := uint(1)
		userID := uint(42)
		tag := data.Tag{ID: 20, Name: "Tag 1"}

		scene2 := data.Scene{ID: 2, Title: "Watched"}
		scene3 := data.Scene{ID: 3, Title: "Not Watched"}

		mockSceneRepo.EXPECT().GetByID(sceneID).Return(&data.Scene{ID: sceneID, StudioID: nil}, nil)
		mockActorRepo.EXPECT().GetSceneActors(sceneID).Return([]data.Actor{}, nil)
		mockTagRepo.EXPECT().GetSceneTags(sceneID).Return([]data.Tag{tag}, nil)

		mockTagRepo.EXPECT().GetSceneIDsByTag(tag.ID, candidateCapTags).Return([]uint{2, 3}, nil)

		mockActorInteractionRepo.EXPECT().GetLikedActorIDs(userID).Return([]uint{}, nil)
		mockStudioInteractionRepo.EXPECT().GetLikedStudioIDs(userID).Return([]uint{}, nil)
		mockWatchHistoryRepo.EXPECT().GetWatchedSceneIDs(userID, 500).Return([]uint{2}, nil)

		mockSceneRepo.EXPECT().GetByIDs(gomock.Any()).Return([]data.Scene{scene2, scene3}, nil)
		mockTagRepo.EXPECT().GetSceneTagsMultiple(gomock.Any()).Return(map[uint][]data.Tag{
			2: {{ID: 20}},
			3: {{ID: 20}},
		}, nil)
		mockActorRepo.EXPECT().GetSceneActorsMultiple(gomock.Any()).Return(map[uint][]data.Actor{}, nil)

		mockSceneRepo.EXPECT().ListPopular(gomock.Any()).Return([]data.Scene{}, nil)

		scenes, err := service.GetRelatedScenes(sceneID, userID, 12)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(scenes) < 2 {
			t.Fatalf("expected 2 scenes, got %d", len(scenes))
		}
		// Unwatched scene should rank higher (8 vs 8-30=0 clamped)
		if scenes[0].ID != 3 {
			t.Errorf("expected unwatched scene 3 to rank first, got scene %d", scenes[0].ID)
		}
	})

	t.Run("applies liked actor bonus for logged in user", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, mockSceneRepo, mockTagRepo, mockActorRepo, _,
			mockActorInteractionRepo, mockStudioInteractionRepo, mockWatchHistoryRepo := setupRelatedScenesService(ctrl)

		sceneID := uint(1)
		userID := uint(42)
		actor1 := data.Actor{ID: 10, Name: "Liked Actor"}
		actor2 := data.Actor{ID: 11, Name: "Not Liked Actor"}

		scene2 := data.Scene{ID: 2, Title: "Liked Actor Scene"}
		scene3 := data.Scene{ID: 3, Title: "Non-Liked Actor Scene"}

		mockSceneRepo.EXPECT().GetByID(sceneID).Return(&data.Scene{ID: sceneID, StudioID: nil}, nil)
		mockActorRepo.EXPECT().GetSceneActors(sceneID).Return([]data.Actor{actor1, actor2}, nil)
		mockTagRepo.EXPECT().GetSceneTags(sceneID).Return([]data.Tag{}, nil)

		mockActorRepo.EXPECT().GetActorSceneIDs(actor1.ID).Return([]uint{2}, nil)
		mockActorRepo.EXPECT().GetActorSceneIDs(actor2.ID).Return([]uint{3}, nil)

		mockActorInteractionRepo.EXPECT().GetLikedActorIDs(userID).Return([]uint{10}, nil)
		mockStudioInteractionRepo.EXPECT().GetLikedStudioIDs(userID).Return([]uint{}, nil)
		mockWatchHistoryRepo.EXPECT().GetWatchedSceneIDs(userID, 500).Return([]uint{}, nil)

		mockSceneRepo.EXPECT().GetByIDs(gomock.Any()).Return([]data.Scene{scene2, scene3}, nil)
		mockTagRepo.EXPECT().GetSceneTagsMultiple(gomock.Any()).Return(map[uint][]data.Tag{}, nil)
		mockActorRepo.EXPECT().GetSceneActorsMultiple(gomock.Any()).Return(map[uint][]data.Actor{
			2: {{ID: 10, Name: "Liked Actor"}},
			3: {{ID: 11, Name: "Not Liked Actor"}},
		}, nil)

		mockSceneRepo.EXPECT().ListPopular(gomock.Any()).Return([]data.Scene{}, nil)

		scenes, err := service.GetRelatedScenes(sceneID, userID, 12)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(scenes) < 2 {
			t.Fatalf("expected 2 scenes, got %d", len(scenes))
		}
		// Scene with liked actor should rank first (40 + 25 = 65 vs 40)
		if scenes[0].ID != 2 {
			t.Errorf("expected scene 2 (liked actor) to rank first, got scene %d", scenes[0].ID)
		}
	})

	t.Run("no user personalization when userID is 0", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, mockSceneRepo, mockTagRepo, mockActorRepo, _,
			_, _, _ := setupRelatedScenesService(ctrl)

		sceneID := uint(1)
		tag := data.Tag{ID: 20, Name: "Tag 1"}
		taggedScene := data.Scene{ID: 2, Title: "Scene"}

		mockSceneRepo.EXPECT().GetByID(sceneID).Return(&data.Scene{ID: sceneID, StudioID: nil}, nil)
		mockActorRepo.EXPECT().GetSceneActors(sceneID).Return([]data.Actor{}, nil)
		mockTagRepo.EXPECT().GetSceneTags(sceneID).Return([]data.Tag{tag}, nil)

		mockTagRepo.EXPECT().GetSceneIDsByTag(tag.ID, candidateCapTags).Return([]uint{2}, nil)

		mockSceneRepo.EXPECT().GetByIDs(gomock.Any()).Return([]data.Scene{taggedScene}, nil)
		mockTagRepo.EXPECT().GetSceneTagsMultiple(gomock.Any()).Return(
			map[uint][]data.Tag{2: {{ID: 20}}}, nil)
		mockActorRepo.EXPECT().GetSceneActorsMultiple(gomock.Any()).Return(map[uint][]data.Actor{}, nil)

		mockSceneRepo.EXPECT().ListPopular(gomock.Any()).Return([]data.Scene{}, nil)

		scenes, err := service.GetRelatedScenes(sceneID, 0, 12)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(scenes) == 0 {
			t.Fatal("expected at least 1 scene")
		}
	})
}
