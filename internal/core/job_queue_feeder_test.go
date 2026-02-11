package core

import (
	"goonhub/internal/config"
	"goonhub/internal/core/processing"
	"goonhub/internal/data"
	"goonhub/internal/mocks"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func newTestFeeder(t *testing.T) (*JobQueueFeeder, *mocks.MockJobHistoryRepository, *mocks.MockSceneRepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	jobHistoryRepo := mocks.NewMockJobHistoryRepository(ctrl)
	sceneRepo := mocks.NewMockSceneRepository(ctrl)

	tmpDir := t.TempDir()
	cfg := config.ProcessingConfig{
		ThumbnailDir:          tmpDir,
		SpriteDir:             tmpDir,
		VttDir:                tmpDir,
		MetadataWorkers:       1,
		ThumbnailWorkers:      1,
		SpritesWorkers:        1,
		MaxFrameDimension:     320,
		MaxFrameDimensionLarge: 960,
		FrameQuality:          75,
		FrameQualityLg:        85,
		FrameQualitySprites:   60,
		SpritesConcurrency:    2,
		FrameInterval:         5,
		GridCols:              5,
		GridRows:              5,
	}

	poolManager := processing.NewPoolManager(cfg, zap.NewNop(), nil, nil)

	feeder := NewJobQueueFeeder(jobHistoryRepo, sceneRepo, nil, nil, poolManager, zap.NewNop())
	return feeder, jobHistoryRepo, sceneRepo
}

func TestSubmitJobToPool_ThumbnailDurationZero(t *testing.T) {
	feeder, _, _ := newTestFeeder(t)

	jobRecord := data.JobHistory{
		JobID:   "test-job-1",
		SceneID: 1,
		Phase:   "thumbnail",
	}
	scene := &data.Scene{
		ID:       1,
		Duration: 0, // metadata not yet extracted
		Width:    1920,
		Height:   1080,
	}

	err := feeder.submitJobToPool(jobRecord, scene)
	if err == nil {
		t.Fatal("expected error when scene duration is 0 for thumbnail job")
	}
	if !strings.Contains(err.Error(), "scene duration is 0") {
		t.Fatalf("expected error about duration being 0, got: %v", err)
	}
}

func TestSubmitJobToPool_SpritesDurationZero(t *testing.T) {
	feeder, _, _ := newTestFeeder(t)

	jobRecord := data.JobHistory{
		JobID:   "test-job-2",
		SceneID: 2,
		Phase:   "sprites",
	}
	scene := &data.Scene{
		ID:       2,
		Duration: 0, // metadata not yet extracted
		Width:    1920,
		Height:   1080,
	}

	err := feeder.submitJobToPool(jobRecord, scene)
	if err == nil {
		t.Fatal("expected error when scene duration is 0 for sprites job")
	}
	if !strings.Contains(err.Error(), "scene duration is 0") {
		t.Fatalf("expected error about duration being 0, got: %v", err)
	}
}

func TestSubmitJobToPool_AnimatedThumbnailsDurationZero(t *testing.T) {
	feeder, _, _ := newTestFeeder(t)

	jobRecord := data.JobHistory{
		JobID:   "test-job-anim-1",
		SceneID: 10,
		Phase:   "animated_thumbnails",
	}
	scene := &data.Scene{
		ID:       10,
		Duration: 0,
		Width:    1920,
		Height:   1080,
	}

	err := feeder.submitJobToPool(jobRecord, scene)
	if err == nil {
		t.Fatal("expected error when scene duration is 0 for animated_thumbnails job")
	}
	if !strings.Contains(err.Error(), "scene duration is 0") {
		t.Fatalf("expected error about duration being 0, got: %v", err)
	}
}

func TestSubmitJobToPool_ThumbnailWithDuration(t *testing.T) {
	feeder, _, _ := newTestFeeder(t)

	// Start the pool so submissions work
	feeder.poolManager.Start()
	defer feeder.poolManager.Stop()

	jobRecord := data.JobHistory{
		JobID:   "test-job-3",
		SceneID: 3,
		Phase:   "thumbnail",
	}
	scene := &data.Scene{
		ID:       3,
		Duration: 120.0,
		Width:    1920,
		Height:   1080,
	}

	err := feeder.submitJobToPool(jobRecord, scene)
	if err != nil {
		t.Fatalf("expected no error for thumbnail job with valid duration, got: %v", err)
	}
}

func TestSubmitJobToPool_SpritesWithDuration(t *testing.T) {
	feeder, _, _ := newTestFeeder(t)

	// Start the pool so submissions work
	feeder.poolManager.Start()
	defer feeder.poolManager.Stop()

	jobRecord := data.JobHistory{
		JobID:   "test-job-4",
		SceneID: 4,
		Phase:   "sprites",
	}
	scene := &data.Scene{
		ID:       4,
		Duration: 120.0,
		Width:    1920,
		Height:   1080,
	}

	err := feeder.submitJobToPool(jobRecord, scene)
	if err != nil {
		t.Fatalf("expected no error for sprites job with valid duration, got: %v", err)
	}
}
