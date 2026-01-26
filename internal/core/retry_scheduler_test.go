package core

import (
	"goonhub/internal/data"
	"goonhub/internal/mocks"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func newTestRetryScheduler(t *testing.T) (*RetryScheduler, *mocks.MockJobHistoryRepository, *mocks.MockDLQRepository, *mocks.MockRetryConfigRepository, *mocks.MockVideoRepository) {
	ctrl := gomock.NewController(t)
	jobHistoryRepo := mocks.NewMockJobHistoryRepository(ctrl)
	dlqRepo := mocks.NewMockDLQRepository(ctrl)
	retryConfigRepo := mocks.NewMockRetryConfigRepository(ctrl)
	videoRepo := mocks.NewMockVideoRepository(ctrl)

	eventBus := NewEventBus(zap.NewNop())

	svc := NewRetryScheduler(jobHistoryRepo, dlqRepo, retryConfigRepo, videoRepo, eventBus, zap.NewNop())
	return svc, jobHistoryRepo, dlqRepo, retryConfigRepo, videoRepo
}

func TestRetryScheduler_CalculateNextRetry_DefaultConfig(t *testing.T) {
	svc, _, _, retryConfigRepo, _ := newTestRetryScheduler(t)

	// Return empty config, will use defaults
	retryConfigRepo.EXPECT().GetAll().Return([]data.RetryConfigRecord{}, nil)
	if err := svc.refreshConfigCache(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// First retry (count=0)
	nextRetry := svc.CalculateNextRetryTime("metadata", 0)
	now := time.Now()
	expectedMinDelay := 30 * time.Second // Default initial delay

	if nextRetry.Before(now.Add(expectedMinDelay - time.Second)) {
		t.Fatalf("expected delay of at least %v, got %v", expectedMinDelay, nextRetry.Sub(now))
	}

	// Second retry (count=1) should have exponential backoff
	nextRetry2 := svc.CalculateNextRetryTime("metadata", 1)
	expectedMinDelay2 := 60 * time.Second // 30 * 2.0^1 = 60

	if nextRetry2.Before(now.Add(expectedMinDelay2 - time.Second)) {
		t.Fatalf("expected delay of at least %v for second retry, got %v", expectedMinDelay2, nextRetry2.Sub(now))
	}
}

func TestRetryScheduler_CalculateNextRetry_CustomConfig(t *testing.T) {
	svc, _, _, retryConfigRepo, _ := newTestRetryScheduler(t)

	// Return custom config
	retryConfigRepo.EXPECT().GetAll().Return([]data.RetryConfigRecord{
		{
			Phase:               "sprites",
			MaxRetries:          5,
			InitialDelaySeconds: 120,
			MaxDelaySeconds:     7200,
			BackoffFactor:       2.5,
		},
	}, nil)
	if err := svc.refreshConfigCache(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// First retry for sprites
	nextRetry := svc.CalculateNextRetryTime("sprites", 0)
	now := time.Now()
	expectedMinDelay := 120 * time.Second

	if nextRetry.Before(now.Add(expectedMinDelay - time.Second)) {
		t.Fatalf("expected delay of at least %v, got %v", expectedMinDelay, nextRetry.Sub(now))
	}
}

func TestRetryScheduler_CalculateNextRetry_MaxDelayRespected(t *testing.T) {
	svc, _, _, retryConfigRepo, _ := newTestRetryScheduler(t)

	// Return config with low max delay
	retryConfigRepo.EXPECT().GetAll().Return([]data.RetryConfigRecord{
		{
			Phase:               "metadata",
			MaxRetries:          10,
			InitialDelaySeconds: 30,
			MaxDelaySeconds:     60, // Low max
			BackoffFactor:       3.0,
		},
	}, nil)
	if err := svc.refreshConfigCache(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// High retry count would exceed max delay
	nextRetry := svc.CalculateNextRetryTime("metadata", 5)
	now := time.Now()
	maxDelay := 60 * time.Second

	// Delay should not exceed max
	if nextRetry.After(now.Add(maxDelay + time.Second)) {
		t.Fatalf("expected delay to be capped at %v, got %v", maxDelay, nextRetry.Sub(now))
	}
}

func TestRetryScheduler_ScheduleRetry_WithinMaxRetries(t *testing.T) {
	svc, jobHistoryRepo, _, retryConfigRepo, _ := newTestRetryScheduler(t)

	// Setup config
	retryConfigRepo.EXPECT().GetAll().Return([]data.RetryConfigRecord{
		{Phase: "metadata", MaxRetries: 3, InitialDelaySeconds: 30, MaxDelaySeconds: 3600, BackoffFactor: 2.0},
	}, nil)
	if err := svc.refreshConfigCache(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Expect retry info update
	jobHistoryRepo.EXPECT().UpdateRetryInfo("job-123", 1, 3, gomock.Any()).Return(nil)

	// Schedule retry for first failure (count=0)
	err := svc.ScheduleRetry("job-123", "metadata", 1, 0, "test error")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestRetryScheduler_ScheduleRetry_ExhaustedRetries(t *testing.T) {
	svc, jobHistoryRepo, dlqRepo, retryConfigRepo, videoRepo := newTestRetryScheduler(t)

	// Setup config
	retryConfigRepo.EXPECT().GetAll().Return([]data.RetryConfigRecord{
		{Phase: "metadata", MaxRetries: 3, InitialDelaySeconds: 30, MaxDelaySeconds: 3600, BackoffFactor: 2.0},
	}, nil)
	if err := svc.refreshConfigCache(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Expect update of retry info before DLQ move (with nil nextRetryAt)
	jobHistoryRepo.EXPECT().UpdateRetryInfo("job-123", 3, 3, nil).Return(nil)

	// Expect move to DLQ
	jobHistoryRepo.EXPECT().MarkNotRetryable("job-123").Return(nil)
	videoRepo.EXPECT().GetByID(uint(1)).Return(&data.Video{ID: 1, Title: "Test Video"}, nil)
	dlqRepo.EXPECT().Create(gomock.Any()).Return(nil)

	// Schedule retry when next attempt would exceed max (count=2, so count+1=3 >= max_retries=3)
	err := svc.ScheduleRetry("job-123", "metadata", 1, 2, "test error")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestRetryScheduler_GetConfigForPhase_DefaultValues(t *testing.T) {
	svc, _, _, retryConfigRepo, _ := newTestRetryScheduler(t)

	// Empty config
	retryConfigRepo.EXPECT().GetAll().Return([]data.RetryConfigRecord{}, nil)
	if err := svc.refreshConfigCache(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	cfg := svc.GetConfigForPhase("unknown_phase")

	if cfg.MaxRetries != 3 {
		t.Fatalf("expected default max_retries 3, got %d", cfg.MaxRetries)
	}
	if cfg.InitialDelaySeconds != 30 {
		t.Fatalf("expected default initial_delay_seconds 30, got %d", cfg.InitialDelaySeconds)
	}
	if cfg.BackoffFactor != 2.0 {
		t.Fatalf("expected default backoff_factor 2.0, got %f", cfg.BackoffFactor)
	}
}

func TestRetryScheduler_RefreshConfigCache(t *testing.T) {
	svc, _, _, retryConfigRepo, _ := newTestRetryScheduler(t)

	// First load
	retryConfigRepo.EXPECT().GetAll().Return([]data.RetryConfigRecord{
		{Phase: "metadata", MaxRetries: 5},
	}, nil)
	if err := svc.RefreshConfigCache(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	cfg := svc.GetConfigForPhase("metadata")
	if cfg.MaxRetries != 5 {
		t.Fatalf("expected max_retries 5, got %d", cfg.MaxRetries)
	}

	// Update config
	retryConfigRepo.EXPECT().GetAll().Return([]data.RetryConfigRecord{
		{Phase: "metadata", MaxRetries: 10},
	}, nil)
	if err := svc.RefreshConfigCache(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	cfg = svc.GetConfigForPhase("metadata")
	if cfg.MaxRetries != 10 {
		t.Fatalf("expected max_retries 10 after refresh, got %d", cfg.MaxRetries)
	}
}
