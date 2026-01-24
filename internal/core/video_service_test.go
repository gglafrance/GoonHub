package core

import (
	"fmt"
	"goonhub/internal/data"
	"goonhub/internal/mocks"
	"testing"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func newTestVideoService(t *testing.T) (*VideoService, *mocks.MockVideoRepository) {
	ctrl := gomock.NewController(t)
	videoRepo := mocks.NewMockVideoRepository(ctrl)

	svc := &VideoService{
		Repo:              videoRepo,
		DataPath:          t.TempDir(),
		ProcessingService: nil,
		logger:            zap.NewNop(),
	}
	return svc, videoRepo
}

func TestUpload_ExtensionValidation(t *testing.T) {
	tests := []struct {
		filename string
		valid    bool
	}{
		// Valid extensions
		{"video.mp4", true},
		{"video.mkv", true},
		{"video.avi", true},
		{"video.mov", true},
		{"video.webm", true},
		{"video.wmv", true},
		{"video.m4v", true},
		// Case insensitive
		{"video.MP4", true},
		{"video.MKV", true},
		{"video.Mp4", true},
		// Invalid extensions
		{"malware.exe", false},
		{"audio.mp3", false},
		{"document.txt", false},
		{"image.png", false},
		{"noextension", false},
		{"", false},
		{"video.flv", false},
	}

	svc, _ := newTestVideoService(t)

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := svc.ValidateExtension(tt.filename)
			if result != tt.valid {
				t.Fatalf("ValidateExtension(%q) = %v, want %v", tt.filename, result, tt.valid)
			}
		})
	}
}

func TestListVideos_Pagination(t *testing.T) {
	svc, videoRepo := newTestVideoService(t)

	videos := []data.Video{
		{ID: 1, Title: "Video 1"},
		{ID: 2, Title: "Video 2"},
	}

	videoRepo.EXPECT().List(3, 10).Return(videos, int64(50), nil)

	result, total, err := svc.ListVideos(3, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 50 {
		t.Fatalf("expected total 50, got %d", total)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 videos, got %d", len(result))
	}
}

func TestListVideos_DefaultsForInvalidInput(t *testing.T) {
	svc, videoRepo := newTestVideoService(t)

	// page < 1 defaults to 1, limit < 1 defaults to 20
	videoRepo.EXPECT().List(1, 20).Return(nil, int64(0), nil)

	_, _, err := svc.ListVideos(0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteVideo_RepoInteraction(t *testing.T) {
	svc, videoRepo := newTestVideoService(t)

	video := &data.Video{
		ID:         1,
		StoredPath: "/nonexistent/path/video.mp4", // file won't exist, that's fine
	}

	// GetByID called first, then Delete
	videoRepo.EXPECT().GetByID(uint(1)).Return(video, nil)
	videoRepo.EXPECT().Delete(uint(1)).Return(nil)

	err := svc.DeleteVideo(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateVideoDetails_Success(t *testing.T) {
	svc, videoRepo := newTestVideoService(t)

	videoRepo.EXPECT().UpdateDetails(uint(1), "New Title", "New Description").Return(nil)
	videoRepo.EXPECT().GetByID(uint(1)).Return(&data.Video{
		ID:          1,
		Title:       "New Title",
		Description: "New Description",
	}, nil)

	video, err := svc.UpdateVideoDetails(1, "New Title", "New Description")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if video.Title != "New Title" {
		t.Fatalf("expected title 'New Title', got %q", video.Title)
	}
	if video.Description != "New Description" {
		t.Fatalf("expected description 'New Description', got %q", video.Description)
	}
}

func TestUpdateVideoDetails_UpdateFails(t *testing.T) {
	svc, videoRepo := newTestVideoService(t)

	videoRepo.EXPECT().UpdateDetails(uint(1), "Title", "Desc").Return(fmt.Errorf("db error"))

	_, err := svc.UpdateVideoDetails(1, "Title", "Desc")
	if err == nil {
		t.Fatal("expected error when update fails")
	}
}

func TestDeleteVideo_NotFound(t *testing.T) {
	svc, videoRepo := newTestVideoService(t)

	videoRepo.EXPECT().GetByID(uint(99)).Return(nil, fmt.Errorf("record not found"))

	err := svc.DeleteVideo(99)
	if err == nil {
		t.Fatal("expected error for non-existent video")
	}
}
