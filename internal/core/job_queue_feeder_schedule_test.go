package core

import (
	"context"
	"goonhub/internal/data"
	"goonhub/internal/jobs"
	"sync"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
)

// fakePlanner returns a plan set by the test.
type fakePlanner struct {
	mu   sync.Mutex
	plan SchedulePlan
}

func (p *fakePlanner) Plan(time.Time) SchedulePlan {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.plan
}

func (p *fakePlanner) set(plan SchedulePlan) {
	p.mu.Lock()
	p.plan = plan
	p.mu.Unlock()
}

func planWith(phases map[string]PhasePlan) SchedulePlan {
	plan := SchedulePlan{Phases: map[string]PhasePlan{}}
	for _, phase := range SchedulablePhases {
		pp, ok := phases[phase]
		if !ok {
			pp = PhasePlan{Allowed: true}
		}
		plan.Phases[phase] = pp
		plan.Order = append(plan.Order, phase)
	}
	return plan
}

// blockingCoreJob is a jobs.Job that blocks until released.
type blockingCoreJob struct {
	id      string
	sceneID uint
	phase   string
	started chan<- string
	release <-chan struct{}
}

func (j *blockingCoreJob) Execute() error { return j.ExecuteWithContext(context.Background()) }
func (j *blockingCoreJob) ExecuteWithContext(ctx context.Context) error {
	j.started <- j.id
	select {
	case <-j.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (j *blockingCoreJob) Cancel()                   {}
func (j *blockingCoreJob) GetID() string             { return j.id }
func (j *blockingCoreJob) GetSceneID() uint          { return j.sceneID }
func (j *blockingCoreJob) GetPhase() string          { return j.phase }
func (j *blockingCoreJob) GetStatus() jobs.JobStatus { return jobs.JobStatusRunning }
func (j *blockingCoreJob) GetError() error           { return nil }

func TestFeeder_PausedPhaseClaimsNothing(t *testing.T) {
	feeder, jobRepo, _ := newTestFeeder(t)
	since := time.Date(2026, 11, 21, 6, 0, 0, 0, time.UTC)
	plan := planWith(map[string]PhasePlan{
		"sprites": {Allowed: false, WindowEnabled: true, NextChange: time.Date(2026, 11, 22, 1, 0, 0, 0, time.UTC)},
	})
	plan.NewSceneSince = since
	feeder.SetScheduler(&fakePlanner{plan: plan})
	feeder.refreshSchedule()

	// Only the allowed phase claims, with the new scene boost.
	jobRepo.EXPECT().ClaimPendingJobs("thumbnail", gomock.Any(), data.ClaimOptions{NewSceneSince: since}).
		Return(nil, nil).Times(1)

	feeder.feedPhase("sprites")
	feeder.feedPhase("thumbnail")
}

func TestFeeder_WithoutSchedulerClaimsAsBefore(t *testing.T) {
	feeder, jobRepo, _ := newTestFeeder(t)
	jobRepo.EXPECT().ClaimPendingJobs("sprites", 10, data.ClaimOptions{}).Return(nil, nil).Times(1)
	feeder.feedPhase("sprites")
}

func TestFeeder_PausingPutsQueuedJobsBackToPending(t *testing.T) {
	feeder, jobRepo, _ := newTestFeeder(t)
	planner := &fakePlanner{plan: planWith(nil)}
	feeder.SetScheduler(planner)
	feeder.refreshSchedule()

	feeder.poolManager.Start()
	defer feeder.poolManager.Stop()

	started := make(chan string, 5)
	release := make(chan struct{})
	defer close(release)
	for i, id := range []string{"run", "queued-1", "queued-2"} {
		job := &blockingCoreJob{id: id, sceneID: uint(i + 1), phase: "sprites", started: started, release: release}
		if err := feeder.poolManager.SubmitToSpritesPool(job); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("first job did not start")
	}

	// The window closes: queued jobs go back to pending, the running job keeps running.
	planner.set(planWith(map[string]PhasePlan{"sprites": {Allowed: false, WindowEnabled: true}}))
	feeder.refreshSchedule()
	jobRepo.EXPECT().ResetJobsToPending([]string{"queued-1", "queued-2"}).Return(int64(2), nil).Times(1)
	feeder.feedPhase("sprites")

	// Still paused: nothing more happens.
	feeder.feedPhase("sprites")

	if q := feeder.poolManager.GetQueueStatus(); q.SpritesQueued != 0 || q.SpritesActive != 1 {
		t.Fatalf("expected 0 queued and 1 running sprites job, got %d and %d", q.SpritesQueued, q.SpritesActive)
	}

	// The window opens again: claiming resumes.
	planner.set(planWith(nil))
	feeder.refreshSchedule()
	jobRepo.EXPECT().ClaimPendingJobs("sprites", gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
	feeder.feedPhase("sprites")
}

func TestFeeder_AppliesQuietHoursWorkersAndCap(t *testing.T) {
	feeder, _, _ := newTestFeeder(t)
	planner := &fakePlanner{}
	night := planWith(map[string]PhasePlan{"sprites": {Allowed: true, QuietWorkers: 4, Priority: 20}})
	night.QuietHours = true
	night.MaxConcurrentJobs = 3
	planner.set(night)
	feeder.SetScheduler(planner)

	feeder.poolManager.Start()
	defer feeder.poolManager.Stop()

	feeder.refreshSchedule()
	if got := feeder.poolManager.GetPoolConfig().SpritesWorkers; got != 4 {
		t.Fatalf("sprites workers at night = %d, want 4", got)
	}
	if got := feeder.poolManager.GetBasePoolConfig().SpritesWorkers; got != 1 {
		t.Fatalf("configured sprites workers = %d, want 1 (unchanged)", got)
	}
	if got := feeder.poolManager.MaxConcurrentJobs(); got != 3 {
		t.Fatalf("global cap = %d, want 3", got)
	}

	// A change of the configured size during the quiet hours is kept for the day.
	cfg := feeder.poolManager.GetBasePoolConfig()
	cfg.SpritesWorkers = 2
	if err := feeder.poolManager.UpdatePoolConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if got := feeder.poolManager.GetPoolConfig().SpritesWorkers; got != 4 {
		t.Fatalf("sprites workers at night after a config change = %d, want 4", got)
	}

	planner.set(planWith(nil)) // day, no cap
	feeder.refreshSchedule()
	if got := feeder.poolManager.GetPoolConfig().SpritesWorkers; got != 2 {
		t.Fatalf("sprites workers by day = %d, want 2", got)
	}
	if got := feeder.poolManager.MaxConcurrentJobs(); got != 0 {
		t.Fatalf("global cap by day = %d, want 0", got)
	}
}

func TestFeeder_StuckPendingRecoverySkipsWindowedPhases(t *testing.T) {
	feeder, jobRepo, _ := newTestFeeder(t)
	feeder.SetScheduler(&fakePlanner{plan: planWith(map[string]PhasePlan{
		"sprites":     {Allowed: false, WindowEnabled: true},
		"fingerprint": {Allowed: true, WindowEnabled: true},
	})})
	feeder.refreshSchedule()

	jobRepo.EXPECT().MarkOrphanedRunningAsFailed(gomock.Any()).Return(int64(0), nil)
	jobRepo.EXPECT().MarkStuckPendingJobsAsFailed(gomock.Any(), []string{"sprites", "fingerprint"}).Return(int64(0), nil)
	feeder.recoverOrphanedJobs()
}
