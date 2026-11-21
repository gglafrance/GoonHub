package jobs

import (
	"context"
	"sync"
	"testing"
	"time"
)

// waitForWaiters blocks until the limiter has n waiters.
func waitForWaiters(t *testing.T, l *SlotLimiter, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for l.Waiting() != n {
		if time.Now().After(deadline) {
			t.Fatalf("expected %d waiters, got %d", n, l.Waiting())
		}
		time.Sleep(time.Millisecond)
	}
}

func TestSlotLimiter_UnlimitedNeverBlocks(t *testing.T) {
	l := NewSlotLimiter(0)
	for i := 0; i < 100; i++ {
		if err := l.Acquire(context.Background(), 0); err != nil {
			t.Fatalf("acquire %d: %v", i, err)
		}
	}
	if l.InUse() != 100 {
		t.Fatalf("expected 100 in use, got %d", l.InUse())
	}
}

func TestSlotLimiter_GrantsByPriorityThenArrival(t *testing.T) {
	l := NewSlotLimiter(1)
	if err := l.Acquire(context.Background(), 0); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var order []string
	var wg sync.WaitGroup
	start := func(name string, priority int) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := l.Acquire(context.Background(), priority); err != nil {
				t.Errorf("acquire %s: %v", name, err)
				return
			}
			mu.Lock()
			order = append(order, name)
			mu.Unlock()
			l.Release()
		}()
	}

	// Arrival order: low, high-1, mid, high-2
	start("low", 10)
	waitForWaiters(t, l, 1)
	start("high-1", 40)
	waitForWaiters(t, l, 2)
	start("mid", 20)
	waitForWaiters(t, l, 3)
	start("high-2", 40)
	waitForWaiters(t, l, 4)

	l.Release()
	wg.Wait()

	want := []string{"high-1", "high-2", "mid", "low"}
	if len(order) != len(want) {
		t.Fatalf("got order %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("got order %v, want %v", order, want)
		}
	}
}

func TestSlotLimiter_CancelledWaiterLeavesQueue(t *testing.T) {
	l := NewSlotLimiter(1)
	if err := l.Acquire(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- l.Acquire(ctx, 5) }()
	waitForWaiters(t, l, 1)
	cancel()
	if err := <-done; err == nil {
		t.Fatal("expected an error from a cancelled acquire")
	}
	if l.Waiting() != 0 || l.InUse() != 1 {
		t.Fatalf("expected 0 waiters and 1 in use, got %d and %d", l.Waiting(), l.InUse())
	}
	l.Release()
	if l.InUse() != 0 {
		t.Fatalf("expected 0 in use, got %d", l.InUse())
	}
}

func TestSlotLimiter_RaisingLimitWakesWaiters(t *testing.T) {
	l := NewSlotLimiter(1)
	if err := l.Acquire(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{}, 2)
	for i := 0; i < 2; i++ {
		go func() {
			if err := l.Acquire(context.Background(), 0); err == nil {
				done <- struct{}{}
			}
		}()
	}
	waitForWaiters(t, l, 2)

	l.SetLimit(3)
	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("waiters not woken after raising the limit")
		}
	}

	// Lowering the limit below the slots in use only delays new acquisitions.
	l.SetLimit(1)
	if l.InUse() != 3 {
		t.Fatalf("expected 3 in use, got %d", l.InUse())
	}
	l.SetLimit(0)
	if err := l.Acquire(context.Background(), 0); err != nil {
		t.Fatalf("unlimited acquire: %v", err)
	}
}
