package worker

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"time"

	"goonhub/pkg/workerproto"

	"goonhub-worker/internal/backoff"
	"goonhub-worker/internal/client"
)

// ErrFatal wraps errors that make retrying pointless (unsupported protocol version).
var ErrFatal = errors.New("fatal")

func (w *Worker) sessionID() string {
	w.sessMu.RLock()
	defer w.sessMu.RUnlock()
	return w.sess.id
}

func (w *Worker) sessionSnapshot() session {
	w.sessMu.RLock()
	defer w.sessMu.RUnlock()
	return w.sess
}

// serverNow is the server clock as estimated from the last answer.
func (w *Worker) serverNow() time.Time {
	w.sessMu.RLock()
	defer w.sessMu.RUnlock()
	return time.Now().Add(w.sess.offset)
}

// localDeadline converts a server lease expiry to the local clock.
func (w *Worker) localDeadline(serverExpiry time.Time) time.Time {
	w.sessMu.RLock()
	defer w.sessMu.RUnlock()
	return serverExpiry.Add(-w.sess.offset)
}

func (w *Worker) noteContact(serverTime time.Time) {
	w.sessMu.Lock()
	defer w.sessMu.Unlock()
	now := time.Now()
	w.sess.lastContact = now
	if !serverTime.IsZero() {
		w.sess.offset = serverTime.Sub(now)
	}
}

func (w *Worker) capabilities() workerproto.Capabilities {
	slots := map[workerproto.Phase]int{}
	for _, p := range w.cfg.EnabledPhases() {
		slots[p] = w.cfg.Phases[p]
	}
	modes := []workerproto.InputMode{workerproto.InputIndirect}
	if len(w.cfg.Storage) > 0 {
		modes = []workerproto.InputMode{workerproto.InputDirect, workerproto.InputIndirect}
	}
	return workerproto.Capabilities{
		Phases:           w.cfg.EnabledPhases(),
		Slots:            slots,
		InputModes:       modes,
		Accel:            w.env.Accel.Label(),
		HWAccels:         w.tools.HWAccels,
		Encoders:         w.tools.Encoders,
		AudioFingerprint: w.tools.HasAudioFingerprint(),
		FFmpegVersion:    w.tools.FFmpegVersion,
	}
}

// pathMappings lists the prefix mappings whose mount is present and readable (the server
// only trusts checked prefixes).
func (w *Worker) pathMappings() []workerproto.PathMapping {
	var out []workerproto.PathMapping
	for _, m := range w.cfg.Storage {
		if m.ServerPrefix == "" {
			continue
		}
		if st, err := os.Stat(m.LocalPath); err != nil || !st.IsDir() {
			continue
		}
		out = append(out, workerproto.PathMapping{ServerPrefix: m.ServerPrefix, LocalPrefix: m.LocalPath})
	}
	return out
}

// registerLoop registers until it works, the worker stops, or the server refuses the
// protocol version.
func (w *Worker) registerLoop() error {
	bo := w.backoff
	for {
		err := w.register(w.killCtx)
		if err == nil {
			return nil
		}
		if errors.Is(err, ErrFatal) {
			return err
		}
		if w.killed() {
			return nil
		}
		delay := bo.Next()
		switch {
		case client.StatusOf(err) == http.StatusUnauthorized:
			w.log.Error("the server refused the worker token (401): check token / token_file; the outbox is kept", "error", err)
			delay = max(delay, time.Minute)
		case client.IsCode(err, workerproto.CodeAddressNotAllowed):
			w.log.Error("the server refused this machine's address (403 address_not_allowed): connect through Tailscale or add the address to workers.allowed_cidrs on the server; the outbox is kept", "error", err)
			delay = max(delay, time.Minute)
		case client.IsCode(err, workerproto.CodeWorkerDisabled) || client.StatusOf(err) == http.StatusForbidden:
			w.log.Error("the server refused this worker (403: disabled by the admin); the outbox is kept", "error", err)
			delay = max(delay, time.Minute)
		case client.IsCode(err, workerproto.CodeWorkersDisabled) || client.StatusOf(err) == http.StatusNotFound:
			w.log.Error("the worker API is off on the server (404: set workers.enabled / GOONHUB_WORKERS_ENABLED=true) or the server URL is wrong; retrying", "error", err)
			delay = max(delay, time.Minute)
		default:
			w.log.Warn("register failed, retrying", "error", err, "retry_in", delay.Round(time.Millisecond))
		}
		select {
		case <-time.After(delay):
		case <-w.stopCh:
			// Never registered: nothing to release; the outbox stays for the next start.
			return nil
		case <-w.killCtx.Done():
			return nil
		}
	}
}

// reregister opens a new session after the server forgot stale (unknown_session); a
// concurrent caller that already re-registered makes it a no-op.
func (w *Worker) reregister(ctx context.Context, stale string) error {
	w.regMu.Lock()
	defer w.regMu.Unlock()
	if w.sessionID() != stale {
		return nil
	}
	return w.registerLocked(ctx)
}

// register opens a new session, resuming every job the worker holds (single-flight).
func (w *Worker) register(ctx context.Context) error {
	w.regMu.Lock()
	defer w.regMu.Unlock()
	return w.registerLocked(ctx)
}

func (w *Worker) registerLocked(ctx context.Context) error {
	w.mu.Lock()
	resume := make([]workerproto.JobLeaseRef, 0, len(w.jobs))
	for _, j := range w.jobs {
		resume = append(resume, workerproto.JobLeaseRef{JobID: j.id, Generation: j.gen})
	}
	w.mu.Unlock()

	req := &workerproto.RegisterRequest{
		APIVersion:     workerproto.APIVersion,
		Name:           w.cfg.Name,
		Version:        w.version,
		Hostname:       w.hostname,
		OS:             runtime.GOOS,
		Arch:           runtime.GOARCH,
		ConnectionMode: w.cfg.Mode,
		Capabilities:   w.capabilities(),
		PathMappings:   w.pathMappings(),
		Resume:         resume,
	}
	if w.cfg.Mode == workerproto.ConnPush {
		req.PushURL = w.cfg.Push.URL
		req.PushToken = w.pushToken
	}
	resp, err := w.client.Register(ctx, req)
	if err != nil {
		if client.IsCode(err, workerproto.CodeUnsupportedAPIVersion) || client.StatusOf(err) == http.StatusUpgradeRequired {
			return fmt.Errorf("%w: the server does not support worker API version %d: update goonhub-worker or the server: %v", ErrFatal, workerproto.APIVersion, err)
		}
		return err
	}
	if resp.SessionID == "" {
		return errors.New("register: the server returned no session")
	}
	w.mu.Lock()
	w.setServerMaxLocked(resp.MaxConcurrency)
	w.mu.Unlock()
	w.sessMu.Lock()
	w.sess = session{
		id:               resp.SessionID,
		workerID:         resp.WorkerID,
		heartbeat:        secs(resp.HeartbeatIntervalSec, 10),
		lease:            secs(resp.LeaseDurationSec, 60),
		claimWait:        resp.ClaimMaxWaitSec,
		maxArtifactBytes: resp.MaxArtifactBytes,
		chunkBytes:       resp.UploadChunkBytes,
		lastContact:      time.Now(),
	}
	if !resp.ServerTime.IsZero() {
		w.sess.offset = time.Until(resp.ServerTime)
	}
	if w.sess.chunkBytes <= 0 {
		w.sess.chunkBytes = 16 << 20
	}
	w.sessMu.Unlock()
	w.log.Info("registered", "worker_id", resp.WorkerID, "heartbeat", secs(resp.HeartbeatIntervalSec, 10),
		"lease", secs(resp.LeaseDurationSec, 60), "resumed", len(resp.Resumed))
	w.applyResumed(resume, resp.Resumed)
	w.regOnce.Do(func() { close(w.registered) })
	w.signalSlot()
	return nil
}

func secs(n, def int) time.Duration {
	if n <= 0 {
		n = def
	}
	return time.Duration(n) * time.Second
}

// applyResumed handles the server's answer about every job sent in resume.
func (w *Worker) applyResumed(sent []workerproto.JobLeaseRef, answers []workerproto.JobLeaseStatus) {
	byID := map[string]workerproto.JobLeaseStatus{}
	for _, a := range answers {
		byID[a.JobID] = a
	}
	for _, ref := range sent {
		w.mu.Lock()
		j := w.jobs[ref.JobID]
		w.mu.Unlock()
		if j == nil || j.gen != ref.Generation {
			continue
		}
		st, ok := byID[ref.JobID]
		if !ok || st.Generation != j.gen {
			st.Status = workerproto.LeaseLost
		}
		switch st.Status {
		case workerproto.LeaseOK:
			if st.LeaseExpiresAt != nil {
				j.setDeadline(w.localDeadline(*st.LeaseExpiresAt))
			}
			if j.recovered && !j.started.Load() {
				w.startRecovered(j)
			}
		default:
			j.log.Info("server ended the lease at register", "status", st.Status)
			w.discard(j)
		}
	}
}

// startRecovered continues a job reloaded from the outbox whose lease is still valid.
func (w *Worker) startRecovered(j *job) {
	st := j.entry.State
	switch {
	case st.ReleasePending != "":
		j.log.Info("releasing a job a previous stop could not release", "reason", st.ReleasePending)
		w.launch(j, func() { w.release(j, st.ReleasePending) })
	case st.Stage.Resumable() && st.Complete != nil:
		j.log.Info("resuming upload and complete of a computed job", "stage", st.Stage)
		w.mu.Lock()
		w.delivering++
		j.delivering.Store(true)
		w.mu.Unlock()
		w.launch(j, func() { w.deliverAndFinish(j) })
	default:
		// The computation was cut by the crash: its output is partial.
		j.log.Info("releasing a job whose computation was interrupted by a crash", "reason", workerproto.ReleaseCrashRecovery)
		w.launch(j, func() { w.release(j, workerproto.ReleaseCrashRecovery) })
	}
}

// discard drops a job the server no longer gives us: running ones are cancelled (their
// goroutine removes the entry), idle ones are removed now.
func (w *Worker) discard(j *job) {
	if j.started.Load() {
		j.cancel(errDiscard)
		return
	}
	w.finish(j)
}

// loadOutbox reloads the jobs a previous run left on disk.
func (w *Worker) loadOutbox() {
	entries, errs := w.outbox.Load()
	for _, err := range errs {
		w.log.Error("outbox entry unreadable (left on disk)", "error", err)
	}
	for _, e := range entries {
		j := w.newJob(e)
		j.recovered = true
		w.mu.Lock()
		w.jobs[j.id] = j
		w.mu.Unlock()
		j.log.Info("outbox entry reloaded", "stage", e.State.Stage, "release_pending", e.State.ReleasePending)
	}
}

// heartbeatLoop renews every lease each heartbeat interval, re-registers on
// unknown_session, and stops computations once the lease is certainly gone.
func (w *Worker) heartbeatLoop(ctx context.Context) {
	bo := backoff.Backoff{Min: w.backoff.Min, Max: 10 * time.Second}
	if bo.Min <= 0 {
		bo.Min = time.Second
	}
	next := w.sessionSnapshot().heartbeat
	for {
		stale := w.sessionID()
		select {
		case <-ctx.Done():
			return
		case <-time.After(next):
		}
		next = w.sessionSnapshot().heartbeat
		err := w.heartbeat(ctx)
		if err == nil {
			bo.Reset()
			continue
		}
		if ctx.Err() != nil {
			return
		}
		if client.IsCode(err, workerproto.CodeUnknownSession) {
			w.log.Warn("server forgot the session (restart?): registering again")
			if rerr := w.reregister(ctx, stale); rerr != nil {
				w.log.Warn("re-register failed", "error", rerr)
				next = min(bo.Next(), next)
			} else {
				next = 0
			}
			continue
		}
		w.checkLeaseGone()
		d := bo.Next()
		w.log.Warn("heartbeat failed, retrying", "error", err, "retry_in", d.Round(time.Millisecond))
		next = min(d, next)
	}
}

func (w *Worker) heartbeat(ctx context.Context) error {
	free := w.updateDisk()
	w.mu.Lock()
	req := &workerproto.HeartbeatRequest{
		SessionID: w.sessionID(),
		Jobs:      make([]workerproto.JobHeartbeat, 0, len(w.jobs)),
		Draining:  w.stopping.Load(),
	}
	if free >= 0 {
		req.FreeDiskBytes = free
	}
	byID := map[string]*job{}
	for _, j := range w.jobs {
		req.Jobs = append(req.Jobs, j.heartbeat())
		byID[j.id] = j
	}
	w.mu.Unlock()

	resp, err := w.client.Heartbeat(ctx, req)
	if err != nil {
		return err
	}
	w.noteContact(resp.ServerTime)
	w.mu.Lock()
	if resp.Drain != w.drain {
		if resp.Drain {
			w.log.Info("server asked to drain: no new jobs, running jobs finish")
		} else {
			w.log.Info("drain lifted: taking jobs again")
		}
	}
	w.drain = resp.Drain
	w.setServerMaxLocked(resp.MaxConcurrency)
	w.mu.Unlock()
	w.signalSlot()
	for _, st := range resp.Jobs {
		j := byID[st.JobID]
		if j == nil || st.Generation != j.gen {
			continue
		}
		switch st.Status {
		case workerproto.LeaseOK:
			if st.LeaseExpiresAt != nil {
				j.setDeadline(w.localDeadline(*st.LeaseExpiresAt))
			}
		case workerproto.LeaseLost, workerproto.LeaseCancelled, workerproto.LeaseCommitted:
			j.log.Info("server ended the lease", "status", st.Status)
			w.discard(j)
		}
	}
	return nil
}

// checkLeaseGone stops the computations when no heartbeat got through for 2 x lease
// duration: the server has certainly given the jobs to someone else. Computed entries are
// kept until the server answers about them.
func (w *Worker) checkLeaseGone() {
	s := w.sessionSnapshot()
	if s.lease <= 0 || time.Since(s.lastContact) < 2*s.lease {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, j := range w.jobs {
		if j.started.Load() && !j.isDelivering() {
			j.log.Warn("no contact with the server for 2 x lease duration: stopping the computation")
			j.cancel(errLeaseGone)
		}
	}
}

// finish removes a job from the worker and its outbox entry.
func (w *Worker) finish(j *job) {
	if w.killed() {
		return
	}
	if err := w.outbox.Remove(j.entry); err != nil {
		j.log.Error("failed to remove outbox entry", "error", err)
	}
	w.forget(j)
}

// forget removes the job from memory only.
func (w *Worker) forget(j *job) {
	w.mu.Lock()
	if cur := w.jobs[j.id]; cur == j {
		delete(w.jobs, j.id)
	}
	w.mu.Unlock()
	if j.handle != nil {
		j.handle.Release()
	}
	w.signalSlot()
}
