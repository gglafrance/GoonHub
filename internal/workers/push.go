package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"goonhub/internal/data"
	"goonhub/pkg/workerproto"

	"go.uber.org/zap"
)

// PushDispatcher assigns jobs to the push workers (section 8): every push_poll_interval it
// asks each enabled push worker its status and, for its free slots, claims jobs exactly as a
// pull claim (same lease) and POSTs them to the worker. An assignment the worker does not
// accept is released at once, without counting an attempt. A worker that stops answering
// gets no new job; its leases expire on their own.
type PushDispatcher struct {
	svc    *Service
	client *http.Client
	logger *zap.Logger
}

// NewPushDispatcher creates the dispatcher and registers it as the push canceller.
func NewPushDispatcher(svc *Service, client *http.Client) *PushDispatcher {
	if client == nil {
		client = &http.Client{Timeout: svc.cfg.PushTimeout}
	}
	d := &PushDispatcher{svc: svc, client: client, logger: svc.logger.With(zap.String("loop", "push_dispatcher"))}
	svc.SetPushCanceller(d)
	return d
}

// Run loops until ctx is done.
func (d *PushDispatcher) Run(ctx context.Context) {
	ticker := time.NewTicker(d.svc.cfg.PushPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.DispatchOnce(ctx)
		}
	}
}

// DispatchOnce polls every enabled push worker once, in parallel, and returns how many jobs
// were assigned.
func (d *PushDispatcher) DispatchOnce(ctx context.Context) int {
	list, err := d.svc.Workers.List()
	if err != nil {
		d.logger.Error("Failed to list workers", zap.Error(err))
		return 0
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	total := 0
	for i := range list {
		w := &list[i]
		if !w.Enabled || w.ConnectionMode != string(workerproto.ConnPush) || w.PushURL == "" || w.SessionID == "" || w.TokenHash == nil {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			n := d.dispatchWorker(ctx, w)
			mu.Lock()
			total += n
			mu.Unlock()
		}()
	}
	wg.Wait()
	return total
}

func (d *PushDispatcher) do(ctx context.Context, w *data.Worker, method, path string, body any, out any) (int, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		reader = bytes.NewReader(raw)
	}
	ctx, cancel := context.WithTimeout(ctx, d.svc.cfg.PushTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, w.PushURL+path, reader)
	if err != nil {
		return 0, err
	}
	req.Header.Set(workerproto.HeaderAuthorization, "Bearer "+w.PushToken)
	req.Header.Set(workerproto.HeaderAPIVersion, strconv.Itoa(workerproto.APIVersion))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if out != nil && resp.StatusCode/100 == 2 {
		if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out); err != nil {
			return resp.StatusCode, fmt.Errorf("invalid answer: %w", err)
		}
	} else {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	}
	return resp.StatusCode, nil
}

func (d *PushDispatcher) dispatchWorker(ctx context.Context, w *data.Worker) int {
	var status workerproto.PushStatus
	code, err := d.do(ctx, w, http.MethodGet, workerproto.PathPushStatus, nil, &status)
	if err == nil && code != http.StatusOK {
		err = fmt.Errorf("status answered %d", code)
	}
	if err != nil {
		d.logger.Debug("Push worker unreachable", zap.Uint("worker_id", w.ID), zap.Error(err))
		if rerr := d.svc.Workers.RecordError(w.ID, "push status: "+err.Error(), d.svc.now()); rerr != nil {
			d.logger.Warn("Failed to record worker error", zap.Error(rerr))
		}
		return 0
	}
	d.svc.touch(w, nil)
	if !status.Accepting || w.Draining {
		return 0
	}
	assignments, err := d.svc.ClaimFor(w, status.FreeSlots)
	if err != nil {
		d.logger.Error("Failed to claim jobs for a push worker", zap.Uint("worker_id", w.ID), zap.Error(err))
	}
	assigned := 0
	for _, a := range assignments {
		// 202 is the acceptance (its body is informational).
		code, err := d.do(ctx, w, http.MethodPost, workerproto.PathPushAssign, workerproto.PushAssignRequest{Assignment: a}, nil)
		if err == nil && code == http.StatusAccepted {
			assigned++
			continue
		}
		if err == nil {
			err = fmt.Errorf("assign answered %d", code)
		}
		// Not accepted (busy, error, timeout): give the job back at once. If the worker did
		// accept it after all, its first call with this generation is refused.
		if _, rerr := d.svc.Leases.ReleaseLease(a.JobID, a.Lease.Generation, w.ID, false); rerr != nil {
			d.logger.Error("Failed to release a push assignment", zap.String("job_id", a.JobID), zap.Error(rerr))
		}
		d.logger.Debug("Push assignment not accepted, released", zap.Uint("worker_id", w.ID), zap.String("job_id", a.JobID), zap.Error(err))
	}
	if assigned > 0 {
		d.logger.Debug("Assigned jobs to push worker", zap.Uint("worker_id", w.ID), zap.Int("count", assigned))
	}
	return assigned
}

// Cancel tells a push worker to stop a cancelled job (best effort, in the background: the
// next heartbeat answers cancelled anyway).
func (d *PushDispatcher) Cancel(w *data.Worker, jobID string, generation int64) {
	go func() {
		if _, err := d.do(context.Background(), w, http.MethodPost, workerproto.PathPushCancel,
			workerproto.PushCancelRequest{JobID: jobID, Generation: generation}, nil); err != nil {
			d.logger.Debug("Push cancel failed", zap.Uint("worker_id", w.ID), zap.String("job_id", jobID), zap.Error(err))
		}
	}()
}
