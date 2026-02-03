package core

import (
	"fmt"
	"goonhub/internal/data"
	"goonhub/internal/mocks"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func newTestWatchHistoryService(t *testing.T) (*WatchHistoryService, *mocks.MockWatchHistoryRepository, *mocks.MockSceneRepository) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockWatchHistoryRepository(ctrl)
	sceneRepo := mocks.NewMockSceneRepository(ctrl)
	logger := zap.NewNop()
	service := NewWatchHistoryService(repo, sceneRepo, nil, logger)
	return service, repo, sceneRepo
}

func TestComputeSinceTime_PositiveDays(t *testing.T) {
	before := time.Now().UTC().AddDate(0, 0, -30)
	result := computeSinceTime(30)
	after := time.Now().UTC().AddDate(0, 0, -30)

	if result.Before(before.Add(-time.Second)) || result.After(after.Add(time.Second)) {
		t.Fatalf("expected since time ~30 days ago, got %v", result)
	}
}

func TestComputeSinceTime_ZeroMeansAllTime(t *testing.T) {
	result := computeSinceTime(0)
	expected := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Fatalf("expected sentinel year-2000 time, got %v", result)
	}
}

func TestComputeSinceTime_NegativeMeansAllTime(t *testing.T) {
	result := computeSinceTime(-5)
	expected := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Fatalf("expected sentinel year-2000 time, got %v", result)
	}
}

func TestGetUserHistoryByDateRange_Success(t *testing.T) {
	service, repo, sceneRepo := newTestWatchHistoryService(t)

	now := time.Now().UTC()
	watches := []data.UserSceneWatch{
		{ID: 1, UserID: 1, SceneID: 10, WatchedAt: now},
		{ID: 2, UserID: 1, SceneID: 20, WatchedAt: now.Add(-time.Hour)},
	}
	scenes := []data.Scene{
		{ID: 10, Title: "Scene 10"},
		{ID: 20, Title: "Scene 20"},
	}

	repo.EXPECT().ListUserHistoryByDateRange(uint(1), gomock.Any(), 2000).Return(watches, nil)
	sceneRepo.EXPECT().GetByIDs(gomock.Any()).Return(scenes, nil)

	entries, err := service.GetUserHistoryByDateRange(1, 30, 2000)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Scene == nil || entries[0].Scene.ID != 10 {
		t.Fatal("expected first entry to have scene ID 10")
	}
	if entries[1].Scene == nil || entries[1].Scene.ID != 20 {
		t.Fatal("expected second entry to have scene ID 20")
	}
}

func TestGetUserHistoryByDateRange_DefaultLimit(t *testing.T) {
	service, repo, sceneRepo := newTestWatchHistoryService(t)

	repo.EXPECT().ListUserHistoryByDateRange(uint(1), gomock.Any(), 2000).Return(nil, nil)
	sceneRepo.EXPECT().GetByIDs(gomock.Any()).Return(nil, nil)

	entries, err := service.GetUserHistoryByDateRange(1, 30, 0)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if entries == nil {
		t.Fatal("expected empty slice, got nil")
	}
}

func TestGetUserHistoryByDateRange_RepoError(t *testing.T) {
	service, repo, _ := newTestWatchHistoryService(t)

	repo.EXPECT().ListUserHistoryByDateRange(uint(1), gomock.Any(), 2000).Return(nil, fmt.Errorf("db error"))

	_, err := service.GetUserHistoryByDateRange(1, 30, 2000)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetUserHistoryByDateRange_DeletedScene(t *testing.T) {
	service, repo, sceneRepo := newTestWatchHistoryService(t)

	now := time.Now().UTC()
	watches := []data.UserSceneWatch{
		{ID: 1, UserID: 1, SceneID: 10, WatchedAt: now},
		{ID: 2, UserID: 1, SceneID: 99, WatchedAt: now.Add(-time.Hour)},
	}
	// Only scene 10 exists, scene 99 was deleted
	scenes := []data.Scene{
		{ID: 10, Title: "Scene 10"},
	}

	repo.EXPECT().ListUserHistoryByDateRange(uint(1), gomock.Any(), 2000).Return(watches, nil)
	sceneRepo.EXPECT().GetByIDs(gomock.Any()).Return(scenes, nil)

	entries, err := service.GetUserHistoryByDateRange(1, 30, 2000)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Scene == nil {
		t.Fatal("expected first entry to have scene")
	}
	if entries[1].Scene != nil {
		t.Fatal("expected second entry to have nil scene (deleted)")
	}
}

func TestGetDailyActivity_Success(t *testing.T) {
	service, repo, _ := newTestWatchHistoryService(t)

	counts := []data.DailyActivityCount{
		{Date: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Count: 5},
		{Date: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC), Count: 3},
	}

	repo.EXPECT().GetDailyActivityCounts(uint(1), gomock.Any()).Return(counts, nil)

	result, err := service.GetDailyActivity(1, 30)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 counts, got %d", len(result))
	}
	if result[0].Count != 5 {
		t.Fatalf("expected first count to be 5, got %d", result[0].Count)
	}
}

func TestGetDailyActivity_RepoError(t *testing.T) {
	service, repo, _ := newTestWatchHistoryService(t)

	repo.EXPECT().GetDailyActivityCounts(uint(1), gomock.Any()).Return(nil, fmt.Errorf("db error"))

	_, err := service.GetDailyActivity(1, 30)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetDailyActivity_AllTime(t *testing.T) {
	service, repo, _ := newTestWatchHistoryService(t)

	repo.EXPECT().GetDailyActivityCounts(uint(1), gomock.Any()).Return(nil, nil)

	result, err := service.GetDailyActivity(1, 0)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil, got %v", result)
	}
}
