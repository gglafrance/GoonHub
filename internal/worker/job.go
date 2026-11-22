package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"goonhub/pkg/compute"
	"goonhub/pkg/workerproto"

	"goonhub-worker/internal/cache"
	"goonhub-worker/internal/client"
	"goonhub-worker/internal/outbox"
)

// job is one leased job held by the worker.
type job struct {
	id    string
	gen   int64
	phase workerproto.Phase
	entry *outbox.Entry
	log   *slog.Logger

	ctx    context.Context
	cancel context.CancelCauseFunc
	done   chan struct{}

	recovered  bool        // reloaded from the outbox at start
	started    atomic.Bool // a goroutine owns it
	delivering atomic.Bool // computed: uploading / completing
	computing  bool        // holds a computing slot (guarded by Worker.mu)

	mu        sync.Mutex
	stage     workerproto.JobStage
	inputMode workerproto.InputMode
	deadline  time.Time // local lease expiry
	mediaURL  string
	urlExpiry time.Time
	progress  atomic.Int32

	handle *cache.Handle // cached source in use
}

func (w *Worker) newJob(e *outbox.Entry) *job {
	a := e.Assignment
	ctx, cancel := context.WithCancelCause(w.killCtx)
	j := &job{
		id:        a.JobID,
		gen:       e.State.Generation,
		phase:     a.Phase,
		entry:     e,
		log:       w.log.With("job_id", a.JobID, "phase", a.Phase, "scene_id", a.SceneID, "generation", e.State.Generation),
		ctx:       ctx,
		cancel:    cancel,
		done:      make(chan struct{}),
		stage:     workerproto.StageComputing,
		inputMode: e.State.InputMode,
		mediaURL:  a.Input.URL,
		urlExpiry: a.Input.URLExpiresAt,
	}
	if !a.Lease.ExpiresAt.IsZero() {
		j.deadline = w.localDeadline(a.Lease.ExpiresAt)
	}
	switch e.State.Stage {
	case outbox.StageUploading, outbox.StageComputed:
		j.stage = workerproto.StageUploading
	case outbox.StageCompleting:
		j.stage = workerproto.StageCompleting
	}
	return j
}

func (j *job) isDelivering() bool { return j.delivering.Load() }

func (j *job) setStage(s workerproto.JobStage) {
	j.mu.Lock()
	j.stage = s
	j.mu.Unlock()
}

func (j *job) setDeadline(t time.Time) {
	j.mu.Lock()
	j.deadline = t
	j.mu.Unlock()
}

func (j *job) setProgress(p int) {
	for {
		cur := j.progress.Load()
		if int32(p) <= cur || j.progress.CompareAndSwap(cur, int32(p)) {
			return
		}
	}
}

func (j *job) heartbeat() workerproto.JobHeartbeat {
	j.mu.Lock()
	defer j.mu.Unlock()
	return workerproto.JobHeartbeat{JobID: j.id, Generation: j.gen, Progress: int(j.progress.Load()), Stage: j.stage, InputMode: j.inputMode}
}

func (j *job) url() (string, time.Time) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.mediaURL, j.urlExpiry
}

// launch runs fn as the job's goroutine.
func (w *Worker) launch(j *job, fn func()) {
	j.started.Store(true)
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		defer close(j.done)
		fn()
	}()
}

// ---------------------------------------------------------------------------------------
// Intake
// ---------------------------------------------------------------------------------------

// errBusy is returned by accept when there is no free slot.
var errBusy = errors.New("no free slot")

// errStale is returned by accept for an older generation of a job already held.
var errStale = errors.New("stale generation")

// errInvalidAssignment is returned by accept for an assignment the worker cannot run.
type errInvalidAssignment struct{ err error }

func (e *errInvalidAssignment) Error() string { return e.err.Error() }

// validateAssignment checks an assignment before taking it.
func (w *Worker) validateAssignment(a *workerproto.Assignment) error {
	if a.JobID == "" {
		return errors.New("assignment without job_id")
	}
	if !a.Phase.Valid() {
		return fmt.Errorf("unknown phase %q", a.Phase)
	}
	if a.Lease.Generation <= 0 {
		return errors.New("assignment without lease generation")
	}
	if err := a.Params.Validate(a.Phase); err != nil {
		return err
	}
	var err error
	switch a.Phase {
	case workerproto.PhaseSprites:
		err = a.Params.Sprites.Validate()
		if err == nil && a.Params.Sprites.SceneID != a.SceneID {
			err = errors.New("sprites params scene_id differs from the assignment")
		}
	case workerproto.PhaseAnimatedThumbnails:
		err = a.Params.AnimatedThumbnails.Validate()
	case workerproto.PhaseFingerprint:
		err = a.Params.Fingerprint.Validate()
	}
	if err != nil {
		return err
	}
	if a.Input.URL == "" && a.Input.ServerPath == "" {
		return errors.New("assignment without input")
	}
	return nil
}

// accept takes an assignment: checks a slot, writes the outbox entry (fsync) and starts
// the job. force skips the slot check (pull claims already counted).
func (w *Worker) accept(a workerproto.Assignment) error {
	if err := w.validateAssignment(&a); err != nil {
		return &errInvalidAssignment{err}
	}
	w.mu.Lock()
	if old := w.jobs[a.JobID]; old != nil {
		if old.gen == a.Lease.Generation {
			w.mu.Unlock()
			return nil // duplicate delivery of the same lease
		}
		if old.gen > a.Lease.Generation {
			w.mu.Unlock()
			return errStale
		}
		// A newer lease of a job we still hold: the old one is dead.
		old.log.Info("new lease of a held job: dropping the old generation")
		w.mu.Unlock()
		if old.started.Load() {
			old.cancel(errDiscard)
			<-old.done
		} else {
			w.finish(old)
		}
		w.mu.Lock()
	}
	if w.cfg.Phases[a.Phase] <= 0 {
		w.mu.Unlock()
		w.log.Warn("assignment of a phase not enabled on this worker", "job_id", a.JobID, "phase", a.Phase)
		return errBusy
	}
	if !w.acceptingLocked() || w.computing[a.Phase] >= w.cfg.Phases[a.Phase] || w.computingTotal >= w.cfg.MaxConcurrency {
		w.mu.Unlock()
		return errBusy
	}
	w.computing[a.Phase]++
	w.computingTotal++
	w.mu.Unlock()

	e, err := w.outbox.Create(a)
	if err != nil {
		w.mu.Lock()
		w.computing[a.Phase]--
		w.computingTotal--
		w.mu.Unlock()
		return fmt.Errorf("outbox: %w", err)
	}
	j := w.newJob(e)
	j.computing = true
	w.mu.Lock()
	w.jobs[j.id] = j
	w.mu.Unlock()
	j.log.Info("job accepted", "attempt", a.Attempt, "size", a.Input.Size, "duration_sec", a.Input.Video.DurationSec)
	w.launch(j, func() { w.runJob(j) })
	return nil
}

// releaseSlot gives back the computing slot of j.
func (w *Worker) releaseSlot(j *job) {
	w.mu.Lock()
	if j.computing {
		j.computing = false
		w.computing[j.phase]--
		w.computingTotal--
	}
	w.mu.Unlock()
	w.signalSlot()
}

// ---------------------------------------------------------------------------------------
// Execution
// ---------------------------------------------------------------------------------------

// runJob computes, then delivers.
func (w *Worker) runJob(j *job) {
	start := time.Now()
	res, stats, err := w.computeJob(j)
	w.releaseSlot(j)
	if err != nil {
		w.handleComputeError(j, err)
		return
	}
	stats.ComputeMillis = time.Since(start).Milliseconds() - stats.DownloadMillis
	if err := w.prepareComplete(j, res, stats); err != nil {
		if w.stopped(j) {
			return
		}
		j.log.Error("computed output is unusable", "error", err)
		w.failJob(j, workerproto.FailureCompute, err, true)
		return
	}
	w.mu.Lock()
	w.delivering++
	j.delivering.Store(true)
	w.mu.Unlock()
	w.deliverAndFinish(j)
}

// stopped handles a cancelled job context: it returns true when the job context ended
// (and the job has been released / dropped accordingly).
func (w *Worker) stopped(j *job) bool {
	if j.ctx.Err() == nil {
		return false
	}
	cause := context.Cause(j.ctx)
	switch {
	case w.killed() || errors.Is(cause, errKilled):
		// Hard stop: leave the outbox as is.
	case errors.Is(cause, errShutdown):
		w.release(j, workerproto.ReleaseShutdown)
	case errors.Is(cause, errDiscard):
		j.log.Info("job dropped (lease ended on the server)")
		w.finish(j)
	case errors.Is(cause, errLeaseGone):
		if j.isDelivering() {
			// Keep the computed result until the server answers about it.
			w.forget(j)
			return true
		}
		j.log.Warn("job abandoned: lease certainly expired")
		w.finish(j)
	default:
		w.finish(j)
	}
	return true
}

// SignalSettle is how long a failed computation waits for a concurrent stop signal
// before being reported as a failure.
var SignalSettle = time.Second

// phaseTimeout is the worker-side job timeout (server semantics).
func (w *Worker) phaseTimeout(a *workerproto.Assignment) time.Duration {
	switch a.Phase {
	case workerproto.PhaseSprites:
		return w.cfg.Timeouts.Sprites.D()
	case workerproto.PhaseAnimatedThumbnails:
		return w.cfg.Timeouts.AnimatedThumbnails.D()
	case workerproto.PhaseFingerprint:
		t := w.cfg.Timeouts.Fingerprint.D()
		if byLen := time.Duration(a.Input.Video.DurationSec) * time.Second / 2; byLen > t {
			t = byLen
		}
		return t
	}
	return time.Hour
}

// computeError carries the classification of a failed computation.
type computeError struct {
	kind      workerproto.FailureKind
	retryable bool
	release   workerproto.ReleaseReason // set: release instead of fail
	err       error
}

func (e *computeError) Error() string { return e.err.Error() }
func (e *computeError) Unwrap() error { return e.err }

func (w *Worker) computeJob(j *job) (workerproto.PhaseResult, workerproto.JobStats, error) {
	a := j.entry.Assignment
	stats := workerproto.JobStats{Accel: w.env.Accel.Label()}

	if a.Phase == workerproto.PhaseFingerprint && a.Input.Video.HasAudio() && !w.tools.HasAudioFingerprint() {
		return workerproto.PhaseResult{}, stats, &computeError{release: workerproto.ReleaseOverloaded,
			err: errors.New("fingerprint of a scene with audio needs fpcalc, which this worker does not have")}
	}

	params := a.Params
	if a.Phase == workerproto.PhaseSprites && w.cfg.SpritesConcurrency > 0 {
		p := *params.Sprites
		p.Concurrency = w.cfg.SpritesConcurrency
		params.Sprites = &p
	}
	env := w.env
	env.TempDir = j.entry.WorkDir()

	const maxNetworkRetries = 3
	netRetries := 0
	triedDirectFallback := false
	forceIndirect := false
	for {
		in, err := w.resolveInput(j, forceIndirect)
		if err != nil {
			return workerproto.PhaseResult{}, stats, err
		}
		stats.InputMode = in.mode
		stats.DownloadMillis += in.downloadMillis
		stats.BytesDownloaded += in.downloaded
		j.mu.Lock()
		j.inputMode = in.mode
		j.mu.Unlock()
		if j.entry.State.InputMode != in.mode {
			j.entry.State.InputMode = in.mode
			_ = j.entry.Save()
		}

		// Start from empty output and scratch directories (a retried attempt must not mix
		// with the files of the previous one).
		if err := resetDir(j.entry.OutDir()); err != nil {
			return workerproto.PhaseResult{}, stats, &computeError{release: workerproto.ReleaseDiskFull, err: err}
		}
		if err := resetDir(env.TempDir); err != nil {
			return workerproto.PhaseResult{}, stats, &computeError{release: workerproto.ReleaseDiskFull, err: err}
		}
		j.setStage(workerproto.StageComputing)
		j.progress.Store(0)
		cctx, cancel := context.WithTimeout(j.ctx, w.phaseTimeout(&a))
		j.log.Info("computing", "input_mode", in.mode, "input", in.describe(), "accel", w.env.Accel.Label())
		res, err := w.compute(cctx, env, compute.Request{
			Input:    in.path,
			Video:    a.Input.Video,
			OutDir:   j.entry.OutDir(),
			Progress: j.setProgress,
		}, a.Phase, params)
		timedOut := errors.Is(cctx.Err(), context.DeadlineExceeded)
		cancel()
		if err == nil {
			return res, stats, nil
		}
		if j.ctx.Err() != nil {
			return res, stats, context.Cause(j.ctx)
		}
		// A stop signal from a terminal (Ctrl+C) or from systemd reaches the ffmpeg
		// children too: they may die just before the worker's own stop cancels the job.
		// Give the stop a moment so such a failure is a release, not a failed attempt.
		select {
		case <-j.ctx.Done():
			return res, stats, context.Cause(j.ctx)
		case <-time.After(SignalSettle):
		}
		if timedOut {
			return res, stats, &computeError{kind: workerproto.FailureTimeout, retryable: true,
				err: fmt.Errorf("worker-side timeout after %s: %w", w.phaseTimeout(&a), err)}
		}
		if errors.Is(err, exec.ErrNotFound) {
			j.log.Error("a binary is missing (ffmpeg/ffprobe/fpcalc): releasing the job", "error", err)
			return res, stats, &computeError{release: workerproto.ReleaseOverloaded, err: err}
		}
		if w.updateDisk(); w.isDiskLow() {
			return res, stats, &computeError{release: workerproto.ReleaseDiskFull, err: fmt.Errorf("worker disk full: %w", err)}
		}

		switch in.mode {
		case workerproto.InputDirect:
			if !triedDirectFallback && a.Input.URL != "" {
				if _, ok, why := w.directPath(&a); !ok {
					// The mount went away during the computation: read over HTTP instead.
					j.log.Warn("direct input became unreadable, retrying in indirect mode", "reason", why)
					triedDirectFallback = true
					forceIndirect = true
					continue
				}
			}
		case workerproto.InputIndirect:
			if in.viaURL {
				retry, perr := w.probeMedia(j)
				if perr != nil {
					return res, stats, perr
				}
				if retry && netRetries < maxNetworkRetries {
					netRetries++
					j.log.Warn("computation failed while the media URL was unreachable or expired: retrying", "error", err, "retry", netRetries)
					continue
				}
			}
		}
		return res, stats, &computeError{kind: workerproto.FailureCompute, retryable: true, err: err}
	}
}

// resetDir empties a directory of the job's outbox entry (never a source directory).
func resetDir(dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("failed to clear %s: %w", dir, err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create %s: %w", dir, err)
	}
	return nil
}

func (w *Worker) isDiskLow() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.diskLow
}

// probeMedia checks the media URL after a failed computation. retry=true when the
// failure may come from the transport (expired URL refreshed, server unreachable and back);
// an error when the lease is over.
func (w *Worker) probeMedia(j *job) (bool, error) {
	bo := w.backoff
	for attempt := 0; ; attempt++ {
		u, _ := j.url()
		_, err := w.client.MediaHead(j.ctx, u)
		switch {
		case err == nil:
			return attempt > 0, nil
		case j.ctx.Err() != nil:
			return false, context.Cause(j.ctx)
		case client.IsCode(err, workerproto.CodeURLExpired):
			if _, rerr := w.refreshURL(j); rerr != nil {
				return false, rerr
			}
			return true, nil
		case client.IsLeaseGone(err):
			j.log.Info("media URL says the lease is over", "error", err)
			j.cancel(errDiscard)
			return false, errDiscard
		case client.IsTransient(err):
			if attempt == 0 {
				j.log.Warn("media URL unreachable after a failed computation: waiting for the server", "error", err)
			}
			if werr := bo.Wait(j.ctx); werr != nil {
				return false, context.Cause(j.ctx)
			}
		default:
			return false, &computeError{kind: workerproto.FailureInput, retryable: true, err: fmt.Errorf("media URL refused: %w", err)}
		}
	}
}

// refreshURL asks a new signed media URL.
func (w *Worker) refreshURL(j *job) (string, error) {
	bo := w.backoff
	for {
		resp, err := w.client.MediaURL(j.ctx, j.id, j.gen)
		if err == nil {
			j.mu.Lock()
			j.mediaURL, j.urlExpiry = resp.URL, resp.ExpiresAt
			j.mu.Unlock()
			j.log.Info("media URL refreshed", "expires_at", resp.ExpiresAt)
			return resp.URL, nil
		}
		if j.ctx.Err() != nil {
			return "", context.Cause(j.ctx)
		}
		if client.IsLeaseGone(err) {
			j.cancel(errDiscard)
			return "", errDiscard
		}
		if !client.IsTransient(err) {
			return "", &computeError{kind: workerproto.FailureInput, retryable: true, err: fmt.Errorf("media URL refresh refused: %w", err)}
		}
		if werr := bo.Wait(j.ctx); werr != nil {
			return "", context.Cause(j.ctx)
		}
	}
}

// handleComputeError turns a failed computation into a release or a fail.
func (w *Worker) handleComputeError(j *job, err error) {
	if w.stopped(j) {
		return
	}
	var ce *computeError
	if !errors.As(err, &ce) {
		ce = &computeError{kind: workerproto.FailureCompute, retryable: true, err: err}
	}
	if ce.release != "" {
		j.log.Warn("releasing the job", "reason", ce.release, "error", ce.err)
		w.release(j, ce.release)
		return
	}
	j.log.Error("job failed", "kind", ce.kind, "error", client.RedactText(ce.err.Error()))
	w.failJob(j, ce.kind, ce.err, ce.retryable)
}

// prepareComplete hashes the artifacts and persists the CompleteRequest (stage computed).
func (w *Worker) prepareComplete(j *job, res workerproto.PhaseResult, stats workerproto.JobStats) error {
	a := j.entry.Assignment
	if err := res.Validate(a.Phase); err != nil {
		return err
	}
	switch a.Phase {
	case workerproto.PhaseSprites:
		if err := res.Sprites.Validate(a.SceneID); err != nil {
			return err
		}
	case workerproto.PhaseAnimatedThumbnails:
		if err := res.AnimatedThumbnails.Validate(a.Params.AnimatedThumbnails); err != nil {
			return err
		}
	case workerproto.PhaseFingerprint:
		if err := res.Fingerprint.Validate(); err != nil {
			return err
		}
	}
	maxBytes := w.sessionSnapshot().maxArtifactBytes
	refs := make([]workerproto.ArtifactRef, 0, len(res.Files()))
	for _, name := range res.Files() {
		ref, err := workerproto.ArtifactRefForFile(name, j.entry.ArtifactPath(name))
		if err != nil {
			return fmt.Errorf("artifact %s: %w", name, err)
		}
		if maxBytes > 0 && ref.Size > maxBytes {
			return fmt.Errorf("artifact %s is %d bytes, above the server limit %d", name, ref.Size, maxBytes)
		}
		refs = append(refs, ref)
	}
	if err := workerproto.ValidateArtifacts(refs, res); err != nil {
		return err
	}
	j.entry.State.Stage = outbox.StageComputed
	j.entry.State.Complete = &workerproto.CompleteRequest{
		Generation: j.gen,
		Phase:      a.Phase,
		Result:     res,
		Artifacts:  refs,
		Stats:      stats,
	}
	if err := j.entry.Save(); err != nil {
		return fmt.Errorf("failed to persist the result: %w", err)
	}
	j.setStage(workerproto.StageUploading)
	j.log.Info("computed", "artifacts", len(refs), "compute_ms", stats.ComputeMillis, "input_mode", stats.InputMode)
	return nil
}
