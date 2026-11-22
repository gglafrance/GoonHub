package worker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"goonhub/pkg/workerproto"

	"goonhub-worker/internal/client"
	"goonhub-worker/internal/outbox"
)

// errPermanent marks an upload/complete answer that retrying cannot fix.
type errPermanent struct{ err error }

func (e *errPermanent) Error() string { return e.err.Error() }
func (e *errPermanent) Unwrap() error { return e.err }

// deliverAndFinish uploads every artifact, then completes. It retries network errors with
// backoff as long as the job is alive; the entry is removed once the server acknowledged
// it (committed / already committed) or said the lease is over.
func (w *Worker) deliverAndFinish(j *job) {
	defer func() {
		w.mu.Lock()
		if j.delivering.Load() {
			w.delivering--
		}
		w.mu.Unlock()
		w.signalSlot()
	}()
	start := time.Now()
	const maxMissingRounds = 5
	for round := 0; ; round++ {
		err := w.uploadAll(j)
		if err == nil {
			j.entry.State.Complete.Stats.UploadMillis = time.Since(start).Milliseconds()
			var missing []string
			missing, err = w.complete(j)
			if err == nil {
				return
			}
			if len(missing) > 0 && round < maxMissingRounds {
				j.log.Warn("server misses artifacts at complete: uploading them again", "artifacts", missing)
				continue
			}
		}
		if w.stopped(j) {
			return
		}
		var pe *errPermanent
		if errors.As(err, &pe) {
			j.log.Error("server refused the job result", "error", err)
			w.failJob(j, workerproto.FailureInternal, err, true)
			return
		}
		// Unknown: treat as a lease end (should not happen: retries are inside).
		j.log.Error("delivery stopped", "error", err)
		w.finish(j)
		return
	}
}

// uploadAll uploads every artifact not yet complete on the server.
func (w *Worker) uploadAll(j *job) error {
	st := &j.entry.State
	if st.Stage == outbox.StageComputed {
		st.Stage = outbox.StageUploading
		_ = j.entry.Save()
	}
	j.setStage(workerproto.StageUploading)
	refs := st.Complete.Artifacts
	for i := range refs {
		if err := w.uploadArtifact(j, &refs[i]); err != nil {
			return err
		}
		j.setProgress((i + 1) * 100 / len(refs))
	}
	return nil
}

// uploadArtifact makes the server hold the complete artifact: GET status, then PUT chunks
// from the received offset; resumes after any error.
func (w *Worker) uploadArtifact(j *job, ref *workerproto.ArtifactRef) error {
	bo := w.backoff
	localPath := j.entry.ArtifactPath(ref.Name)
	offset := int64(-1) // unknown: ask the server
	for {
		if err := j.ctx.Err(); err != nil {
			return context.Cause(j.ctx)
		}
		if offset < 0 {
			us, err := w.client.UploadStatus(j.ctx, j.id, ref.Name, j.gen)
			if err != nil {
				if rerr := w.retryable(j, err, &bo, "upload status"); rerr != nil {
					return rerr
				}
				continue
			}
			switch {
			case us.Complete && us.Size == ref.Size && us.SHA256 == ref.SHA256:
				return nil
			case us.Received > 0 && us.Size == ref.Size && us.SHA256 == ref.SHA256 && us.Received <= ref.Size:
				offset = us.Received
			default:
				offset = 0 // nothing, or another declaration: start again (offset 0 truncates)
			}
		}
		status, err := w.putFrom(j, ref, localPath, offset)
		if err == nil {
			if status.Complete {
				j.entry.State.Uploaded = setUploaded(j.entry.State.Uploaded, ref.Name, ref.Size)
				_ = j.entry.Save()
				return nil
			}
			offset = status.Received
			bo.Reset()
			continue
		}
		switch {
		case client.IsCode(err, workerproto.CodeOffsetMismatch):
			offset = -1
			if ae, ok := client.AsAPIError(err); ok {
				if r, perr := strconv.ParseInt(ae.Details["received"], 10, 64); perr == nil && r >= 0 && r <= ref.Size {
					offset = r
				}
			}
			continue
		case client.IsCode(err, workerproto.CodeSHA256Mismatch):
			j.log.Warn("server got a corrupted artifact: re-hashing and uploading again from 0", "artifact", ref.Name)
			fresh, herr := workerproto.ArtifactRefForFile(ref.Name, localPath)
			if herr != nil {
				return &errPermanent{fmt.Errorf("artifact %s unreadable: %w", ref.Name, herr)}
			}
			if fresh != *ref {
				*ref = fresh
				_ = j.entry.Save()
			}
			offset = 0
			continue
		}
		var lerr *localReadError
		if errors.As(err, &lerr) {
			return &errPermanent{err}
		}
		if rerr := w.retryable(j, err, &bo, "upload "+ref.Name); rerr != nil {
			return rerr
		}
		offset = -1
	}
}

func setUploaded(m map[string]int64, name string, n int64) map[string]int64 {
	if m == nil {
		m = map[string]int64{}
	}
	m[name] = n
	return m
}

// localReadError is a failure to read the local artifact.
type localReadError struct{ err error }

func (e *localReadError) Error() string { return e.err.Error() }
func (e *localReadError) Unwrap() error { return e.err }

// putFrom sends the artifact from offset, one chunk per PUT, until the server holds it
// all or a call fails.
func (w *Worker) putFrom(j *job, ref *workerproto.ArtifactRef, localPath string, offset int64) (*workerproto.UploadStatus, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return nil, &localReadError{fmt.Errorf("failed to open artifact %s: %w", ref.Name, err)}
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, &localReadError{err}
	}
	if st.Size() != ref.Size {
		return nil, &localReadError{fmt.Errorf("artifact %s changed on disk (%d bytes, declared %d)", ref.Name, st.Size(), ref.Size)}
	}
	chunk := w.sessionSnapshot().chunkBytes
	for {
		n := min(chunk, ref.Size-offset)
		if n < 0 {
			n = 0
		}
		body := w.upLimit.Reader(j.ctx, io.NewSectionReader(f, offset, n))
		// Generous bound: 60 s + 1 s per 256 KiB.
		timeout := 60*time.Second + time.Duration(n/(256<<10))*time.Second
		status, err := w.client.PutArtifact(j.ctx, j.id, j.gen, *ref, offset, body, n, timeout)
		if err != nil {
			return nil, err
		}
		if status.Complete || status.Received >= ref.Size {
			return status, nil
		}
		if status.Received <= offset {
			// No progress: let the caller re-sync with the server.
			return status, nil
		}
		offset = status.Received
	}
}

// retryable decides what to do with a failed delivery call: nil = retry after a backoff
// wait; otherwise the error to return (lease over, permanent, job cancelled).
func (w *Worker) retryable(j *job, err error, bo interface{ Wait(context.Context) error }, what string) error {
	if j.ctx.Err() != nil {
		return context.Cause(j.ctx)
	}
	if client.IsLeaseGone(err) {
		j.log.Info("server says the lease is over", "call", what, "error", err)
		j.cancel(errDiscard)
		return context.Cause(j.ctx)
	}
	// Transient, or a configuration problem the admin can fix (token, disabled worker,
	// address, worker API off): keep the result and retry.
	if client.IsCode(err, workerproto.CodeUnknownSession, workerproto.CodeWorkersDisabled, workerproto.CodeAddressNotAllowed) ||
		client.IsTransient(err) || client.StatusOf(err) == http.StatusUnauthorized || client.StatusOf(err) == http.StatusForbidden {
		j.log.Warn("delivery call failed, retrying", "call", what, "error", err)
		if werr := bo.Wait(j.ctx); werr != nil {
			return context.Cause(j.ctx)
		}
		return nil
	}
	return &errPermanent{err}
}

// complete sends the CompleteRequest. It returns the artifact names the server misses
// (422 artifact_missing) so the caller uploads them again.
func (w *Worker) complete(j *job) ([]string, error) {
	st := &j.entry.State
	st.Stage = outbox.StageCompleting
	_ = j.entry.Save()
	j.setStage(workerproto.StageCompleting)
	bo := w.backoff
	for {
		resp, err := w.client.Complete(j.ctx, j.id, st.Complete)
		if err == nil {
			j.log.Info("job completed", "status", resp.Status)
			w.finish(j)
			return nil, nil
		}
		if client.IsCode(err, workerproto.CodeArtifactMissing) {
			ae, _ := client.AsAPIError(err)
			var missing []string
			for name := range ae.Details {
				missing = append(missing, name)
			}
			if len(missing) == 0 {
				for _, ref := range st.Complete.Artifacts {
					missing = append(missing, ref.Name)
				}
			}
			return missing, err
		}
		if rerr := w.retryable(j, err, &bo, "complete"); rerr != nil {
			return nil, rerr
		}
	}
}

// release gives the job back to the server (graceful: pending at once) and removes the
// entry. When the server cannot be reached the entry is kept with release_pending so the
// next start releases it.
func (w *Worker) release(j *job, reason workerproto.ReleaseReason) {
	if w.killed() {
		return
	}
	ctx, cancel := context.WithTimeout(w.killCtx, 8*time.Second)
	defer cancel()
	bo := w.backoff
	bo.Max = min(bo.Max, 2*time.Second)
	if bo.Max <= 0 {
		bo.Max = 2 * time.Second
	}
	var err error
	for {
		err = w.client.Release(ctx, j.id, &workerproto.ReleaseRequest{Generation: j.gen, Reason: reason})
		if err == nil || client.IsLeaseGone(err) || !client.IsTransient(err) {
			break
		}
		if werr := bo.Wait(ctx); werr != nil {
			break
		}
	}
	if err != nil && client.IsTransient(err) {
		if w.killed() {
			return
		}
		j.log.Warn("could not release the job (server unreachable): it will be released at the next start, or its lease expires", "reason", reason, "error", err)
		j.entry.State.ReleasePending = reason
		if serr := j.entry.Save(); serr != nil {
			j.log.Error("failed to save the outbox entry", "error", serr)
		}
		w.forget(j)
		return
	}
	if err != nil && !client.IsLeaseGone(err) {
		j.log.Warn("server refused the release", "reason", reason, "error", err)
	} else {
		j.log.Info("job released", "reason", reason)
	}
	w.finish(j)
}

// failJob reports a failure (the server applies its retry policy) and removes the entry.
func (w *Worker) failJob(j *job, kind workerproto.FailureKind, cause error, retryable bool) {
	if w.killed() {
		return
	}
	ctx, cancel := context.WithTimeout(w.killCtx, 30*time.Second)
	defer cancel()
	msg := client.RedactText(cause.Error())
	if len(msg) > 8000 {
		msg = msg[:8000] + "..."
	}
	bo := w.backoff
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		err = w.client.Fail(ctx, j.id, &workerproto.FailRequest{Generation: j.gen, Kind: kind, Error: msg, Retryable: retryable})
		if err == nil || client.IsLeaseGone(err) || !client.IsTransient(err) {
			break
		}
		if werr := bo.Wait(ctx); werr != nil {
			break
		}
	}
	if err != nil && !client.IsLeaseGone(err) {
		j.log.Warn("could not report the failure (the lease will expire on the server)", "error", err)
	}
	w.finish(j)
}
