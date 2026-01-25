package core

import (
	"errors"
	"goonhub/internal/data"
	"goonhub/internal/mocks"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func newTestDLQService(t *testing.T) (*DLQService, *mocks.MockDLQRepository, *mocks.MockJobHistoryRepository, *mocks.MockVideoRepository) {
	ctrl := gomock.NewController(t)
	dlqRepo := mocks.NewMockDLQRepository(ctrl)
	jobHistoryRepo := mocks.NewMockJobHistoryRepository(ctrl)
	videoRepo := mocks.NewMockVideoRepository(ctrl)

	eventBus := NewEventBus(zap.NewNop())

	svc := NewDLQService(dlqRepo, jobHistoryRepo, videoRepo, eventBus, zap.NewNop())
	return svc, dlqRepo, jobHistoryRepo, videoRepo
}

func TestDLQService_ListPending(t *testing.T) {
	svc, dlqRepo, _, _ := newTestDLQService(t)

	expected := []data.DLQEntry{
		{ID: 1, JobID: "job-1", VideoID: 1, Phase: "metadata", Status: "pending_review"},
		{ID: 2, JobID: "job-2", VideoID: 2, Phase: "sprites", Status: "pending_review"},
	}
	dlqRepo.EXPECT().ListPending(1, 50).Return(expected, int64(2), nil)

	entries, total, err := svc.ListPending(1, 50)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}

func TestDLQService_ListByStatus(t *testing.T) {
	svc, dlqRepo, _, _ := newTestDLQService(t)

	expected := []data.DLQEntry{
		{ID: 1, JobID: "job-1", VideoID: 1, Phase: "metadata", Status: "abandoned"},
	}
	dlqRepo.EXPECT().ListByStatus("abandoned", 1, 50).Return(expected, int64(1), nil)

	entries, total, err := svc.ListByStatus("abandoned", 1, 50)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	if entries[0].Status != "abandoned" {
		t.Fatalf("expected status 'abandoned', got %q", entries[0].Status)
	}
}

func TestDLQService_Abandon(t *testing.T) {
	svc, dlqRepo, _, _ := newTestDLQService(t)

	dlqRepo.EXPECT().GetByJobID("job-123").Return(&data.DLQEntry{
		JobID:   "job-123",
		VideoID: 1,
		Phase:   "metadata",
	}, nil)
	dlqRepo.EXPECT().MarkAbandoned("job-123").Return(nil)

	err := svc.Abandon("job-123")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestDLQService_Abandon_NotFound(t *testing.T) {
	svc, dlqRepo, _, _ := newTestDLQService(t)

	dlqRepo.EXPECT().GetByJobID("job-123").Return(nil, errors.New("not found"))

	err := svc.Abandon("job-123")
	if err == nil {
		t.Fatal("expected error for not found entry")
	}
}

func TestDLQService_GetStats(t *testing.T) {
	svc, dlqRepo, _, _ := newTestDLQService(t)

	dlqRepo.EXPECT().CountByStatus("pending_review").Return(int64(5), nil)
	dlqRepo.EXPECT().CountByStatus("retrying").Return(int64(2), nil)
	dlqRepo.EXPECT().CountByStatus("abandoned").Return(int64(10), nil)

	stats, err := svc.GetStats()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if stats["pending_review"] != 5 {
		t.Fatalf("expected pending_review 5, got %d", stats["pending_review"])
	}
	if stats["retrying"] != 2 {
		t.Fatalf("expected retrying 2, got %d", stats["retrying"])
	}
	if stats["abandoned"] != 10 {
		t.Fatalf("expected abandoned 10, got %d", stats["abandoned"])
	}
	if stats["total"] != 17 {
		t.Fatalf("expected total 17, got %d", stats["total"])
	}
}

func TestDLQService_GetByJobID(t *testing.T) {
	svc, dlqRepo, _, _ := newTestDLQService(t)

	expected := &data.DLQEntry{
		ID:            1,
		JobID:         "job-123",
		VideoID:       1,
		VideoTitle:    "Test Video",
		Phase:         "sprites",
		OriginalError: "ffmpeg failed",
		FailureCount:  3,
		LastError:     "ffmpeg failed",
		Status:        "pending_review",
		CreatedAt:     time.Now(),
	}
	dlqRepo.EXPECT().GetByJobID("job-123").Return(expected, nil)

	entry, err := svc.GetByJobID("job-123")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if entry.JobID != "job-123" {
		t.Fatalf("expected job_id 'job-123', got %q", entry.JobID)
	}
	if entry.FailureCount != 3 {
		t.Fatalf("expected failure_count 3, got %d", entry.FailureCount)
	}
}

func TestDLQService_RetryFromDLQ_NoProcessingService(t *testing.T) {
	svc, _, _, _ := newTestDLQService(t)
	// processingService is nil by default

	err := svc.RetryFromDLQ("job-123")
	if err == nil {
		t.Fatal("expected error when processing service not configured")
	}
}
