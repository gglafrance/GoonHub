package jobs

import (
	"context"
	"sync"
)

// SlotLimiter caps the number of jobs executing at the same time across several worker pools.
// When every slot is taken, waiting workers are served by priority (higher first), then in
// arrival order. A limit of 0 means unlimited.
type SlotLimiter struct {
	mu      sync.Mutex
	limit   int
	inUse   int
	seq     uint64
	waiters []*slotWaiter
}

type slotWaiter struct {
	ch       chan struct{}
	priority int
	seq      uint64
	granted  bool
}

// NewSlotLimiter creates a limiter with the given limit (0 = unlimited).
func NewSlotLimiter(limit int) *SlotLimiter {
	if limit < 0 {
		limit = 0
	}
	return &SlotLimiter{limit: limit}
}

// Acquire blocks until a slot is free or ctx is done. Every successful Acquire must be
// followed by exactly one Release.
func (l *SlotLimiter) Acquire(ctx context.Context, priority int) error {
	l.mu.Lock()
	if l.hasFreeSlotLocked() && len(l.waiters) == 0 {
		l.inUse++
		l.mu.Unlock()
		return nil
	}
	l.seq++
	w := &slotWaiter{ch: make(chan struct{}), priority: priority, seq: l.seq}
	l.waiters = append(l.waiters, w)
	l.mu.Unlock()

	select {
	case <-w.ch:
		return nil
	case <-ctx.Done():
		l.mu.Lock()
		if w.granted {
			// Granted while giving up: hand the slot to the next waiter.
			l.inUse--
			l.grantLocked()
		} else {
			l.removeWaiterLocked(w)
		}
		l.mu.Unlock()
		return ctx.Err()
	}
}

// Release frees a slot taken by Acquire.
func (l *SlotLimiter) Release() {
	l.mu.Lock()
	if l.inUse > 0 {
		l.inUse--
	}
	l.grantLocked()
	l.mu.Unlock()
}

// SetLimit changes the limit (0 = unlimited). Lowering it never interrupts running jobs: new
// jobs simply wait until enough of them have finished.
func (l *SlotLimiter) SetLimit(limit int) {
	if limit < 0 {
		limit = 0
	}
	l.mu.Lock()
	l.limit = limit
	l.grantLocked()
	l.mu.Unlock()
}

// Limit returns the current limit (0 = unlimited).
func (l *SlotLimiter) Limit() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.limit
}

// InUse returns the number of slots currently taken.
func (l *SlotLimiter) InUse() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.inUse
}

// Waiting returns the number of workers waiting for a slot.
func (l *SlotLimiter) Waiting() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.waiters)
}

func (l *SlotLimiter) hasFreeSlotLocked() bool {
	return l.limit <= 0 || l.inUse < l.limit
}

// grantLocked hands free slots to the best waiters.
func (l *SlotLimiter) grantLocked() {
	for l.hasFreeSlotLocked() && len(l.waiters) > 0 {
		best := 0
		for i, w := range l.waiters[1:] {
			b := l.waiters[best]
			if w.priority > b.priority || (w.priority == b.priority && w.seq < b.seq) {
				best = i + 1
			}
		}
		w := l.waiters[best]
		l.waiters = append(l.waiters[:best], l.waiters[best+1:]...)
		w.granted = true
		l.inUse++
		close(w.ch)
	}
}

func (l *SlotLimiter) removeWaiterLocked(target *slotWaiter) {
	for i, w := range l.waiters {
		if w == target {
			l.waiters = append(l.waiters[:i], l.waiters[i+1:]...)
			return
		}
	}
}
