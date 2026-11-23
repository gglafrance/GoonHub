package worker

import (
	"context"
	"errors"
	"net/http"
	"time"

	"goonhub/pkg/workerproto"

	"goonhub-worker/internal/client"
)

// claimLoop is pull mode: claim jobs for the free slots with a long-poll, back off on
// errors, stop at the graceful stop.
func (w *Worker) claimLoop() {
	ctx, cancel := context.WithCancel(w.killCtx)
	defer cancel()
	go func() {
		select {
		case <-w.stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()
	bo := w.backoff
	for {
		if ctx.Err() != nil {
			return
		}
		w.mu.Lock()
		slots := w.freeSlotsLocked()
		w.mu.Unlock()
		if len(slots) == 0 {
			select {
			case <-ctx.Done():
				return
			case <-w.slotFreed:
			case <-time.After(5 * time.Second):
			}
			continue
		}
		sess := w.sessionSnapshot()
		wait := sess.claimWait
		if wait <= 0 {
			wait = 25
		}
		w.log.Debug("claiming", "slots", slots, "wait_sec", wait)
		resp, err := w.client.Claim(ctx, &workerproto.ClaimRequest{SessionID: sess.id, Slots: slots, WaitSec: wait})
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			delay := bo.Next()
			switch {
			case client.IsCode(err, workerproto.CodeUnknownSession):
				w.log.Warn("claim: server forgot the session: registering again")
				if rerr := w.reregister(ctx, sess.id); rerr == nil {
					continue
				} else if errors.Is(rerr, ErrFatal) {
					w.log.Error("cannot register", "error", rerr)
				}
			case client.StatusOf(err) == http.StatusUnauthorized || client.StatusOf(err) == http.StatusForbidden ||
				client.IsCode(err, workerproto.CodeWorkersDisabled):
				w.log.Error("claim refused (token revoked, worker disabled, address not allowed or worker API off): retrying later", "error", err)
				delay = max(delay, time.Minute)
			default:
				w.log.Warn("claim failed, retrying", "error", err, "retry_in", delay.Round(time.Millisecond))
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
			continue
		}
		bo.Reset()
		w.noteContact(time.Time{})
		asked := map[workerproto.Phase]int{}
		for p, n := range slots {
			asked[p] = n
		}
		for _, a := range resp.Assignments {
			if asked[a.Phase] <= 0 {
				// More than asked: give it back without counting an attempt.
				w.log.Warn("server assigned more jobs than asked: releasing the extra", "job_id", a.JobID, "phase", a.Phase)
				w.releaseUnaccepted(a, workerproto.ReleaseOverloaded)
				continue
			}
			asked[a.Phase]--
			w.intake(a)
		}
	}
}

// intake accepts an assignment from a claim; refused ones are given back.
func (w *Worker) intake(a workerproto.Assignment) {
	err := w.accept(a)
	if err == nil {
		return
	}
	var inv *errInvalidAssignment
	switch {
	case errors.As(err, &inv):
		w.log.Error("assignment refused", "job_id", a.JobID, "phase", a.Phase, "error", err)
		w.failUnaccepted(a, workerproto.FailureUnsupported, err)
	case errors.Is(err, errStale):
	case errors.Is(err, errBusy):
		w.releaseUnaccepted(a, workerproto.ReleaseOverloaded)
	default:
		// Local problem (outbox write: disk full...).
		w.log.Error("cannot take the assignment", "job_id", a.JobID, "error", err)
		w.releaseUnaccepted(a, workerproto.ReleaseDiskFull)
	}
}

// releaseUnaccepted releases a lease the worker never started.
func (w *Worker) releaseUnaccepted(a workerproto.Assignment, reason workerproto.ReleaseReason) {
	ctx, cancel := context.WithTimeout(w.killCtx, 10*time.Second)
	defer cancel()
	if err := w.client.Release(ctx, a.JobID, &workerproto.ReleaseRequest{Generation: a.Lease.Generation, Reason: reason}); err != nil && !client.IsLeaseGone(err) {
		w.log.Warn("release failed (the lease will expire)", "job_id", a.JobID, "error", err)
	}
}

// failUnaccepted reports an assignment the worker cannot run at all.
func (w *Worker) failUnaccepted(a workerproto.Assignment, kind workerproto.FailureKind, cause error) {
	ctx, cancel := context.WithTimeout(w.killCtx, 10*time.Second)
	defer cancel()
	if err := w.client.Fail(ctx, a.JobID, &workerproto.FailRequest{Generation: a.Lease.Generation, Kind: kind, Error: cause.Error(), Retryable: false}); err != nil && !client.IsLeaseGone(err) {
		w.log.Warn("fail report failed (the lease will expire)", "job_id", a.JobID, "error", err)
	}
}
