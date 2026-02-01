package core

import (
	"fmt"
	"goonhub/internal/data"
	"goonhub/internal/mocks"
	"testing"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func newTestSceneService(t *testing.T) (*SceneService, *mocks.MockSceneRepository) {
	ctrl := gomock.NewController(t)
	sceneRepo := mocks.NewMockSceneRepository(ctrl)

	tempDir := t.TempDir()
	svc := &SceneService{
		Repo:              sceneRepo,
		ScenePath:         tempDir,
		MetadataPath:      tempDir,
		ProcessingService: nil,
		logger:            zap.NewNop(),
	}
	return svc, sceneRepo
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

	svc, _ := newTestSceneService(t)

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := svc.ValidateExtension(tt.filename)
			if result != tt.valid {
				t.Fatalf("ValidateExtension(%q) = %v, want %v", tt.filename, result, tt.valid)
			}
		})
	}
}

func TestListScenes_Pagination(t *testing.T) {
	svc, sceneRepo := newTestSceneService(t)

	scenes := []data.Scene{
		{ID: 1, Title: "Scene 1"},
		{ID: 2, Title: "Scene 2"},
	}

	sceneRepo.EXPECT().List(3, 10).Return(scenes, int64(50), nil)

	result, total, err := svc.ListScenes(3, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 50 {
		t.Fatalf("expected total 50, got %d", total)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 scenes, got %d", len(result))
	}
}

func TestListScenes_DefaultsForInvalidInput(t *testing.T) {
	svc, sceneRepo := newTestSceneService(t)

	// page < 1 defaults to 1, limit < 1 defaults to 20
	sceneRepo.EXPECT().List(1, 20).Return(nil, int64(0), nil)

	_, _, err := svc.ListScenes(0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteScene_RepoInteraction(t *testing.T) {
	svc, sceneRepo := newTestSceneService(t)

	scene := &data.Scene{
		ID:         1,
		StoredPath: "/nonexistent/path/video.mp4", // file won't exist, that's fine
	}

	// GetByID called first, then Delete
	sceneRepo.EXPECT().GetByID(uint(1)).Return(scene, nil)
	sceneRepo.EXPECT().Delete(uint(1)).Return(nil)

	err := svc.DeleteScene(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateSceneDetails_Success(t *testing.T) {
	svc, sceneRepo := newTestSceneService(t)

	sceneRepo.EXPECT().UpdateDetails(uint(1), "New Title", "New Description", gomock.Any()).Return(nil)
	sceneRepo.EXPECT().GetByID(uint(1)).Return(&data.Scene{
		ID:          1,
		Title:       "New Title",
		Description: "New Description",
	}, nil)

	scene, err := svc.UpdateSceneDetails(1, "New Title", "New Description", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scene.Title != "New Title" {
		t.Fatalf("expected title 'New Title', got %q", scene.Title)
	}
	if scene.Description != "New Description" {
		t.Fatalf("expected description 'New Description', got %q", scene.Description)
	}
}

func TestUpdateSceneDetails_UpdateFails(t *testing.T) {
	svc, sceneRepo := newTestSceneService(t)

	sceneRepo.EXPECT().UpdateDetails(uint(1), "Title", "Desc", gomock.Any()).Return(fmt.Errorf("db error"))

	_, err := svc.UpdateSceneDetails(1, "Title", "Desc", nil)
	if err == nil {
		t.Fatal("expected error when update fails")
	}
}

func TestDeleteScene_NotFound(t *testing.T) {
	svc, sceneRepo := newTestSceneService(t)

	sceneRepo.EXPECT().GetByID(uint(99)).Return(nil, fmt.Errorf("record not found"))

	err := svc.DeleteScene(99)
	if err == nil {
		t.Fatal("expected error for non-existent scene")
	}
}
