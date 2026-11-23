package core

import (
	"testing"
	"time"

	"goonhub/internal/config"
	"goonhub/internal/data"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

type testRouter struct {
	remoteOnly map[string]bool
	remote     map[string]bool
}

func (r testRouter) LocalAllowed(phase string) bool  { return !r.remoteOnly[phase] }
func (r testRouter) RemoteAllowed(phase string) bool { return r.remote[phase] || r.remoteOnly[phase] }
func (r testRouter) RemoteOnlyPhases() []string {
	var out []string
	for p := range r.remoteOnly {
		out = append(out, p)
	}
	return out
}

type testRemoteCanceller struct {
	phases []string
	jobs   []string
	n      int
}

func (c *testRemoteCanceller) CancelJob(jobID string) (bool, error) {
	c.jobs = append(c.jobs, jobID)
	return jobID == "remote-job", nil
}
func (c *testRemoteCanceller) CancelPhases(phases []string) (int, error) {
	c.phases = phases
	return c.n, nil
}

// A phase routed to the remote workers only is never claimed by the local feeder.
func TestFeederDoesNotClaimRemoteOnlyPhases(t *testing.T) {
	feeder, repo, _ := newTestFeeder(t)
	feeder.SetPhaseRouter(testRouter{remoteOnly: map[string]bool{"sprites": true}})
	// No ClaimPendingJobs expectation for sprites: gomock fails the test if it is called.
	feeder.feedPhase("sprites")

	repo.EXPECT().ClaimPendingJobs("thumbnail", gomock.Any(), gomock.Any()).Return(nil, nil)
	feeder.feedPhase("thumbnail")
}

// The startup stuck-pending recovery leaves the jobs of remote-only phases alone: they wait
// for a worker on purpose.
func TestStuckPendingRecoverySkipsRemoteOnlyPhases(t *testing.T) {
	feeder, repo, _ := newTestFeeder(t)
	feeder.SetPhaseRouter(testRouter{remoteOnly: map[string]bool{"fingerprint": true}})
	repo.EXPECT().MarkOrphanedRunningAsFailed(gomock.Any()).Return(int64(0), nil)
	repo.EXPECT().MarkStuckPendingJobsAsFailed(gomock.Any(), []string{"fingerprint"}).Return(int64(0), nil)
	feeder.recoverOrphanedJobs()
}

func TestRemoteClaimOptionsWithoutSchedule(t *testing.T) {
	feeder, _, _ := newTestFeeder(t)
	ok, since := feeder.RemoteClaimOptions("sprites")
	if !ok || !since.IsZero() {
		t.Fatalf("without a schedule every phase may run: %v %v", ok, since)
	}
	plan := &SchedulePlan{NewSceneSince: time.Unix(1000, 0)}
	feeder.planMu.Lock()
	feeder.plan = plan
	feeder.planMu.Unlock()
	if _, since := feeder.RemoteClaimOptions("sprites"); !since.Equal(time.Unix(1000, 0)) {
		t.Fatalf("new scene since %v", since)
	}
}

// A bulk cancel with running jobs cancels the remote ones too.
func TestBulkCancelIncludesRemoteJobs(t *testing.T) {
	feeder, repo, _ := newTestFeeder(t)
	c := &testRemoteCanceller{n: 2}
	feeder.SetRemoteCanceller(c)
	repo.EXPECT().CancelPendingJobsByPhases([]string{"sprites"}, gomock.Any()).Return(int64(0), nil)
	res, err := feeder.CancelBulk("sprites", true)
	if err != nil {
		t.Fatal(err)
	}
	if res.RunningCancelled != 2 || len(c.phases) != 1 || c.phases[0] != "sprites" {
		t.Fatalf("result %+v, phases %v", res, c.phases)
	}
	repo.EXPECT().CancelPendingJobsByPhases([]string{"sprites"}, gomock.Any()).Return(int64(0), nil)
	c.phases = nil
	if _, err := feeder.CancelBulk("sprites", false); err != nil || c.phases != nil {
		t.Fatal("remote jobs cancelled without include_running")
	}
}

// Cancelling a job leased to a remote worker goes to the worker service.
func TestCancelJobFallsBackToRemoteWorkers(t *testing.T) {
	feeder, repo, _ := newTestFeeder(t)
	svc := &SceneProcessingService{poolManager: feeder.poolManager, jobHistory: NewJobHistoryService(repo, config.ProcessingConfig{}, zap.NewNop())}
	c := &testRemoteCanceller{}
	svc.SetRemoteCanceller(c)
	if err := svc.CancelJob("remote-job"); err != nil {
		t.Fatalf("cancel remote job: %v", err)
	}
	repo.EXPECT().CancelPendingJob("pending-job").Return(nil)
	if err := svc.CancelJob("pending-job"); err != nil {
		t.Fatalf("cancel pending job: %v", err)
	}
	if len(c.jobs) != 2 {
		t.Fatalf("remote canceller asked %v", c.jobs)
	}
}

var _ = data.JobStatusPending
