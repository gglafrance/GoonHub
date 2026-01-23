package jobs

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// testJob is a minimal Job implementation for testing
type testJob struct {
	id        string
	status    JobStatus
	err       error
	executeFn func() error
	cancelled atomic.Bool
}

func newTestJob(id string, fn func() error) *testJob {
	return &testJob{
		id:        id,
		status:    JobStatusPending,
		executeFn: fn,
	}
}

func (j *testJob) Execute() error {
	if j.cancelled.Load() {
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
func (j *testJob) GetVideoID() uint     { return 0 }
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
		if r.Status != JobStatusFailed {
			t.Fatalf("expected cancelled job to have failed status, got %s", r.Status)
		}
		if r.Error == nil || r.Error.Error() != "job cancelled" {
			t.Fatalf("expected 'job cancelled' error, got: %v", r.Error)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for cancelled job result")
	}

	pool.Stop()
}
