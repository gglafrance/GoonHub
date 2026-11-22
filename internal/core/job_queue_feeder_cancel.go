package core

import (
	"fmt"
	"strings"
	"sync"

	"goonhub/internal/apperrors"

	"go.uber.org/zap"
)

// QueuePhases are the processing phases that go through the DB-backed job queue.
var QueuePhases = []string{"metadata", "thumbnail", "sprites", "animated_thumbnails", "fingerprint"}

// BulkCancelAllPhases selects every queue phase in CancelBulk.
const BulkCancelAllPhases = "all"

// bulkCancelReason is the error message stored on the jobs cancelled by a bulk cancel.
const bulkCancelReason = "Cancelled by admin (bulk cancel)"

// BulkCancelResult reports what a bulk cancel did.
type BulkCancelResult struct {
	Phases []string `json:"phases"`
	// PendingCancelled counts the pending jobs cancelled in the database.
	PendingCancelled int64 `json:"pending_cancelled"`
	// QueuedSkipped counts the jobs claimed by the feeder but not started (waiting in a worker
	// pool queue or for a slot): they will not run.
	QueuedSkipped int `json:"queued_skipped"`
	// RunningCancelled counts the running jobs asked to stop (only with include_running).
	RunningCancelled int  `json:"running_cancelled"`
	IncludeRunning   bool `json:"include_running"`
}

func newFeedLocks() map[string]*sync.Mutex {
	locks := make(map[string]*sync.Mutex, len(QueuePhases))
	for _, phase := range QueuePhases {
		locks[phase] = &sync.Mutex{}
	}
	return locks
}

// feedLockFallback guards the phases outside QueuePhases (none in practice).
var feedLockFallback sync.Mutex

func (f *JobQueueFeeder) feedLock(phase string) *sync.Mutex {
	if lock, ok := f.feedLocks[phase]; ok {
		return lock
	}
	return &feedLockFallback
}

// SetEventBus sets the event bus used to announce bulk cancellations. Optional.
func (f *JobQueueFeeder) SetEventBus(eventBus *EventBus) {
	f.eventBus = eventBus
}

// ResolveBulkCancelPhases turns the phase of a bulk cancel request ("all" or a queue phase)
// into the list of phases to cancel.
func ResolveBulkCancelPhases(phase string) ([]string, error) {
	if phase == BulkCancelAllPhases {
		return append([]string(nil), QueuePhases...), nil
	}
	for _, p := range QueuePhases {
		if p == phase {
			return []string{phase}, nil
		}
	}
	return nil, apperrors.NewValidationErrorWithField("phase",
		fmt.Sprintf("phase must be %q or one of: %s", BulkCancelAllPhases, strings.Join(QueuePhases, ", ")))
}

// CancelBulk cancels the jobs of a phase (or of every phase with "all"):
//   - all its pending jobs, in a single UPDATE;
//   - the jobs already claimed but not started: they are taken out of the worker pool queue
//     and their rows cancelled, and the ones a worker holds while waiting for a slot are
//     cancelled in memory (they return at once when they get it);
//   - with includeRunning, its running jobs, through the per-job cancel mechanism (the result
//     handler records them as cancelled when they stop).
//
// The feed of the phases is held during the operation, so the feeder cannot claim jobs while
// they are being cancelled.
func (f *JobQueueFeeder) CancelBulk(phase string, includeRunning bool) (*BulkCancelResult, error) {
	phases, err := ResolveBulkCancelPhases(phase)
	if err != nil {
		return nil, err
	}

	// Locks are always taken in QueuePhases order; feeders only hold one at a time.
	for _, p := range phases {
		f.feedLock(p).Lock()
	}
	defer func() {
		for _, p := range phases {
			f.feedLock(p).Unlock()
		}
	}()

	result := &BulkCancelResult{Phases: phases, IncludeRunning: includeRunning}

	pending, err := f.repo.CancelPendingJobsByPhases(phases, bulkCancelReason)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to cancel pending jobs", err)
	}
	result.PendingCancelled = pending

	var drained []string
	for _, p := range phases {
		drained = append(drained, f.poolManager.DrainQueued(p)...)
	}
	result.QueuedSkipped = len(drained)
	if len(drained) > 0 {
		if _, err := f.repo.CancelClaimedJobs(drained, bulkCancelReason); err != nil {
			// The jobs are out of the queues and will not run; their rows stay 'running' until
			// the orphan recovery of the next start marks them failed.
			f.logger.Error("Failed to mark queued jobs as cancelled",
				zap.Int("count", len(drained)), zap.Error(err))
			return nil, apperrors.NewInternalError("failed to cancel queued jobs", err)
		}
	}

	for _, p := range phases {
		waiting, running := f.poolManager.CancelRegistered(p, includeRunning)
		result.QueuedSkipped += waiting
		result.RunningCancelled += running
	}

	f.logger.Info("Bulk cancelled jobs",
		zap.Strings("phases", phases),
		zap.Int64("pending_cancelled", result.PendingCancelled),
		zap.Int("queued_skipped", result.QueuedSkipped),
		zap.Int("running_cancelled", result.RunningCancelled),
		zap.Bool("include_running", includeRunning),
	)

	if f.eventBus != nil {
		f.eventBus.Publish(SceneEvent{
			Type: "jobs:bulk_cancelled",
			Data: map[string]any{
				"phases":            phases,
				"pending_cancelled": result.PendingCancelled,
				"queued_skipped":    result.QueuedSkipped,
				"running_cancelled": result.RunningCancelled,
			},
		})
	}

	return result, nil
}
