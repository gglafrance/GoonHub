package core

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"
	"goonhub/internal/jobs"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

// cancellableCoreJob blocks until cancelled, like a real job honouring Cancel.
type cancellableCoreJob struct {
	id        string
	sceneID   uint
	phase     string
	started   chan<- string
	cancelled atomic.Bool
	done      chan struct{}
	ran       atomic.Bool
}

func newCancellableCoreJob(id string, sceneID uint, phase string, started chan<- string) *cancellableCoreJob {
	return &cancellableCoreJob{id: id, sceneID: sceneID, phase: phase, started: started, done: make(chan struct{})}
}

func (j *cancellableCoreJob) Execute() error { return j.ExecuteWithContext(context.Background()) }
func (j *cancellableCoreJob) ExecuteWithContext(ctx context.Context) error {
	if j.cancelled.Load() {
		return context.Canceled
	}
	j.ran.Store(true)
	j.started <- j.id
	select {
	case <-j.done:
		return context.Canceled
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (j *cancellableCoreJob) Cancel() {
	if j.cancelled.CompareAndSwap(false, true) {
		close(j.done)
	}
}
func (j *cancellableCoreJob) GetID() string    { return j.id }
func (j *cancellableCoreJob) GetSceneID() uint { return j.sceneID }
func (j *cancellableCoreJob) GetPhase() string { return j.phase }
func (j *cancellableCoreJob) GetStatus() jobs.JobStatus {
	if j.cancelled.Load() {
		return jobs.JobStatusCancelled
	}
	return jobs.JobStatusRunning
}
func (j *cancellableCoreJob) GetError() error { return nil }

// startSpritesJobs starts the sprites pool (1 worker) with one running job and queued jobs.
func startSpritesJobs(t *testing.T, feeder *JobQueueFeeder, queued ...string) (*cancellableCoreJob, []*cancellableCoreJob) {
	t.Helper()
	feeder.poolManager.Start()
	t.Cleanup(feeder.poolManager.Stop)

	started := make(chan string, len(queued)+1)
	run := newCancellableCoreJob("run", 1, "sprites", started)
	if err := feeder.poolManager.SubmitToSpritesPool(run); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("first job did not start")
	}
	var waiting []*cancellableCoreJob
	for i, id := range queued {
		job := newCancellableCoreJob(id, uint(i+2), "sprites", started)
		if err := feeder.poolManager.SubmitToSpritesPool(job); err != nil {
			t.Fatal(err)
		}
		waiting = append(waiting, job)
	}
	return run, waiting
}

func TestCancelBulk_PhaseCancelsPendingQueuedAndRunning(t *testing.T) {
	feeder, jobRepo, _ := newTestFeeder(t)
	run, queued := startSpritesJobs(t, feeder, "queued-1", "queued-2")

	gomock.InOrder(
		jobRepo.EXPECT().CancelPendingJobsByPhases([]string{"sprites"}, bulkCancelReason).Return(int64(7), nil),
		jobRepo.EXPECT().CancelClaimedJobs([]string{"queued-1", "queued-2"}, bulkCancelReason).Return(int64(2), nil),
	)

	result, err := feeder.CancelBulk("sprites", true)
	if err != nil {
		t.Fatalf("CancelBulk: %v", err)
	}
	if result.PendingCancelled != 7 || result.QueuedSkipped != 2 || result.RunningCancelled != 1 {
		t.Fatalf("result = %+v, want 7 pending, 2 queued, 1 running", result)
	}
	if !run.cancelled.Load() {
		t.Fatal("the running job was not cancelled")
	}
	if q := feeder.poolManager.GetQueueStatus(); q.SpritesQueued != 0 {
		t.Fatalf("sprites queue still holds %d jobs", q.SpritesQueued)
	}
	for _, j := range queued {
		if j.ran.Load() {
			t.Fatalf("queued job %s ran", j.id)
		}
	}
}

func TestCancelBulk_AllPhasesKeepsRunningJobs(t *testing.T) {
	feeder, jobRepo, _ := newTestFeeder(t)
	run, _ := startSpritesJobs(t, feeder)

	jobRepo.EXPECT().CancelPendingJobsByPhases(QueuePhases, bulkCancelReason).Return(int64(30931), nil)

	result, err := feeder.CancelBulk(BulkCancelAllPhases, false)
	if err != nil {
		t.Fatalf("CancelBulk: %v", err)
	}
	if result.PendingCancelled != 30931 || result.QueuedSkipped != 0 || result.RunningCancelled != 0 {
		t.Fatalf("result = %+v, want 30931 pending only", result)
	}
	if len(result.Phases) != len(QueuePhases) {
		t.Fatalf("phases = %v, want %v", result.Phases, QueuePhases)
	}
	if run.cancelled.Load() {
		t.Fatal("the running job was cancelled without include_running")
	}
}

func TestCancelBulk_InvalidPhase(t *testing.T) {
	feeder, _, _ := newTestFeeder(t)
	for _, phase := range []string{"", "scan", "ALL", "sprite"} {
		if _, err := feeder.CancelBulk(phase, false); !apperrors.IsValidation(err) {
			t.Fatalf("CancelBulk(%q) error = %v, want a validation error", phase, err)
		}
	}
}

func TestCancelBulk_RepositoryErrorStopsBeforeDraining(t *testing.T) {
	feeder, jobRepo, _ := newTestFeeder(t)
	startSpritesJobs(t, feeder, "queued-1")

	jobRepo.EXPECT().CancelPendingJobsByPhases(gomock.Any(), gomock.Any()).Return(int64(0), context.DeadlineExceeded)

	if _, err := feeder.CancelBulk("sprites", false); err == nil {
		t.Fatal("expected an error")
	}
	if q := feeder.poolManager.GetQueueStatus(); q.SpritesQueued != 1 {
		t.Fatalf("sprites queue holds %d jobs, want the queued job left in place", q.SpritesQueued)
	}
}

// The feed of the phase waits for the bulk cancel: nothing is claimed while the pending jobs
// are being cancelled.
func TestCancelBulk_FeederWaitsForCancel(t *testing.T) {
	feeder, jobRepo, _ := newTestFeeder(t)

	var cancelDone atomic.Bool
	fed := make(chan struct{})
	jobRepo.EXPECT().CancelPendingJobsByPhases([]string{"sprites"}, gomock.Any()).DoAndReturn(
		func([]string, string) (int64, error) {
			go func() {
				feeder.feedPhase("sprites")
				close(fed)
			}()
			select {
			case <-fed:
				t.Error("the feeder claimed jobs during the bulk cancel")
			case <-time.After(100 * time.Millisecond):
			}
			cancelDone.Store(true)
			return 3, nil
		})
	jobRepo.EXPECT().ClaimPendingJobs("sprites", gomock.Any(), gomock.Any()).DoAndReturn(
		func(string, int, data.ClaimOptions) ([]data.JobHistory, error) {
			if !cancelDone.Load() {
				t.Error("claim ran before the bulk cancel finished")
			}
			return nil, nil
		})

	if _, err := feeder.CancelBulk("sprites", false); err != nil {
		t.Fatalf("CancelBulk: %v", err)
	}
	select {
	case <-fed:
	case <-time.After(2 * time.Second):
		t.Fatal("the feeder did not resume after the bulk cancel")
	}
}

func TestCancelBulk_PublishesEvent(t *testing.T) {
	feeder, jobRepo, _ := newTestFeeder(t)
	bus := NewEventBus(zap.NewNop())
	feeder.SetEventBus(bus)
	id, events := bus.Subscribe()
	defer bus.Unsubscribe(id)

	jobRepo.EXPECT().CancelPendingJobsByPhases([]string{"metadata"}, gomock.Any()).Return(int64(4), nil)

	if _, err := feeder.CancelBulk("metadata", false); err != nil {
		t.Fatalf("CancelBulk: %v", err)
	}
	select {
	case ev := <-events:
		if ev.Type != "jobs:bulk_cancelled" {
			t.Fatalf("event type = %s", ev.Type)
		}
		data, ok := ev.Data.(map[string]any)
		if !ok || data["pending_cancelled"] != int64(4) {
			t.Fatalf("event data = %#v", ev.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("no event published")
	}
}
