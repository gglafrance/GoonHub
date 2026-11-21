package core

import (
	"errors"
	"goonhub/internal/apperrors"
	"goonhub/internal/config"
	"goonhub/internal/core/processing"
	"goonhub/internal/data"
	"goonhub/internal/mocks"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func newSchedulingTestPoolManager(t *testing.T) *processing.PoolManager {
	t.Helper()
	dir := t.TempDir()
	return processing.NewPoolManager(config.ProcessingConfig{
		ThumbnailDir:           dir,
		SpriteDir:              dir,
		VttDir:                 dir,
		MetadataWorkers:        2,
		ThumbnailWorkers:       2,
		SpritesWorkers:         3,
		MaxFrameDimension:      320,
		MaxFrameDimensionLarge: 960,
		FrameQuality:           75,
		FrameQualityLg:         85,
		FrameQualitySprites:    60,
		FrameInterval:          5,
		GridCols:               5,
		GridRows:               5,
	}, zap.NewNop(), nil, nil, nil)
}

func TestJobSchedulingService_LoadsSettingsFromDB(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockJobScheduleRepository(ctrl)
	repo.EXPECT().GetConfig().Return(&data.JobScheduleConfigRecord{
		ID: 1, Timezone: "Europe/Paris", QuietHoursStart: "22:00", QuietHoursEnd: "06:00",
		MaxConcurrentJobs: 6, NewScenesFirst: true, NewScenesWindowHours: 12,
	}, nil)
	repo.EXPECT().ListPhases().Return([]data.JobPhaseScheduleRecord{
		{Phase: "fingerprint", Priority: 5, WindowEnabled: true, WindowStart: "01:00", WindowEnd: "07:00", QuietWorkers: 12},
	}, nil)

	s := NewJobSchedulingService(repo, nil, nil, zap.NewNop())
	cfg := s.GetConfig()
	if cfg.Timezone != "Europe/Paris" || cfg.MaxConcurrentJobs != 6 || !cfg.NewScenesFirst || cfg.NewScenesWindowHours != 12 {
		t.Fatalf("global settings not loaded: %+v", cfg)
	}
	if len(cfg.Phases) != len(SchedulablePhases) {
		t.Fatalf("expected all phases (missing ones defaulted), got %d", len(cfg.Phases))
	}
	fp := cfg.Phases[4]
	if fp.Phase != "fingerprint" || fp.Priority != 5 || !fp.WindowEnabled || fp.QuietWorkers != 12 {
		t.Fatalf("fingerprint settings not loaded: %+v", fp)
	}
	// 10:00 UTC = 11:00 Paris: fingerprint outside 01:00-07:00
	if s.Plan(time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)).Phase("fingerprint").Allowed {
		t.Fatal("fingerprint should be paused at 11:00 Paris")
	}
}

func TestJobSchedulingService_FallsBackToDefaults(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockJobScheduleRepository(ctrl)
	repo.EXPECT().GetConfig().Return(nil, errors.New("db down"))

	s := NewJobSchedulingService(repo, nil, nil, zap.NewNop())
	if got := s.GetConfig(); got.Timezone != "UTC" || len(got.Phases) != len(SchedulablePhases) {
		t.Fatalf("expected defaults, got %+v", got)
	}
}

func TestJobSchedulingService_UpdateValidatesAndSaves(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockJobScheduleRepository(ctrl)
	repo.EXPECT().GetConfig().Return(nil, nil)
	repo.EXPECT().ListPhases().Return(nil, nil)
	s := NewJobSchedulingService(repo, nil, nil, zap.NewNop())

	bad := DefaultJobScheduleConfig()
	bad.Timezone = "Nowhere/City"
	if _, err := s.UpdateConfig(bad); !apperrors.IsValidation(err) {
		t.Fatalf("expected a validation error, got %v", err)
	}

	good := DefaultJobScheduleConfig()
	good.MaxConcurrentJobs = 4
	setPhase(&good, "sprites", func(p *PhaseScheduleConfig) {
		p.WindowEnabled = true
		p.WindowStart = "01:00"
		p.WindowEnd = "07:00"
	})
	repo.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(
		func(cfg *data.JobScheduleConfigRecord, phases []data.JobPhaseScheduleRecord) error {
			if cfg.MaxConcurrentJobs != 4 || cfg.Timezone != "UTC" {
				t.Fatalf("unexpected global record: %+v", cfg)
			}
			if len(phases) != len(SchedulablePhases) {
				t.Fatalf("expected %d phase records, got %d", len(SchedulablePhases), len(phases))
			}
			if phases[2].Phase != "sprites" || !phases[2].WindowEnabled {
				t.Fatalf("unexpected sprites record: %+v", phases[2])
			}
			return nil
		})
	got, err := s.UpdateConfig(good)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if got.MaxConcurrentJobs != 4 {
		t.Fatalf("returned config not updated: %+v", got)
	}
	if s.Plan(time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)).Phase("sprites").Allowed {
		t.Fatal("new window not applied")
	}

	repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(errors.New("disk full"))
	if _, err := s.UpdateConfig(good); !apperrors.IsInternal(err) {
		t.Fatalf("expected an internal error when saving fails, got %v", err)
	}
}

func TestJobSchedulingService_Status(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockJobScheduleRepository(ctrl)
	jobRepo := mocks.NewMockJobHistoryRepository(ctrl)
	repo.EXPECT().GetConfig().Return(nil, nil)
	repo.EXPECT().ListPhases().Return(nil, nil)
	pm := newSchedulingTestPoolManager(t)
	s := NewJobSchedulingService(repo, pm, jobRepo, zap.NewNop())

	cfg := DefaultJobScheduleConfig()
	cfg.QuietHoursStart = "22:00"
	cfg.QuietHoursEnd = "06:00"
	setPhase(&cfg, "sprites", func(p *PhaseScheduleConfig) {
		p.WindowEnabled = true
		p.WindowStart = "01:00"
		p.WindowEnd = "05:00"
		p.QuietWorkers = 6
	})
	repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
	if _, err := s.UpdateConfig(cfg); err != nil {
		t.Fatal(err)
	}

	// 23:00 UTC: quiet hours, sprites window (01:00-05:00) not open yet
	now := time.Date(2026, 11, 21, 23, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	pm.ApplySchedule(s.Plan(now).PoolSettings()) // what the feeder does on its tick

	jobRepo.EXPECT().CountPendingByPhase().Return(map[string]int{"sprites": 30900, "thumbnail": 12}, nil)
	status, err := s.Status()
	if err != nil {
		t.Fatal(err)
	}
	if !status.QuietHours {
		t.Fatal("expected quiet hours at 23:00")
	}
	byPhase := map[string]PhaseScheduleState{}
	for _, p := range status.Phases {
		byPhase[p.Phase] = p
	}
	if _, ok := byPhase["fingerprint"]; ok {
		t.Fatal("fingerprint listed while duplicate detection is disabled")
	}
	sp := byPhase["sprites"]
	if sp.State != "paused" || sp.NextChange == nil || !sp.NextChange.Equal(time.Date(2026, 11, 22, 1, 0, 0, 0, time.UTC)) {
		t.Fatalf("sprites should be paused until 01:00, got %+v", sp)
	}
	if sp.Workers != 6 || sp.ConfiguredWorkers != 3 || sp.WorkerMode != "quiet" || sp.Pending != 30900 {
		t.Fatalf("unexpected sprites numbers: %+v", sp)
	}
	th := byPhase["thumbnail"]
	if th.State != "running" || th.NextChange != nil || th.Workers != 2 || th.WorkerMode != "day" || th.Pending != 12 {
		t.Fatalf("unexpected thumbnail state: %+v", th)
	}
	// Phases are listed by priority
	if status.Phases[0].Phase != "metadata" {
		t.Fatalf("expected metadata first, got %s", status.Phases[0].Phase)
	}
}
