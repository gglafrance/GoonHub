// Package lifecycle provides goroutine lifecycle management for graceful shutdown.
package lifecycle

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager manages goroutine lifecycles for graceful shutdown coordination.
// It provides a way to track goroutines and wait for them to complete during shutdown.
type Manager struct {
	wg       sync.WaitGroup
	shutdown chan struct{}
	mu       sync.RWMutex
	started  bool
	logger   *zap.Logger
}

// NewManager creates a new lifecycle manager.
func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		shutdown: make(chan struct{}),
		logger:   logger,
	}
}

// Go starts a supervised goroutine that will be tracked for shutdown.
// The function receives a done channel that will be closed when shutdown is initiated.
// The name is used for logging purposes.
func (m *Manager) Go(name string, fn func(done <-chan struct{})) {
	m.mu.RLock()
	if m.started {
		m.mu.RUnlock()
		m.logger.Warn("Cannot start goroutine after shutdown initiated", zap.String("name", name))
		return
	}
	m.mu.RUnlock()

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				m.logger.Error("Goroutine panicked",
					zap.String("name", name),
					zap.Any("panic", r),
				)
			}
		}()

		m.logger.Debug("Starting managed goroutine", zap.String("name", name))
		fn(m.shutdown)
		m.logger.Debug("Managed goroutine completed", zap.String("name", name))
	}()
}

// GoWithContext starts a supervised goroutine with a context.
// The context is cancelled when shutdown is initiated.
func (m *Manager) GoWithContext(ctx context.Context, name string, fn func(ctx context.Context)) {
	childCtx, cancel := context.WithCancel(ctx)

	m.Go(name, func(done <-chan struct{}) {
		defer cancel()

		select {
		case <-done:
			cancel()
		case <-childCtx.Done():
		}
	})

	// Start the actual work in a nested goroutine
	go func() {
		defer cancel()
		fn(childCtx)
	}()
}

// Done returns a channel that is closed when shutdown is initiated.
// This can be used by goroutines to check for shutdown.
func (m *Manager) Done() <-chan struct{} {
	return m.shutdown
}

// Shutdown initiates graceful shutdown and waits for all goroutines to complete.
// Returns an error if the timeout is exceeded.
func (m *Manager) Shutdown(timeout time.Duration) error {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return nil // Already shutting down
	}
	m.started = true
	m.mu.Unlock()

	m.logger.Info("Initiating graceful shutdown")
	close(m.shutdown)

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		m.logger.Info("All goroutines completed gracefully")
		return nil
	case <-time.After(timeout):
		m.logger.Warn("Shutdown timeout exceeded, some goroutines may not have completed",
			zap.Duration("timeout", timeout),
		)
		return context.DeadlineExceeded
	}
}

// IsShuttingDown returns true if shutdown has been initiated.
func (m *Manager) IsShuttingDown() bool {
	select {
	case <-m.shutdown:
		return true
	default:
		return false
	}
}
