package workers

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"goonhub/internal/data"
	"goonhub/internal/jobs"
	"goonhub/pkg/workerproto"

	"go.uber.org/zap"
)

// expireBatch bounds the rows one expiry statement handles.
const expireBatch = 200

// Start runs the background loops of the worker API: the startup lease grace once, then the
// lease expiry sweep, the staging janitor and (with a dispatcher) the push dispatcher. Stop
// ends them. Nothing runs when the worker API is disabled, except the startup grace and the
// expiry sweep when remote leases are still in the database (they must never be lost).
func (s *Service) Start(dispatcher *PushDispatcher) {
	ctx, cancel := context.WithCancel(context.Background())
	s.stopMu.Lock()
	s.stop = cancel
	s.stopMu.Unlock()

	if n, err := s.Leases.ExtendRemoteLeases(s.now().Add(s.cfg.StartupLeaseGrace)); err != nil {
		s.logger.Error("Failed to extend the remote leases at start", zap.Error(err))
	} else if n > 0 {
		s.logger.Info("Extended the remote leases kept across the restart",
			zap.Int64("leases", n), zap.Duration("grace", s.cfg.StartupLeaseGrace))
	}

	s.loops.Add(1)
	go func() {
		defer s.loops.Done()
		ticker := time.NewTicker(s.cfg.ExpirySweepInterval)
		defer ticker.Stop()
		janitor := time.NewTicker(s.cfg.JanitorInterval)
		defer janitor.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.SweepExpired()
			case <-janitor.C:
				s.CleanStaging()
			}
		}
	}()
	if dispatcher != nil && s.cfg.Enabled {
		s.loops.Add(1)
		go func() {
			defer s.loops.Done()
			dispatcher.Run(ctx)
		}()
	}
}

// Stop ends the background loops (leases stay in the database).
func (s *Service) Stop() {
	s.stopMu.Lock()
	stop := s.stop
	s.stopMu.Unlock()
	if stop != nil {
		stop()
	}
	s.loops.Wait()
}

type stopState struct {
	stopMu sync.Mutex
	stop   context.CancelFunc
	loops  sync.WaitGroup
}

// SweepExpired ends the expired remote leases: back to pending with an attempt counted, or
// failed through the retry path when the attempts are exhausted. Returns how many leases
// were requeued and failed.
func (s *Service) SweepExpired() (requeued, failed int) {
	maxRetries := map[string]int{}
	for _, p := range workerproto.RemotePhases {
		maxRetries[string(p)] = 3
		if s.Retry != nil {
			maxRetries[string(p)] = s.Retry.MaxRetries(string(p))
		}
	}
	names := map[uint]string{}
	workerName := func(id *uint) string {
		if id == nil {
			return "?"
		}
		if n, ok := names[*id]; ok {
			return n
		}
		n := fmt.Sprint(*id)
		if w, err := s.Workers.GetByID(*id); err == nil {
			n = w.Name
		}
		names[*id] = n
		return n
	}
	for {
		req, fail, err := s.Leases.ExpireLeases(s.now(), maxRetries, expireBatch)
		if err != nil {
			s.logger.Error("Failed to expire remote leases", zap.Error(err))
			return requeued, failed
		}
		for _, j := range req {
			if j.WorkerID != nil {
				s.count(*j.WorkerID, data.WorkerCounterExpired, 1)
			}
			s.logger.Warn("Remote lease expired, job back to pending",
				zap.String("job_id", j.JobID), zap.String("phase", j.Phase), zap.Uint("scene_id", j.SceneID),
				zap.String("worker", workerName(j.WorkerID)), zap.Int("attempt", j.RetryCount+1))
		}
		for _, j := range fail {
			if j.WorkerID != nil {
				s.count(*j.WorkerID, data.WorkerCounterExpired, 1)
			}
			msg := "lease expired on worker " + workerName(j.WorkerID)
			s.logger.Warn("Remote lease expired, attempts exhausted",
				zap.String("job_id", j.JobID), zap.String("phase", j.Phase), zap.Uint("scene_id", j.SceneID))
			s.Results.HandleResult(jobs.JobResult{JobID: j.JobID, SceneID: j.SceneID, Phase: j.Phase, Status: jobs.JobStatusFailed, Error: errors.New(msg)})
		}
		requeued += len(req)
		failed += len(fail)
		if len(req)+len(fail) < expireBatch {
			return requeued, failed
		}
	}
}

// CleanStaging deletes the staged generations that no current lease can commit any more,
// once untouched for workers.staging_retention. Returns how many were deleted.
func (s *Service) CleanStaging() int {
	if s.Staging == nil {
		return 0
	}
	staged, err := s.Staging.List()
	if err != nil {
		s.logger.Error("Failed to list the worker staging", zap.Error(err))
		return 0
	}
	if len(staged) == 0 {
		return 0
	}
	ids := make([]string, 0, len(staged))
	for _, g := range staged {
		ids = append(ids, g.JobID)
	}
	jobsByID, err := s.Leases.GetJobs(ids)
	if err != nil {
		s.logger.Error("Failed to load the jobs of the worker staging", zap.Error(err))
		return 0
	}
	cutoff := s.now().Add(-s.cfg.StagingRetention)
	removed := 0
	for _, g := range staged {
		j, ok := jobsByID[g.JobID]
		live := ok && j.Status == data.JobStatusRunning && j.WorkerID != nil && j.LeaseGeneration == g.Generation
		if live || g.ModTime.After(cutoff) {
			continue
		}
		if err := s.Staging.RemoveGeneration(g); err != nil {
			s.logger.Warn("Failed to delete an abandoned staging directory", zap.String("dir", g.Dir), zap.Error(err))
			continue
		}
		removed++
	}
	if removed > 0 {
		s.logger.Info("Deleted abandoned worker uploads", zap.Int("generations", removed))
	}
	return removed
}
