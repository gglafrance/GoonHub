package core

import (
	"testing"

	"goonhub/internal/core/processing"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"github.com/robfig/cron/v3"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestTriggerSchedulerSkipsDisabledFingerprint(t *testing.T) {
	ctrl := gomock.NewController(t)
	triggers := mocks.NewMockTriggerConfigRepository(ctrl)
	scenes := mocks.NewMockSceneRepository(ctrl)
	expression := "* * * * *"
	triggers.EXPECT().GetAll().Return([]data.TriggerConfigRecord{
		{Phase: "fingerprint", TriggerType: "scheduled", CronExpression: &expression},
		{Phase: "metadata", TriggerType: "scheduled", CronExpression: &expression},
	}, nil)
	service := &SceneProcessingService{poolManager: &processing.PoolManager{}}
	scheduler := NewTriggerScheduler(triggers, scenes, service, zap.NewNop())
	scheduler.cron = cron.New()
	if err := scheduler.loadSchedules(); err != nil {
		t.Fatal(err)
	}
	if len(scheduler.entryIDs) != 1 {
		t.Fatalf("expected only the metadata schedule, got %d entries", len(scheduler.entryIDs))
	}
	// No scene-repository calls are expected for a disabled scheduled phase.
	scheduler.runScheduledPhase("fingerprint")
}
