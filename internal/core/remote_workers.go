package core

import (
	"time"

	"goonhub/internal/core/processing"
	"goonhub/internal/jobs"

	"go.uber.org/zap"
)

// Hooks of the remote workers (internal/workers, docs/REMOTE_WORKERS.md) into the local
// processing: phase routing for the feeder and the result handler, remote cancellation,
// retry policy and the job schedule for remote claims.

// PhaseRouter says whether the local pools and the remote workers may claim a phase.
type PhaseRouter = processing.PhaseRouter

// RemoteJobCanceller cancels the jobs leased to remote workers.
type RemoteJobCanceller interface {
	// CancelJob cancels one running remote job (false: not a remote job).
	CancelJob(jobID string) (bool, error)
	// CancelPhases cancels every running remote job of the phases.
	CancelPhases(phases []string) (int, error)
}

// RemoteOnlyPhaseLister lists the phases only remote workers process.
type RemoteOnlyPhaseLister interface {
	RemoteOnlyPhases() []string
}

// ---------------------------------------------------------------------------------------
// JobQueueFeeder
// ---------------------------------------------------------------------------------------

// SetPhaseRouter sets the routing of the phases (nil = every phase local). Must be called
// before Start.
func (f *JobQueueFeeder) SetPhaseRouter(r PhaseRouter) {
	f.router = r
}

// SetRemoteCanceller sets who cancels the running remote jobs in a bulk cancel.
func (f *JobQueueFeeder) SetRemoteCanceller(c RemoteJobCanceller) {
	f.remoteCanceller = c
}

// localAllowed reports whether the local pool of a phase may claim its jobs. When it may not
// (remote-only phase), the jobs claimed but still queued go back to pending once.
func (f *JobQueueFeeder) localAllowed(phase string) bool {
	if f.router == nil || f.router.LocalAllowed(phase) {
		f.pausedMu.Lock()
		delete(f.remoteOnly, phase)
		f.pausedMu.Unlock()
		return true
	}
	f.pausedMu.Lock()
	was := f.remoteOnly[phase]
	if f.remoteOnly == nil {
		f.remoteOnly = map[string]bool{}
	}
	f.remoteOnly[phase] = true
	f.pausedMu.Unlock()
	if !was {
		reclaimed := int64(0)
		if ids := f.poolManager.DrainQueued(phase); len(ids) > 0 {
			n, err := f.repo.ResetJobsToPending(ids)
			if err != nil {
				f.logger.Error("Failed to put queued jobs back to pending", zap.String("phase", phase), zap.Error(err))
			}
			reclaimed = n
		}
		f.logger.Info("Phase routed to remote workers only, the local pool stops claiming",
			zap.String("phase", phase), zap.Int64("queued_jobs_back_to_pending", reclaimed))
	}
	return false
}

// remoteOnlyPhases lists the phases the stuck-pending recovery must not fail.
func (f *JobQueueFeeder) remoteOnlyPhases() []string {
	if lister, ok := f.router.(RemoteOnlyPhaseLister); ok && lister != nil {
		return lister.RemoteOnlyPhases()
	}
	return nil
}

// RemoteClaimOptions applies the job schedule to remote claims: whether the phase is inside
// its time window now, and since when scenes count as new (zero = "new scenes first" off).
func (f *JobQueueFeeder) RemoteClaimOptions(phase string) (bool, time.Time) {
	plan := f.currentPlan()
	if plan == nil {
		return true, time.Time{}
	}
	pp := plan.Phase(phase)
	return pp.Allowed, plan.NewSceneSince
}

// ---------------------------------------------------------------------------------------
// SceneProcessingService
// ---------------------------------------------------------------------------------------

// SetRemoteCanceller sets who cancels the jobs leased to remote workers (CancelJob).
func (s *SceneProcessingService) SetRemoteCanceller(c RemoteJobCanceller) {
	s.remoteCanceller = c
}

// SetPhaseRouter sets the phase routing used after metadata (sprites go through the queue
// when remote workers may take them).
func (s *SceneProcessingService) SetPhaseRouter(r PhaseRouter) {
	s.resultHandler.SetPhaseRouter(r)
}

// HandleResult runs the post-completion of a job (job history, events, phase tracking,
// triggered phases), for the remote workers.
func (s *SceneProcessingService) HandleResult(result jobs.JobResult) {
	s.resultHandler.HandleResult(result)
}

// ---------------------------------------------------------------------------------------
// RetryScheduler
// ---------------------------------------------------------------------------------------

// MaxRetries returns the configured retries of a phase.
func (rs *RetryScheduler) MaxRetries(phase string) int {
	return rs.GetConfigForPhase(phase).MaxRetries
}

// ToDeadLetter sends a failed job to the dead letter queue (a non-retryable remote failure).
func (rs *RetryScheduler) ToDeadLetter(jobID, phase string, sceneID uint, retryCount int, message string) error {
	if err := rs.jobHistoryRepo.UpdateRetryInfo(jobID, retryCount, rs.MaxRetries(phase), nil); err != nil {
		rs.logger.Warn("Failed to update final retry info before DLQ", zap.String("job_id", jobID), zap.Error(err))
	}
	return rs.moveToDLQ(jobID, phase, sceneID, message, retryCount)
}
