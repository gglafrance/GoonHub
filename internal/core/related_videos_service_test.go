package core

import (
	"testing"

	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestRelatedVideosService_GetRelatedVideos(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockVideoRepo := mocks.NewMockVideoRepository(ctrl)
	mockTagRepo := mocks.NewMockTagRepository(ctrl)
	mockActorRepo := mocks.NewMockActorRepository(ctrl)
	mockStudioRepo := mocks.NewMockStudioRepository(ctrl)

	service := NewRelatedVideosService(
		mockVideoRepo,
		mockTagRepo,
		mockActorRepo,
		mockStudioRepo,
		zap.NewNop(),
	)

	t.Run("returns videos from actors", func(t *testing.T) {
		videoID := uint(1)
		actor := data.Actor{ID: 10, Name: "Actor 1"}
		relatedVideo := data.Video{ID: 2, Title: "Related Video"}

		mockActorRepo.EXPECT().GetVideoActors(videoID).Return([]data.Actor{actor}, nil)
		mockActorRepo.EXPECT().GetActorVideos(actor.ID, 1, gomock.Any()).Return([]data.Video{relatedVideo}, int64(1), nil)
		mockTagRepo.EXPECT().GetVideoTags(videoID).Return([]data.Tag{}, nil)
		mockVideoRepo.EXPECT().GetByID(videoID).Return(&data.Video{ID: videoID, StudioID: nil}, nil)
		mockVideoRepo.EXPECT().List(1, gomock.Any()).Return([]data.Video{}, int64(0), nil)

		videos, err := service.GetRelatedVideos(videoID, 12)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(videos) != 1 {
			t.Fatalf("expected 1 video, got %d", len(videos))
		}
		if videos[0].ID != relatedVideo.ID {
			t.Errorf("expected video ID %d, got %d", relatedVideo.ID, videos[0].ID)
		}
	})

	t.Run("excludes source video from results", func(t *testing.T) {
		videoID := uint(1)
		actor := data.Actor{ID: 10, Name: "Actor 1"}
		sourceVideo := data.Video{ID: videoID, Title: "Source Video"}
		relatedVideo := data.Video{ID: 2, Title: "Related Video"}

		mockActorRepo.EXPECT().GetVideoActors(videoID).Return([]data.Actor{actor}, nil)
		mockActorRepo.EXPECT().GetActorVideos(actor.ID, 1, gomock.Any()).Return([]data.Video{sourceVideo, relatedVideo}, int64(2), nil)
		mockTagRepo.EXPECT().GetVideoTags(videoID).Return([]data.Tag{}, nil)
		mockVideoRepo.EXPECT().GetByID(videoID).Return(&data.Video{ID: videoID, StudioID: nil}, nil)
		mockVideoRepo.EXPECT().List(1, gomock.Any()).Return([]data.Video{}, int64(0), nil)

		videos, err := service.GetRelatedVideos(videoID, 12)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		for _, v := range videos {
			if v.ID == videoID {
				t.Errorf("source video should be excluded from results")
			}
		}
	})

	t.Run("falls back to recent videos when no matches", func(t *testing.T) {
		videoID := uint(1)
		fallbackVideo := data.Video{ID: 3, Title: "Fallback Video"}

		mockActorRepo.EXPECT().GetVideoActors(videoID).Return([]data.Actor{}, nil)
		mockTagRepo.EXPECT().GetVideoTags(videoID).Return([]data.Tag{}, nil)
		mockVideoRepo.EXPECT().GetByID(videoID).Return(&data.Video{ID: videoID, StudioID: nil}, nil)
		mockVideoRepo.EXPECT().List(1, gomock.Any()).Return([]data.Video{fallbackVideo}, int64(1), nil)

		videos, err := service.GetRelatedVideos(videoID, 12)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(videos) != 1 {
			t.Fatalf("expected 1 fallback video, got %d", len(videos))
		}
		if videos[0].ID != fallbackVideo.ID {
			t.Errorf("expected fallback video ID %d, got %d", fallbackVideo.ID, videos[0].ID)
		}
	})

	t.Run("returns videos from studio", func(t *testing.T) {
		videoID := uint(1)
		studioID := uint(5)
		studioVideo := data.Video{ID: 4, Title: "Studio Video"}

		mockActorRepo.EXPECT().GetVideoActors(videoID).Return([]data.Actor{}, nil)
		mockTagRepo.EXPECT().GetVideoTags(videoID).Return([]data.Tag{}, nil)
		mockVideoRepo.EXPECT().GetByID(videoID).Return(&data.Video{ID: videoID, StudioID: &studioID}, nil)
		mockStudioRepo.EXPECT().GetStudioVideos(studioID, 1, gomock.Any()).Return([]data.Video{studioVideo}, int64(1), nil)
		mockVideoRepo.EXPECT().List(1, gomock.Any()).Return([]data.Video{}, int64(0), nil)

		videos, err := service.GetRelatedVideos(videoID, 12)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		found := false
		for _, v := range videos {
			if v.ID == studioVideo.ID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected studio video in results")
		}
	})

	t.Run("respects limit parameter", func(t *testing.T) {
		videoID := uint(1)
		limit := 5
		fallbackVideos := make([]data.Video, 10)
		for i := range fallbackVideos {
			fallbackVideos[i] = data.Video{ID: uint(i + 2), Title: "Video"}
		}

		mockActorRepo.EXPECT().GetVideoActors(videoID).Return([]data.Actor{}, nil)
		mockTagRepo.EXPECT().GetVideoTags(videoID).Return([]data.Tag{}, nil)
		mockVideoRepo.EXPECT().GetByID(videoID).Return(&data.Video{ID: videoID, StudioID: nil}, nil)
		mockVideoRepo.EXPECT().List(1, gomock.Any()).Return(fallbackVideos, int64(10), nil)

		videos, err := service.GetRelatedVideos(videoID, limit)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(videos) > limit {
			t.Errorf("expected at most %d videos, got %d", limit, len(videos))
		}
	})

	t.Run("caps limit at 50", func(t *testing.T) {
		videoID := uint(1)

		mockActorRepo.EXPECT().GetVideoActors(videoID).Return([]data.Actor{}, nil)
		mockTagRepo.EXPECT().GetVideoTags(videoID).Return([]data.Tag{}, nil)
		mockVideoRepo.EXPECT().GetByID(videoID).Return(&data.Video{ID: videoID, StudioID: nil}, nil)
		mockVideoRepo.EXPECT().List(1, gomock.Any()).Return([]data.Video{}, int64(0), nil)

		videos, err := service.GetRelatedVideos(videoID, 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Just verify it doesn't crash with large limit
		if videos == nil {
			t.Errorf("expected non-nil slice")
		}
	})

	t.Run("returns videos from tags", func(t *testing.T) {
		videoID := uint(1)
		tag := data.Tag{ID: 20, Name: "Test Tag"}
		taggedVideoID := uint(5)
		taggedVideo := data.Video{ID: taggedVideoID, Title: "Tagged Video"}

		mockActorRepo.EXPECT().GetVideoActors(videoID).Return([]data.Actor{}, nil)
		mockTagRepo.EXPECT().GetVideoTags(videoID).Return([]data.Tag{tag}, nil)
		mockTagRepo.EXPECT().GetVideoIDsByTag(tag.ID, gomock.Any()).Return([]uint{taggedVideoID}, nil)
		mockVideoRepo.EXPECT().GetByIDs([]uint{taggedVideoID}).Return([]data.Video{taggedVideo}, nil)
		mockVideoRepo.EXPECT().GetByID(videoID).Return(&data.Video{ID: videoID, StudioID: nil}, nil)
		mockVideoRepo.EXPECT().List(1, gomock.Any()).Return([]data.Video{}, int64(0), nil)

		videos, err := service.GetRelatedVideos(videoID, 12)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		found := false
		for _, v := range videos {
			if v.ID == taggedVideo.ID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected tagged video in results")
		}
	})
}
