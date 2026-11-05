package processing

import (
	"testing"

	"goonhub/internal/data"
	"goonhub/internal/jobs"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

type processingTestEvents struct {
	events []SceneEvent
}

func (events *processingTestEvents) Publish(event SceneEvent) {
	events.events = append(events.events, event)
}

func TestThumbnailCompletesSceneWithFingerprintDisabled(t *testing.T) {
	for _, after := range []string{"metadata", "thumbnail"} {
		t.Run(after, func(t *testing.T) {
			tracker := NewPhaseTracker(nil, false)
			metadata := "metadata"
			tracker.triggerCache = []data.TriggerConfigRecord{
				{Phase: "thumbnail", TriggerType: "after_job", AfterPhase: &metadata},
				{Phase: "fingerprint", TriggerType: "after_job", AfterPhase: &after},
			}
			tracker.InitPhaseState(5)
			repo := mocks.NewMockSceneRepository(gomock.NewController(t))
			repo.EXPECT().UpdateProcessingStatus(uint(5), "completed", "").Return(nil)
			events := &processingTestEvents{}
			handler := NewResultHandler(repo, nil, events, nil, tracker, nil, zap.NewNop())
			handler.SetOnPhaseComplete(func(uint, string) error {
				t.Fatal("disabled fingerprint phase must not be submitted")
				return nil
			})
			handler.onThumbnailComplete(jobs.JobResult{SceneID: 5})
			if len(events.events) != 1 || events.events[0].Type != "scene:completed" || events.events[0].SceneID != 5 {
				t.Fatalf("expected scene completion event, got %+v", events.events)
			}
		})
	}
}

func TestManualFingerprintSubmissionStillRejectsDisabledFeature(t *testing.T) {
	submitter := NewJobSubmitter(nil, &PoolManager{}, NewPhaseTracker(nil, false), nil, zap.NewNop())
	if err := submitter.SubmitPhase(5, "fingerprint"); err == nil {
		t.Fatal("manual fingerprint request must report that the feature is disabled")
	}
}
