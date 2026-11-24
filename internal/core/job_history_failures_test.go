package core

import (
	"errors"
	"testing"

	"goonhub/internal/config"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

// fakeTracker records the calls of JobHistoryService to the failure tracker.
type fakeTracker struct {
	failing  bool
	recorded []string
	cleared  []string
}

func (f *fakeTracker) RecordFailure(sceneID uint, phase, errMsg string) bool {
	f.recorded = append(f.recorded, phase+":"+errMsg)
	return f.failing
}

func (f *fakeTracker) ClearFailure(sceneID uint, phase string) {
	f.cleared = append(f.cleared, phase)
}

func newTestHistoryWithTracker(t *testing.T, failing bool) (*JobHistoryService, *mocks.MockJobHistoryRepository, *fakeTracker) {
	t.Helper()
	repo := mocks.NewMockJobHistoryRepository(gomock.NewController(t))
	svc := NewJobHistoryService(repo, config.ProcessingConfig{}, zap.NewNop())
	tracker := &fakeTracker{failing: failing}
	svc.SetFailureTracker(tracker)
	return svc, repo, tracker
}

func TestJobHistory_CompleteClearsFailures(t *testing.T) {
	svc, repo, tracker := newTestHistoryWithTracker(t, false)
	repo.EXPECT().UpdateStatus("j1", "completed", nil, gomock.Any()).Return(nil)
	svc.RecordJobComplete("j1", 7, "sprites")
	if len(tracker.cleared) != 1 || tracker.cleared[0] != "sprites" {
		t.Fatalf("cleared %v, want the sprites counter", tracker.cleared)
	}
}

func TestJobHistory_FailureCountsAndFailingStopsRetries(t *testing.T) {
	svc, repo, tracker := newTestHistoryWithTracker(t, true)
	repo.EXPECT().GetByJobID("j1").Return(&data.JobHistory{JobID: "j1", IsRetryable: true}, nil)
	repo.EXPECT().UpdateStatus("j1", data.JobStatusFailed, gomock.Any(), gomock.Any()).Return(nil)
	repo.EXPECT().MarkNotRetryable("j1").Return(nil)
	// A retry scheduler is set: ScheduleRetry would touch the repository (unexpected call)
	svc.SetRetryScheduler(NewRetryScheduler(repo, nil, nil, nil, NewEventBus(zap.NewNop()), zap.NewNop()))

	svc.RecordJobFailedWithRetry("j1", 7, "metadata", errors.New("video file is corrupted"))
	if len(tracker.recorded) != 1 || tracker.recorded[0] != "metadata:video file is corrupted" {
		t.Fatalf("recorded %v", tracker.recorded)
	}
}

func TestJobHistory_FailureBelowThresholdStillRetried(t *testing.T) {
	svc, repo, tracker := newTestHistoryWithTracker(t, false)
	repo.EXPECT().GetByJobID("j1").Return(&data.JobHistory{JobID: "j1", IsRetryable: true, RetryCount: 0}, nil)
	repo.EXPECT().UpdateStatus("j1", data.JobStatusFailed, gomock.Any(), gomock.Any()).Return(nil)
	repo.EXPECT().UpdateRetryInfo("j1", 1, 3, gomock.Any()).Return(nil)
	svc.SetRetryScheduler(NewRetryScheduler(repo, nil, nil, nil, NewEventBus(zap.NewNop()), zap.NewNop()))

	svc.RecordJobFailedWithRetry("j1", 7, "metadata", errors.New("boom"))
	if len(tracker.recorded) != 1 {
		t.Fatalf("the failure must be counted, got %v", tracker.recorded)
	}
}

func TestJobHistory_ShutdownInterruptionsDoNotCount(t *testing.T) {
	svc, repo, tracker := newTestHistoryWithTracker(t, true)
	svc.BeginShutdown()

	var messages []string
	repo.EXPECT().UpdateStatus(gomock.Any(), data.JobStatusFailed, gomock.Any(), gomock.Any()).
		DoAndReturn(func(jobID, status string, msg *string, _ any) error {
			messages = append(messages, *msg)
			return nil
		}).Times(2)

	svc.RecordJobFailedWithRetry("j1", 7, "sprites", errors.New("signal: killed"))
	svc.RecordJobCancelled("j2")

	if len(tracker.recorded) != 0 {
		t.Fatalf("shutdown interruptions must not count, got %v", tracker.recorded)
	}
	for _, m := range messages {
		if m != data.JobErrorInterruptedByShutdown {
			t.Fatalf("recorded %q, want the shutdown message (re-queued at startup)", m)
		}
	}
}

func TestJobHistory_CancellationDoesNotCount(t *testing.T) {
	svc, repo, tracker := newTestHistoryWithTracker(t, true)
	repo.EXPECT().UpdateStatus("j1", data.JobStatusCancelled, gomock.Any(), gomock.Any()).Return(nil)
	svc.RecordJobCancelled("j1")
	if len(tracker.recorded) != 0 || len(tracker.cleared) != 0 {
		t.Fatalf("a cancellation must not touch the counters: %+v", tracker)
	}
}

// fakeRetryChecker answers IsFailing for the retry scheduler.
type fakeRetryChecker struct {
	failing bool
	err     error
}

func (f fakeRetryChecker) IsFailing(uint, string) (bool, error) { return f.failing, f.err }

func TestRetryScheduler_DropsRetryOfFailingScene(t *testing.T) {
	svc, jobRepo, _, _, _ := newTestRetryScheduler(t)
	svc.SetFailingChecker(fakeRetryChecker{failing: true})
	jobRepo.EXPECT().MarkNotRetryable("j1").Return(nil)
	// No processing service: reaching the resubmission would only log, the expectation above
	// checks that the retry was dropped before
	svc.retryJob(data.JobHistory{JobID: "j1", SceneID: 7, Phase: "sprites"})
}

func TestRetryScheduler_RetriesWhenNotFailing(t *testing.T) {
	svc, jobRepo, _, _, _ := newTestRetryScheduler(t)
	svc.SetFailingChecker(fakeRetryChecker{failing: false})
	jobRepo.EXPECT().MarkNotRetryable(gomock.Any()).Times(0)
	svc.retryJob(data.JobHistory{JobID: "j1", SceneID: 7, Phase: "sprites"})

	svc.SetFailingChecker(fakeRetryChecker{err: errors.New("db down")})
	svc.retryJob(data.JobHistory{JobID: "j1", SceneID: 7, Phase: "sprites"})
}
