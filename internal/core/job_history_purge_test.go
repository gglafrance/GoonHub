package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"goonhub/internal/config"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

var purgeTestNow = time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)

func newPurgeTestService(t *testing.T, cfg config.ProcessingConfig) (*JobHistoryService, *mocks.MockJobHistoryRepository) {
	t.Helper()
	repo := mocks.NewMockJobHistoryRepository(gomock.NewController(t))
	svc := NewJobHistoryService(repo, cfg, zap.NewNop())
	svc.now = func() time.Time { return purgeTestNow }
	return svc, repo
}

func TestHistoryPurgeSettings(t *testing.T) {
	tests := []struct {
		name          string
		cfg           config.ProcessingConfig
		completed     string
		failed        string
		completedAge  time.Duration
		failedAge     time.Duration
		interval      string
		batchSize     int
		completedOn   bool
		failedEnabled bool
	}{
		{
			name:      "defaults when unset",
			cfg:       config.ProcessingConfig{},
			completed: "30d", failed: "90d",
			completedAge: 30 * 24 * time.Hour, failedAge: 90 * 24 * time.Hour,
			interval: "1h", batchSize: 5000, completedOn: true, failedEnabled: true,
		},
		{
			name: "configured values",
			cfg: config.ProcessingConfig{
				JobHistoryRetention: "7d", JobHistoryFailedRetention: "36h",
				JobHistoryPurgeInterval: 30 * time.Minute, JobHistoryPurgeBatchSize: 100,
			},
			completed: "7d", failed: "36h",
			completedAge: 7 * 24 * time.Hour, failedAge: 36 * time.Hour,
			interval: "30m", batchSize: 100, completedOn: true, failedEnabled: true,
		},
		{
			name:      "zero disables a group",
			cfg:       config.ProcessingConfig{JobHistoryRetention: "0", JobHistoryFailedRetention: "0d"},
			completed: "0", failed: "0d",
			interval: "1h", batchSize: 5000,
		},
		{
			name:      "invalid values fall back to the defaults",
			cfg:       config.ProcessingConfig{JobHistoryRetention: "soon", JobHistoryFailedRetention: "xd", JobHistoryPurgeBatchSize: -1},
			completed: "30d", failed: "90d",
			completedAge: 30 * 24 * time.Hour, failedAge: 90 * 24 * time.Hour,
			interval: "1h", batchSize: 5000, completedOn: true, failedEnabled: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newHistoryPurgeSettings(tt.cfg, zap.NewNop())
			if s.CompletedRetention != tt.completed || s.FailedRetention != tt.failed {
				t.Fatalf("retentions = %q/%q, want %q/%q", s.CompletedRetention, s.FailedRetention, tt.completed, tt.failed)
			}
			if s.completedAge != tt.completedAge || s.failedAge != tt.failedAge {
				t.Fatalf("ages = %v/%v, want %v/%v", s.completedAge, s.failedAge, tt.completedAge, tt.failedAge)
			}
			if s.CompletedEnabled != tt.completedOn || s.FailedEnabled != tt.failedEnabled {
				t.Fatalf("enabled = %v/%v, want %v/%v", s.CompletedEnabled, s.FailedEnabled, tt.completedOn, tt.failedEnabled)
			}
			if s.Interval != tt.interval || s.BatchSize != tt.batchSize {
				t.Fatalf("interval/batch = %q/%d, want %q/%d", s.Interval, s.BatchSize, tt.interval, tt.batchSize)
			}
		})
	}
}

func TestPurgeHistory_DeletesInBatchesPerGroup(t *testing.T) {
	svc, repo := newPurgeTestService(t, config.ProcessingConfig{JobHistoryPurgeBatchSize: 2})
	completedBefore := purgeTestNow.Add(-30 * 24 * time.Hour)
	failedBefore := purgeTestNow.Add(-90 * 24 * time.Hour)
	completed := []string{data.JobStatusCompleted, data.JobStatusCancelled}
	failed := []string{data.JobStatusFailed, data.JobStatusTimedOut}

	gomock.InOrder(
		repo.EXPECT().PurgeFinishedBatch(completed, completedBefore, 2).Return(int64(2), nil),
		repo.EXPECT().PurgeFinishedBatch(completed, completedBefore, 2).Return(int64(2), nil),
		repo.EXPECT().PurgeFinishedBatch(completed, completedBefore, 2).Return(int64(1), nil),
		repo.EXPECT().PurgeFinishedBatch(failed, failedBefore, 2).Return(int64(0), nil),
	)

	result, err := svc.PurgeHistory(context.Background(), HistoryPurgeManual)
	if err != nil {
		t.Fatalf("PurgeHistory: %v", err)
	}
	if result.CompletedDeleted != 5 || result.FailedDeleted != 0 || result.Total != 5 || result.Batches != 4 {
		t.Fatalf("result = %+v, want 5 completed, 0 failed, 4 batches", result)
	}
	if result.Trigger != HistoryPurgeManual {
		t.Fatalf("trigger = %q", result.Trigger)
	}
	last := svc.LastPurge()
	if last == nil || last.Total != 5 || last.Error != "" {
		t.Fatalf("last purge = %+v", last)
	}
}

func TestPurgeHistory_DisabledGroupIsSkipped(t *testing.T) {
	svc, repo := newPurgeTestService(t, config.ProcessingConfig{JobHistoryRetention: "0", JobHistoryFailedRetention: "10d"})
	failed := []string{data.JobStatusFailed, data.JobStatusTimedOut}
	repo.EXPECT().PurgeFinishedBatch(failed, purgeTestNow.Add(-10*24*time.Hour), 5000).Return(int64(3), nil)

	result, err := svc.PurgeHistory(context.Background(), HistoryPurgeScheduled)
	if err != nil {
		t.Fatalf("PurgeHistory: %v", err)
	}
	if result.CompletedDeleted != 0 || result.FailedDeleted != 3 {
		t.Fatalf("result = %+v", result)
	}
}

func TestPurgeHistory_ErrorIsReportedAndRecorded(t *testing.T) {
	svc, repo := newPurgeTestService(t, config.ProcessingConfig{JobHistoryPurgeBatchSize: 2})
	gomock.InOrder(
		repo.EXPECT().PurgeFinishedBatch(gomock.Any(), gomock.Any(), 2).Return(int64(2), nil),
		repo.EXPECT().PurgeFinishedBatch(gomock.Any(), gomock.Any(), 2).Return(int64(0), errors.New("lock timeout")),
	)

	result, err := svc.PurgeHistory(context.Background(), HistoryPurgeManual)
	if err == nil {
		t.Fatal("expected an error")
	}
	if result.CompletedDeleted != 2 {
		t.Fatalf("partial result = %+v, want the 2 rows of the first batch", result)
	}
	if last := svc.LastPurge(); last == nil || last.Error == "" {
		t.Fatalf("last purge = %+v, want the error recorded", last)
	}
}

func TestPurgeHistory_StopsWhenContextCancelled(t *testing.T) {
	svc, repo := newPurgeTestService(t, config.ProcessingConfig{JobHistoryPurgeBatchSize: 2})
	ctx, cancel := context.WithCancel(context.Background())
	repo.EXPECT().PurgeFinishedBatch(gomock.Any(), gomock.Any(), 2).DoAndReturn(
		func([]string, time.Time, int) (int64, error) {
			cancel()
			return 2, nil
		})

	if _, err := svc.PurgeHistory(ctx, HistoryPurgeScheduled); err == nil {
		t.Fatal("expected the purge to stop with an error")
	}
}

func TestPreviewPurge(t *testing.T) {
	svc, repo := newPurgeTestService(t, config.ProcessingConfig{})
	repo.EXPECT().CountPurgeable([]string{data.JobStatusCompleted, data.JobStatusCancelled}, purgeTestNow.Add(-30*24*time.Hour)).
		Return(map[string]int64{data.JobStatusCompleted: 40000, data.JobStatusCancelled: 30931}, nil)
	repo.EXPECT().CountPurgeable([]string{data.JobStatusFailed, data.JobStatusTimedOut}, purgeTestNow.Add(-90*24*time.Hour)).
		Return(map[string]int64{data.JobStatusFailed: 12}, nil)

	preview, err := svc.PreviewPurge()
	if err != nil {
		t.Fatalf("PreviewPurge: %v", err)
	}
	if preview.Completed != 40000 || preview.Cancelled != 30931 || preview.Failed != 12 || preview.TimedOut != 0 {
		t.Fatalf("preview = %+v", preview)
	}
	if preview.Total != 70943 {
		t.Fatalf("total = %d, want 70943", preview.Total)
	}
	if preview.CompletedBefore == nil || preview.FailedBefore == nil {
		t.Fatal("cutoff dates missing")
	}
}

func TestPreviewPurge_Error(t *testing.T) {
	svc, repo := newPurgeTestService(t, config.ProcessingConfig{})
	repo.EXPECT().CountPurgeable(gomock.Any(), gomock.Any()).Return(nil, errors.New("db down"))
	if _, err := svc.PreviewPurge(); err == nil {
		t.Fatal("expected an error")
	}
}
