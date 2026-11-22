package jobs

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func collectResults(t *testing.T, pool *WorkerPool, n int) map[string]JobResult {
	t.Helper()
	results := make(map[string]JobResult, n)
	for len(results) < n {
		select {
		case r := <-pool.Results():
			results[r.JobID] = r
		case <-time.After(3 * time.Second):
			t.Fatalf("timed out waiting for results, got %d of %d", len(results), n)
		}
	}
	return results
}

func blockingJob(id string, sceneID uint, started chan<- struct{}) *testJobWithSceneID {
	return newTestJobWithSceneIDContext(id, sceneID, "fingerprint", func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})
}

// A job taken from the queue by a worker waiting for a slot of the global cap is cancelled
// without running; the executing job is left alone without includeExecuting.
func TestWorkerPool_CancelRegisteredSkipsSlotWaiters(t *testing.T) {
	limiter := NewSlotLimiter(1)
	pool := NewWorkerPool(2, 10)
	pool.SetSlotLimiter(limiter)
	pool.Start()
	defer pool.Stop()

	started := make(chan struct{})
	running := blockingJob("running", 1, started)
	var waiterRan atomic.Bool
	waiter := newTestJobWithSceneID("waiter", 2, "fingerprint", func() error {
		waiterRan.Store(true)
		return nil
	})

	if err := pool.Submit(running); err != nil {
		t.Fatal(err)
	}
	<-started
	if err := pool.Submit(waiter); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the second worker to wait for a slot", func() bool { return limiter.Waiting() == 1 })

	waiting, executing := pool.CancelRegistered(false)
	if waiting != 1 || executing != 0 {
		t.Fatalf("CancelRegistered(false) = %d waiting, %d executing, want 1 and 0", waiting, executing)
	}
	if running.cancelled.Load() {
		t.Fatal("the executing job was cancelled without includeExecuting")
	}

	// Free the slot: the waiter gets it and returns as cancelled without running.
	if err := pool.CancelJob("running"); err != nil {
		t.Fatal(err)
	}
	results := collectResults(t, pool, 2)
	if got := results["waiter"].Status; got != JobStatusCancelled {
		t.Fatalf("waiter status = %s, want cancelled", got)
	}
	if waiterRan.Load() {
		t.Fatal("the cancelled waiter ran")
	}
}

func TestWorkerPool_CancelRegisteredIncludeExecuting(t *testing.T) {
	pool := NewWorkerPool(1, 10)
	pool.Start()
	defer pool.Stop()

	started := make(chan struct{})
	if err := pool.Submit(blockingJob("running", 1, started)); err != nil {
		t.Fatal(err)
	}
	<-started
	waitFor(t, "the job to be marked executing", func() bool { return pool.Registry().IsExecuting("running") })

	waiting, executing := pool.CancelRegistered(true)
	if waiting != 0 || executing != 1 {
		t.Fatalf("CancelRegistered(true) = %d waiting, %d executing, want 0 and 1", waiting, executing)
	}
	results := collectResults(t, pool, 1)
	if got := results["running"].Status; got != JobStatusCancelled {
		t.Fatalf("running job status = %s, want cancelled", got)
	}
}

// Queued jobs are taken out by DrainQueued before CancelRegistered: they are neither run nor
// counted twice.
func TestWorkerPool_DrainThenCancelRegistered(t *testing.T) {
	pool := NewWorkerPool(1, 10)
	pool.Start()
	defer pool.Stop()

	started := make(chan struct{})
	if err := pool.Submit(blockingJob("running", 1, started)); err != nil {
		t.Fatal(err)
	}
	<-started
	waitFor(t, "the job to be marked executing", func() bool { return pool.Registry().IsExecuting("running") })
	for i, id := range []string{"queued-1", "queued-2"} {
		if err := pool.Submit(newTestJobWithSceneID(id, uint(10+i), "fingerprint", func() error { return nil })); err != nil {
			t.Fatal(err)
		}
	}

	drained := pool.DrainQueued()
	if len(drained) != 2 {
		t.Fatalf("drained %v, want the 2 queued jobs", drained)
	}
	waiting, executing := pool.CancelRegistered(false)
	if waiting != 0 || executing != 0 {
		t.Fatalf("CancelRegistered(false) = %d waiting, %d executing, want 0 and 0", waiting, executing)
	}
	if pool.QueueSize() != 0 {
		t.Fatalf("queue size = %d, want 0", pool.QueueSize())
	}
	if err := pool.CancelJob("running"); err != nil {
		t.Fatal(err)
	}
	results := collectResults(t, pool, 1)
	if _, ok := results["running"]; !ok {
		t.Fatalf("unexpected results %v", results)
	}
}
