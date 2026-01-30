package lifecycle

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestManager_Go(t *testing.T) {
	m := NewManager(zap.NewNop())

	var executed atomic.Bool
	m.Go("test", func(done <-chan struct{}) {
		executed.Store(true)
	})

	// Give the goroutine time to execute
	time.Sleep(10 * time.Millisecond)

	if !executed.Load() {
		t.Fatal("expected goroutine to execute")
	}

	err := m.Shutdown(time.Second)
	if err != nil {
		t.Fatalf("unexpected shutdown error: %v", err)
	}
}

func TestManager_GoWaitsForDone(t *testing.T) {
	m := NewManager(zap.NewNop())

	var started, finished atomic.Bool
	m.Go("test", func(done <-chan struct{}) {
		started.Store(true)
		<-done
		finished.Store(true)
	})

	// Wait for goroutine to start
	time.Sleep(10 * time.Millisecond)

	if !started.Load() {
		t.Fatal("expected goroutine to start")
	}
	if finished.Load() {
		t.Fatal("expected goroutine to not finish before shutdown")
	}

	err := m.Shutdown(time.Second)
	if err != nil {
		t.Fatalf("unexpected shutdown error: %v", err)
	}

	if !finished.Load() {
		t.Fatal("expected goroutine to finish after shutdown")
	}
}

func TestManager_ShutdownTimeout(t *testing.T) {
	m := NewManager(zap.NewNop())

	m.Go("blocking", func(done <-chan struct{}) {
		// Ignore the done signal and sleep forever
		time.Sleep(10 * time.Second)
	})

	err := m.Shutdown(50 * time.Millisecond)
	if err != context.DeadlineExceeded {
		t.Fatalf("expected DeadlineExceeded, got: %v", err)
	}
}

func TestManager_MultipleGoroutines(t *testing.T) {
	m := NewManager(zap.NewNop())

	var count atomic.Int32
	for i := 0; i < 5; i++ {
		m.Go("worker", func(done <-chan struct{}) {
			count.Add(1)
			<-done
		})
	}

	// Wait for all goroutines to start
	time.Sleep(50 * time.Millisecond)

	if count.Load() != 5 {
		t.Fatalf("expected 5 goroutines to start, got %d", count.Load())
	}

	err := m.Shutdown(time.Second)
	if err != nil {
		t.Fatalf("unexpected shutdown error: %v", err)
	}
}

func TestManager_Done(t *testing.T) {
	m := NewManager(zap.NewNop())

	select {
	case <-m.Done():
		t.Fatal("expected Done() to not be closed initially")
	default:
		// Expected
	}

	go m.Shutdown(time.Second)

	select {
	case <-m.Done():
		// Expected
	case <-time.After(time.Second):
		t.Fatal("expected Done() to be closed after shutdown")
	}
}

func TestManager_IsShuttingDown(t *testing.T) {
	m := NewManager(zap.NewNop())

	if m.IsShuttingDown() {
		t.Fatal("expected IsShuttingDown() to be false initially")
	}

	go m.Shutdown(time.Second)

	// Wait for shutdown to start
	time.Sleep(10 * time.Millisecond)

	if !m.IsShuttingDown() {
		t.Fatal("expected IsShuttingDown() to be true after shutdown")
	}
}

func TestManager_GoAfterShutdown(t *testing.T) {
	m := NewManager(zap.NewNop())

	m.Shutdown(time.Second)

	var executed atomic.Bool
	m.Go("late-goroutine", func(done <-chan struct{}) {
		executed.Store(true)
	})

	time.Sleep(10 * time.Millisecond)

	if executed.Load() {
		t.Fatal("expected goroutine to not execute after shutdown")
	}
}

func TestManager_RecoverFromPanic(t *testing.T) {
	m := NewManager(zap.NewNop())

	m.Go("panicking", func(done <-chan struct{}) {
		panic("test panic")
	})

	// Give the goroutine time to panic and recover
	time.Sleep(10 * time.Millisecond)

	// Should still be able to shutdown gracefully
	err := m.Shutdown(time.Second)
	if err != nil {
		t.Fatalf("unexpected shutdown error: %v", err)
	}
}

func TestManager_DoubleShutdown(t *testing.T) {
	m := NewManager(zap.NewNop())

	err1 := m.Shutdown(time.Second)
	if err1 != nil {
		t.Fatalf("unexpected first shutdown error: %v", err1)
	}

	// Second shutdown should be a no-op
	err2 := m.Shutdown(time.Second)
	if err2 != nil {
		t.Fatalf("unexpected second shutdown error: %v", err2)
	}
}
