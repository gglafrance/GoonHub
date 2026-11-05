package processing

import (
	"reflect"
	"testing"

	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
)

func TestPhaseTrackerFiltersDisabledFingerprintTriggers(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[enabled], func(t *testing.T) {
			for _, after := range []string{"metadata", "thumbnail"} {
				t.Run(after, func(t *testing.T) {
					repo := mocks.NewMockTriggerConfigRepository(gomock.NewController(t))
					repo.EXPECT().GetAll().Return([]data.TriggerConfigRecord{
						{Phase: "sprites", TriggerType: "after_job", AfterPhase: &after},
						{Phase: "fingerprint", TriggerType: "after_job", AfterPhase: &after},
					}, nil)
					tracker := NewPhaseTracker(repo, enabled)
					if err := tracker.RefreshTriggerCache(); err != nil {
						t.Fatal(err)
					}
					want := []string{"sprites"}
					if enabled {
						want = append(want, "fingerprint")
					}
					if got := tracker.GetPhasesTriggeredAfter(after); !reflect.DeepEqual(got, want) {
						t.Fatalf("got triggers %v, want %v", got, want)
					}
					if tracker.ShouldAutoDispatch("fingerprint") != enabled {
						t.Fatal("fingerprint auto-dispatch must honor the feature flag")
					}
					if tracker.GetTriggerForPhase("fingerprint") == nil {
						t.Fatal("disabling the feature must preserve the saved trigger configuration")
					}
				})
			}
		})
	}
}

func TestPhaseTrackerCompletesWithoutDisabledFingerprint(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[enabled], func(t *testing.T) {
			tracker := NewPhaseTracker(nil, enabled)
			after := "metadata"
			tracker.triggerCache = []data.TriggerConfigRecord{
				{Phase: "thumbnail", TriggerType: "after_job", AfterPhase: &after},
				{Phase: "fingerprint", TriggerType: "after_job", AfterPhase: &after},
			}
			tracker.InitPhaseState(5)
			tracker.MarkPhaseComplete(5, "thumbnail")
			if got := tracker.CheckAllPhasesComplete(5, "thumbnail"); got == enabled {
				t.Fatalf("thumbnail completion=%v with duplication enabled=%v", got, enabled)
			}
			if enabled {
				tracker.MarkPhaseComplete(5, "fingerprint")
				if !tracker.CheckAllPhasesComplete(5, "fingerprint") {
					t.Fatal("pipeline must finish after enabled fingerprinting completes")
				}
			}
			if _, exists := tracker.GetPhaseState(5); exists {
				t.Fatal("completed phase state must be cleared")
			}
		})
	}
}

func TestPhaseTrackerMetadataOnlyWithDisabledFingerprint(t *testing.T) {
	after := "metadata"
	tracker := NewPhaseTracker(nil, false)
	tracker.triggerCache = []data.TriggerConfigRecord{
		{Phase: "fingerprint", TriggerType: "after_job", AfterPhase: &after},
	}
	if !tracker.CheckAllPhasesComplete(5, "metadata") {
		t.Fatal("metadata must complete a pipeline whose only follow-up is disabled")
	}
}

func TestPhaseTrackerDefaultDispatchHonorsFeatureFlag(t *testing.T) {
	tracker := NewPhaseTracker(nil, false)
	if tracker.ShouldAutoDispatch("fingerprint") {
		t.Fatal("missing trigger config must not enable fingerprinting")
	}
	if !tracker.ShouldAutoDispatch("metadata") || !tracker.ShouldAutoDispatch("thumbnail") {
		t.Fatal("default dispatch for other phases must be preserved")
	}
}
