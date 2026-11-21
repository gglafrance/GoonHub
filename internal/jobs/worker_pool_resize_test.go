package jobs

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// blockingJobs creates n jobs that report when they start and block until release is closed.
func blockingJobs(prefix string, n int, firstSceneID uint, started chan<- string, release <-chan struct{}) []*testJobWithSceneID {
	out := make([]*testJobWithSceneID, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("%s-%d", prefix, i)
		out[i] = newTestJobWithSceneIDContext(id, firstSceneID+uint(i), prefix, func(ctx context.Context) error {
			started <- id
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}
	return out
}

func waitStarted(t *testing.T, started <-chan string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatalf("only %d of %d jobs started", i, n)
		}
	}
}

func expectNoStart(t *testing.T, started <-chan string) {
	t.Helper()
	select {
	case id := <-started:
		t.Fatalf("job %s started, expected none", id)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestWorkerPool_ResizeGrowRunsMoreJobs(t *testing.T) {
	pool := NewWorkerPool(1, 10)
	pool.Start()
	defer pool.Stop()

	started := make(chan string, 10)
	release := make(chan struct{})
	defer close(release)
	for _, j := range blockingJobs("grow", 3, 1, started, release) {
		if err := pool.Submit(j); err != nil {
			t.Fatal(err)
		}
	}
	waitStarted(t, started, 1)
	expectNoStart(t, started)

	pool.Resize(3)
	if pool.ActiveWorkers() != 3 {
		t.Fatalf("expected 3 workers, got %d", pool.ActiveWorkers())
	}
	waitStarted(t, started, 2)
}

func TestWorkerPool_ResizeShrinkLetsRunningJobsFinish(t *testing.T) {
	pool := NewWorkerPool(3, 10)
	pool.Start()
	defer pool.Stop()

	started := make(chan string, 10)
	release := make(chan struct{})
	jobs := blockingJobs("shrink", 5, 10, started, release)
	for _, j := range jobs {
		if err := pool.Submit(j); err != nil {
			t.Fatal(err)
		}
	}
	waitStarted(t, started, 3)

	pool.Resize(1)
	if pool.ActiveWorkers() != 1 {
		t.Fatalf("expected 1 worker, got %d", pool.ActiveWorkers())
	}

	// Let the three running jobs finish: none of them is cancelled.
	close(release)
	completed := 0
	for completed < 5 {
		select {
		case res := <-pool.Results():
			if res.Status != JobStatusCompleted {
				t.Fatalf("job %s ended with %s, expected completed", res.JobID, res.Status)
			}
			completed++
		case <-time.After(3 * time.Second):
			t.Fatalf("only %d of 5 jobs completed", completed)
		}
	}
	// Drain the start notifications of the last two jobs.
	waitStarted(t, started, 2)
}

func TestWorkerPool_ResizeShrinkThenGrowKeepsCount(t *testing.T) {
	pool := NewWorkerPool(2, 10)
	pool.Start()
	defer pool.Stop()

	started := make(chan string, 10)
	release := make(chan struct{})
	defer close(release)
	for _, j := range blockingJobs("both", 2, 20, started, release) {
		if err := pool.Submit(j); err != nil {
			t.Fatal(err)
		}
	}
	waitStarted(t, started, 2)

	// Both workers are busy: the retirement stays pending, then growing cancels it.
	pool.Resize(1)
	pool.Resize(3)
	if pool.ActiveWorkers() != 3 {
		t.Fatalf("expected 3 workers, got %d", pool.ActiveWorkers())
	}
	for _, j := range blockingJobs("more", 1, 30, started, release) {
		if err := pool.Submit(j); err != nil {
			t.Fatal(err)
		}
	}
	waitStarted(t, started, 1)
}

func TestWorkerPool_DrainQueuedKeepsRunningJobs(t *testing.T) {
	pool := NewWorkerPool(1, 10)
	pool.Start()
	defer pool.Stop()

	started := make(chan string, 10)
	release := make(chan struct{})
	jobs := blockingJobs("drain", 3, 40, started, release)
	for _, j := range jobs {
		if err := pool.Submit(j); err != nil {
			t.Fatal(err)
		}
	}
	waitStarted(t, started, 1)

	ids := pool.DrainQueued()
	if len(ids) != 2 {
		t.Fatalf("expected 2 queued jobs drained, got %v", ids)
	}
	if pool.QueueSize() != 0 {
		t.Fatalf("expected an empty queue, got %d", pool.QueueSize())
	}
	// A drained job can be submitted again (it left the registry).
	if _, ok := pool.GetJob(ids[0]); ok {
		t.Fatal("drained job still registered")
	}

	close(release)
	select {
	case res := <-pool.Results():
		if res.Status != JobStatusCompleted {
			t.Fatalf("running job ended with %s", res.Status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("running job did not complete")
	}
}

func TestWorkerPool_SharedLimiterCapsRunningJobsAcrossPools(t *testing.T) {
	limiter := NewSlotLimiter(2)
	high := NewWorkerPool(3, 10)
	low := NewWorkerPool(3, 10)
	high.SetSlotLimiter(limiter)
	low.SetSlotLimiter(limiter)
	high.SetSlotPriority(40)
	low.SetSlotPriority(10)
	high.Start()
	low.Start()
	defer high.Stop()
	defer low.Stop()

	var running, maxRunning atomic.Int32
	release := make(chan struct{})
	var releaseAll sync.Once
	defer releaseAll.Do(func() { close(release) })
	started := make(chan string, 20)
	mk := func(pool *WorkerPool, prefix string, n int, firstScene uint) {
		for i := 0; i < n; i++ {
			id := fmt.Sprintf("%s-%d", prefix, i)
			j := newTestJobWithSceneIDContext(id, firstScene+uint(i), prefix, func(ctx context.Context) error {
				cur := running.Add(1)
				for {
					m := maxRunning.Load()
					if cur <= m || maxRunning.CompareAndSwap(m, cur) {
						break
					}
				}
				started <- id
				<-release
				running.Add(-1)
				return nil
			})
			if err := pool.Submit(j); err != nil {
				t.Fatal(err)
			}
		}
	}

	// The low priority pool takes the two slots first.
	mk(low, "low", 3, 100)
	waitStarted(t, started, 2)
	waitForWaiters(t, limiter, 1)

	// Then high priority work arrives: it must get the next free slots before the third low job.
	mk(high, "high", 2, 200)
	waitForWaiters(t, limiter, 3)

	releaseOne := func() string {
		release <- struct{}{}
		select {
		case id := <-started:
			return id
		case <-time.After(2 * time.Second):
			t.Fatal("no job started after a slot was freed")
		}
		return ""
	}
	if id := releaseOne(); id[:4] != "high" {
		t.Fatalf("expected a high priority job to get the freed slot, got %s", id)
	}
	if id := releaseOne(); id[:4] != "high" {
		t.Fatalf("expected a high priority job to get the freed slot, got %s", id)
	}
	// The low job that was waiting (any of the three, workers pick them in any order) runs last.
	if id := releaseOne(); id[:3] != "low" {
		t.Fatalf("expected the waiting low priority job last, got %s", id)
	}
	releaseAll.Do(func() { close(release) })

	if maxRunning.Load() > 2 {
		t.Fatalf("expected at most 2 jobs running at once, saw %d", maxRunning.Load())
	}
}

func TestWorkerPool_GracefulStopReclaimsJobsWaitingForSlot(t *testing.T) {
	limiter := NewSlotLimiter(1)
	pool := NewWorkerPool(2, 10)
	pool.SetSlotLimiter(limiter)
	pool.Start()

	started := make(chan string, 10)
	release := make(chan struct{})
	for _, j := range blockingJobs("slot", 2, 300, started, release) {
		if err := pool.Submit(j); err != nil {
			t.Fatal(err)
		}
	}
	waitStarted(t, started, 1)
	waitForWaiters(t, limiter, 1)

	defer close(release)
	// Stopping cancels the running job and gives back the one still waiting for its slot.
	ids := pool.GracefulStop(2 * time.Second)
	if len(ids) != 1 {
		t.Fatalf("expected the job waiting for a slot to be reclaimed, got %v", ids)
	}
}
