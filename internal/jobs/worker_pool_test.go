package jobs

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// testJob is a minimal Job implementation for testing
type testJob struct {
	id        string
	sceneID   uint // Unique per job to avoid deduplication
	status    JobStatus
	err       error
	executeFn func() error
	cancelled atomic.Bool
}

var testJobCounter atomic.Uint64

func newTestJob(id string, fn func() error) *testJob {
	return &testJob{
		id:        id,
		sceneID:   uint(testJobCounter.Add(1)), // Unique scene ID for each job
		status:    JobStatusPending,
		executeFn: fn,
	}
}

func (j *testJob) Execute() error {
	return j.ExecuteWithContext(context.Background())
}

func (j *testJob) ExecuteWithContext(ctx context.Context) error {
	if j.cancelled.Load() || ctx.Err() != nil {
		j.status = JobStatusCancelled
		return fmt.Errorf("job cancelled")
	}
	j.status = JobStatusRunning
	err := j.executeFn()
	if err != nil {
		j.status = JobStatusFailed
		j.err = err
	} else {
		j.status = JobStatusCompleted
	}
	return err
}

func (j *testJob) Cancel() {
	j.cancelled.Store(true)
}

func (j *testJob) GetID() string        { return j.id }
func (j *testJob) GetSceneID() uint     { return j.sceneID }
func (j *testJob) GetPhase() string     { return "test" }
func (j *testJob) GetStatus() JobStatus { return j.status }
func (j *testJob) GetError() error      { return j.err }

func TestWorkerPool_ExecutesJobs(t *testing.T) {
	pool := NewWorkerPool(2, 10)
	pool.Start()

	var completed atomic.Int32

	for i := 0; i < 3; i++ {
		job := newTestJob(fmt.Sprintf("job-%d", i), func() error {
			completed.Add(1)
			return nil
		})
		if err := pool.Submit(job); err != nil {
			t.Fatalf("failed to submit job: %v", err)
		}
	}

	// Drain results
	for i := 0; i < 3; i++ {
		select {
		case result := <-pool.Results():
			if result.Status != JobStatusCompleted {
				t.Fatalf("expected completed status, got %s", result.Status)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for job result")
		}
	}

	if completed.Load() != 3 {
		t.Fatalf("expected 3 jobs completed, got %d", completed.Load())
	}

	pool.Stop()
}

func TestWorkerPool_PropagatesErrors(t *testing.T) {
	pool := NewWorkerPool(1, 10)
	pool.Start()

	job := newTestJob("failing-job", func() error {
		return fmt.Errorf("something went wrong")
	})
	if err := pool.Submit(job); err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	select {
	case result := <-pool.Results():
		if result.Status != JobStatusFailed {
			t.Fatalf("expected failed status, got %s", result.Status)
		}
		if result.Error == nil {
			t.Fatal("expected non-nil error")
		}
		if result.Error.Error() != "something went wrong" {
			t.Fatalf("expected 'something went wrong', got: %v", result.Error)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for job result")
	}

	pool.Stop()
}

func TestWorkerPool_GracefulShutdown(t *testing.T) {
	pool := NewWorkerPool(1, 10)
	pool.Start()

	started := make(chan struct{})
	done := make(chan struct{})
	job := newTestJob("slow-job", func() error {
		close(started)
		time.Sleep(100 * time.Millisecond)
		close(done)
		return nil
	})

	if err := pool.Submit(job); err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	// Wait for the job to start executing
	<-started

	// Stop should wait for the running job to complete
	pool.Stop()

	// Verify the job actually completed (wasn't terminated mid-flight)
	select {
	case <-done:
		// Job completed before pool stopped
	default:
		t.Fatal("expected job to complete during graceful shutdown")
	}

	if pool.Running() {
		t.Fatal("pool should not be running after Stop")
	}
}

func TestWorkerPool_SubmitAfterStop(t *testing.T) {
	pool := NewWorkerPool(1, 10)
	pool.Start()
	pool.Stop()

	job := newTestJob("late-job", func() error { return nil })
	err := pool.Submit(job)
	if err == nil {
		t.Fatal("expected error when submitting to stopped pool")
	}
}

func TestWorkerPool_ConcurrentSubmit(t *testing.T) {
	pool := NewWorkerPool(4, 100)
	pool.Start()

	var wg sync.WaitGroup
	var submitted atomic.Int32

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			job := newTestJob(fmt.Sprintf("concurrent-%d", id), func() error {
				return nil
			})
			if err := pool.Submit(job); err == nil {
				submitted.Add(1)
			}
		}(i)
	}
	wg.Wait()

	if submitted.Load() != 50 {
		t.Fatalf("expected 50 submitted, got %d", submitted.Load())
	}

	// Drain all results
	for i := 0; i < 50; i++ {
		select {
		case <-pool.Results():
		case <-time.After(10 * time.Second):
			t.Fatalf("timed out waiting for result %d", i)
		}
	}

	pool.Stop()
}

func TestWorkerPool_ResultsChannel(t *testing.T) {
	pool := NewWorkerPool(2, 10)
	pool.Start()

	jobCount := 5
	for i := 0; i < jobCount; i++ {
		job := newTestJob(fmt.Sprintf("result-%d", i), func() error { return nil })
		if err := pool.Submit(job); err != nil {
			t.Fatalf("submit failed: %v", err)
		}
	}

	received := 0
	for received < jobCount {
		select {
		case result := <-pool.Results():
			if result.Status != JobStatusCompleted {
				t.Fatalf("expected completed, got %s", result.Status)
			}
			received++
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out after receiving %d/%d results", received, jobCount)
		}
	}

	pool.Stop()
}

func TestWorkerPool_CancelledJob(t *testing.T) {
	pool := NewWorkerPool(2, 10)
	pool.Start()

	// Submit a job that gets cancelled before the worker picks it up
	cancelledJob := newTestJob("to-cancel", func() error {
		return nil
	})
	cancelledJob.Cancel()

	if err := pool.Submit(cancelledJob); err != nil {
		t.Fatalf("failed to submit cancelled job: %v", err)
	}

	// The cancelled job's Execute checks the cancelled flag and returns error
	select {
	case r := <-pool.Results():
		if r.JobID != "to-cancel" {
			t.Fatalf("expected job ID 'to-cancel', got %s", r.JobID)
		}
		// Job can report as cancelled or failed depending on when the check happens
		if r.Status != JobStatusCancelled && r.Status != JobStatusFailed {
			t.Fatalf("expected cancelled or failed status, got %s", r.Status)
		}
		if r.Error == nil || r.Error.Error() != "job cancelled" {
			t.Fatalf("expected 'job cancelled' error, got: %v", r.Error)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for cancelled job result")
	}

	pool.Stop()
}

// testJobWithSceneID extends testJob with a scene ID for duplicate testing
type testJobWithSceneID struct {
	id        string
	sceneID   uint
	phase     string
	status    JobStatus
	err       error
	executeFn func(ctx context.Context) error
	cancelled atomic.Bool
	cancelFn  context.CancelFunc
	mu        sync.Mutex
}

func newTestJobWithSceneID(id string, sceneID uint, phase string, fn func() error) *testJobWithSceneID {
	return &testJobWithSceneID{
		id:      id,
		sceneID: sceneID,
		phase:   phase,
		status:  JobStatusPending,
		executeFn: func(ctx context.Context) error {
			return fn()
		},
	}
}

func newTestJobWithSceneIDContext(id string, sceneID uint, phase string, fn func(ctx context.Context) error) *testJobWithSceneID {
	return &testJobWithSceneID{
		id:        id,
		sceneID:   sceneID,
		phase:     phase,
		status:    JobStatusPending,
		executeFn: fn,
	}
}

func (j *testJobWithSceneID) Execute() error {
	return j.ExecuteWithContext(context.Background())
}

func (j *testJobWithSceneID) ExecuteWithContext(ctx context.Context) error {
	// Create a cancellable context for this execution
	j.mu.Lock()
	execCtx, cancelFn := context.WithCancel(ctx)
	j.cancelFn = cancelFn
	j.mu.Unlock()
	defer cancelFn()

	if j.cancelled.Load() || execCtx.Err() != nil {
		j.status = JobStatusCancelled
		return fmt.Errorf("job cancelled")
	}
	j.status = JobStatusRunning
	err := j.executeFn(execCtx)
	if ctx.Err() == context.DeadlineExceeded || execCtx.Err() == context.DeadlineExceeded {
		j.status = JobStatusTimedOut
		j.err = fmt.Errorf("job timed out")
		return j.err
	}
	if ctx.Err() == context.Canceled || execCtx.Err() == context.Canceled || j.cancelled.Load() {
		j.status = JobStatusCancelled
		return fmt.Errorf("job cancelled")
	}
	if err != nil {
		j.status = JobStatusFailed
		j.err = err
	} else {
		j.status = JobStatusCompleted
	}
	return err
}

func (j *testJobWithSceneID) Cancel() {
	j.cancelled.Store(true)
	j.mu.Lock()
	if j.cancelFn != nil {
		j.cancelFn()
	}
	j.mu.Unlock()
}

func (j *testJobWithSceneID) GetID() string        { return j.id }
func (j *testJobWithSceneID) GetSceneID() uint     { return j.sceneID }
func (j *testJobWithSceneID) GetPhase() string     { return j.phase }
func (j *testJobWithSceneID) GetStatus() JobStatus { return j.status }
func (j *testJobWithSceneID) GetError() error      { return j.err }

func TestWorkerPool_DuplicateJobRejection(t *testing.T) {
	pool := NewWorkerPool(1, 10)
	pool.Start()

	executed := make(chan struct{}, 2)

	// Submit first job
	job1 := newTestJobWithSceneID("job-1", 100, "metadata", func() error {
		time.Sleep(100 * time.Millisecond) // Hold the job for a bit
		executed <- struct{}{}
		return nil
	})

	if err := pool.Submit(job1); err != nil {
		t.Fatalf("failed to submit first job: %v", err)
	}

	// Try to submit duplicate job (same scene+phase)
	job2 := newTestJobWithSceneID("job-2", 100, "metadata", func() error {
		executed <- struct{}{}
		return nil
	})

	err := pool.Submit(job2)
	if err == nil {
		t.Fatal("expected error for duplicate job, got nil")
	}

	if !IsDuplicateJobError(err) {
		t.Fatalf("expected DuplicateJobError, got: %v", err)
	}

	// Wait for first job to complete
	select {
	case <-pool.Results():
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for job result")
	}

	// Verify only one job executed
	select {
	case <-executed:
	case <-time.After(1 * time.Second):
		t.Fatal("first job did not execute")
	}

	// Second execution should not happen
	select {
	case <-executed:
		t.Fatal("duplicate job should not have executed")
	default:
		// This is expected
	}

	pool.Stop()
}

func TestWorkerPool_CancelJobActive(t *testing.T) {
	pool := NewWorkerPool(1, 10)
	pool.Start()

	started := make(chan struct{})
	job := newTestJobWithSceneIDContext("cancellable", 200, "thumbnail", func(ctx context.Context) error {
		close(started)
		// Simulate a long-running job that checks context
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			return nil
		}
	})

	if err := pool.Submit(job); err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	// Wait for job to start
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("job did not start")
	}

	// Cancel the job
	if err := pool.CancelJob("cancellable"); err != nil {
		t.Fatalf("failed to cancel job: %v", err)
	}

	// Result should indicate cancellation
	select {
	case result := <-pool.Results():
		if result.Status != JobStatusCancelled && result.Status != JobStatusFailed {
			t.Fatalf("expected cancelled or failed status, got %s", result.Status)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for cancelled job result")
	}

	pool.Stop()
}

func TestWorkerPool_CancelJobNotFound(t *testing.T) {
	pool := NewWorkerPool(1, 10)
	pool.Start()

	err := pool.CancelJob("non-existent")
	if err == nil {
		t.Fatal("expected error for non-existent job")
	}

	if err.Error() != "job not found: non-existent" {
		t.Fatalf("unexpected error message: %v", err)
	}

	pool.Stop()
}

func TestWorkerPool_Timeout(t *testing.T) {
	pool := NewWorkerPool(1, 10)
	pool.SetTimeout(100 * time.Millisecond)
	pool.Start()

	job := newTestJobWithSceneIDContext("slow-job", 300, "sprites", func(ctx context.Context) error {
		// Simulate a long-running job that checks context
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			return nil
		}
	})

	if err := pool.Submit(job); err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	// Result should indicate timeout
	select {
	case result := <-pool.Results():
		// The job may report timed_out, cancelled, or failed depending on timing
		if result.Status != JobStatusTimedOut && result.Status != JobStatusCancelled && result.Status != JobStatusFailed {
			t.Fatalf("expected timed_out, cancelled, or failed status, got %s", result.Status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for timeout result")
	}

	pool.Stop()
}

func TestWorkerPool_GetJob(t *testing.T) {
	pool := NewWorkerPool(1, 10)
	pool.Start()

	done := make(chan struct{})
	job := newTestJobWithSceneID("trackable", 400, "metadata", func() error {
		<-done // Wait until we signal to complete
		return nil
	})

	if err := pool.Submit(job); err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	// Give time for job to be picked up
	time.Sleep(50 * time.Millisecond)

	// Should be able to get the job while it's running
	retrieved, ok := pool.GetJob("trackable")
	if !ok {
		t.Fatal("expected to find job by ID")
	}
	if retrieved.GetID() != "trackable" {
		t.Fatalf("expected job ID 'trackable', got %s", retrieved.GetID())
	}

	// Signal job to complete
	close(done)

	// Drain result
	select {
	case <-pool.Results():
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for job result")
	}

	// After completion, job should be unregistered
	_, ok = pool.GetJob("trackable")
	if ok {
		t.Fatal("expected job to be unregistered after completion")
	}

	pool.Stop()
}

func TestWorkerPool_SetTimeout(t *testing.T) {
	pool := NewWorkerPool(1, 10)

	// Initial timeout should be 0
	if pool.GetTimeout() != 0 {
		t.Fatalf("expected initial timeout 0, got %v", pool.GetTimeout())
	}

	// Set timeout
	pool.SetTimeout(5 * time.Minute)
	if pool.GetTimeout() != 5*time.Minute {
		t.Fatalf("expected timeout 5m, got %v", pool.GetTimeout())
	}
}

func TestWorkerPool_ResubmitAfterComplete(t *testing.T) {
	pool := NewWorkerPool(1, 10)
	pool.Start()

	// Submit and complete first job
	job1 := newTestJobWithSceneID("job-1", 500, "metadata", func() error {
		return nil
	})

	if err := pool.Submit(job1); err != nil {
		t.Fatalf("failed to submit first job: %v", err)
	}

	// Wait for completion
	select {
	case <-pool.Results():
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for first job")
	}

	// Should be able to resubmit for same scene+phase
	job2 := newTestJobWithSceneID("job-2", 500, "metadata", func() error {
		return nil
	})

	if err := pool.Submit(job2); err != nil {
		t.Fatalf("failed to resubmit job: %v", err)
	}

	// Wait for completion
	select {
	case <-pool.Results():
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for second job")
	}

	pool.Stop()
}
