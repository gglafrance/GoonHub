package jobs

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

type WorkerPool struct {
	workerCount int
	jobQueue    chan Job
	resultChan  chan JobResult
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	running     atomic.Bool
	logger      *zap.Logger
	registry    *JobRegistry
	timeout     time.Duration
}

func NewWorkerPool(workerCount int, queueSize int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		workerCount: workerCount,
		jobQueue:    make(chan Job, queueSize),
		resultChan:  make(chan JobResult, queueSize),
		ctx:         ctx,
		cancel:      cancel,
		logger:      zap.NewNop(),
		registry:    NewJobRegistry(),
		timeout:     0, // no timeout by default
	}
}

func (p *WorkerPool) SetLogger(logger *zap.Logger) {
	p.logger = logger.With(zap.String("component", "worker_pool"))
}

func (p *WorkerPool) Start() {
	if !p.running.CompareAndSwap(false, true) {
		return
	}

	p.logger.Info("Starting worker pool",
		zap.Int("worker_count", p.workerCount),
		zap.Int("queue_size", cap(p.jobQueue)),
	)

	for i := 0; i < p.workerCount; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}

	p.logger.Info("All workers started and ready")
}

func (p *WorkerPool) worker(id int) {
	defer p.wg.Done()
	p.logger.Debug("Worker started", zap.Int("worker_id", id))

	for {
		select {
		case <-p.ctx.Done():
			p.logger.Debug("Worker shutting down", zap.Int("worker_id", id))
			return
		case job := <-p.jobQueue:
			if job == nil {
				return
			}

			p.logger.Info("Worker accepted job",
				zap.Int("worker_id", id),
				zap.String("job_id", job.GetID()),
				zap.String("job_status", string(job.GetStatus())),
				zap.Int("queue_depth", p.QueueSize()),
			)

			result := JobResult{
				JobID:   job.GetID(),
				VideoID: job.GetVideoID(),
				Phase:   job.GetPhase(),
			}

			// Create execution context with optional timeout
			var execCtx context.Context
			var execCancel context.CancelFunc
			if p.timeout > 0 {
				execCtx, execCancel = context.WithTimeout(p.ctx, p.timeout)
			} else {
				execCtx, execCancel = context.WithCancel(p.ctx)
			}

			err := job.ExecuteWithContext(execCtx)
			execCancel()

			// Unregister the job from the registry after execution
			p.registry.Unregister(job.GetID())

			if err != nil {
				// Check for timeout vs cancellation vs other failures
				jobStatus := job.GetStatus()
				if jobStatus == JobStatusTimedOut {
					result.Status = JobStatusTimedOut
					result.Error = err
					p.logger.Warn("Worker job timed out",
						zap.Int("worker_id", id),
						zap.String("job_id", job.GetID()),
						zap.String("phase", job.GetPhase()),
						zap.Uint("video_id", job.GetVideoID()),
						zap.Duration("timeout", p.timeout),
					)
				} else if jobStatus == JobStatusCancelled {
					result.Status = JobStatusCancelled
					result.Error = err
					p.logger.Warn("Worker job cancelled",
						zap.Int("worker_id", id),
						zap.String("job_id", job.GetID()),
						zap.String("phase", job.GetPhase()),
						zap.Uint("video_id", job.GetVideoID()),
					)
				} else {
					result.Status = JobStatusFailed
					result.Error = err
					p.logger.Error("Worker job failed",
						zap.Int("worker_id", id),
						zap.String("job_id", job.GetID()),
						zap.String("phase", job.GetPhase()),
						zap.Uint("video_id", job.GetVideoID()),
						zap.Error(err),
					)
				}
			} else {
				result.Status = JobStatusCompleted
				result.Data = job
				p.logger.Debug("Worker job completed",
					zap.Int("worker_id", id),
					zap.String("job_id", job.GetID()),
					zap.String("phase", job.GetPhase()),
					zap.Uint("video_id", job.GetVideoID()),
				)
			}

			select {
			case p.resultChan <- result:
			case <-p.ctx.Done():
				return
			}
		}
	}
}

func (p *WorkerPool) Submit(job Job) error {
	if !p.running.Load() {
		return fmt.Errorf("worker pool is stopped")
	}

	// Check for duplicate job (same video+phase already in progress)
	if existingJobID := p.registry.Register(job); existingJobID != "" {
		return &DuplicateJobError{
			VideoID:       job.GetVideoID(),
			Phase:         job.GetPhase(),
			ExistingJobID: existingJobID,
		}
	}

	select {
	case <-p.ctx.Done():
		// Unregister since we couldn't queue the job
		p.registry.Unregister(job.GetID())
		return p.ctx.Err()
	case p.jobQueue <- job:
		p.logger.Debug("Job submitted to queue",
			zap.String("job_id", job.GetID()),
			zap.Int("queue_depth", p.QueueSize()),
		)
		return nil
	}
}

func (p *WorkerPool) Results() <-chan JobResult {
	return p.resultChan
}

func (p *WorkerPool) Stop() {
	if !p.running.CompareAndSwap(true, false) {
		return
	}

	p.logger.Info("Stopping worker pool gracefully",
		zap.Int("pending_jobs", p.QueueSize()),
		zap.Int("active_workers", p.workerCount),
	)

	p.cancel()
	close(p.jobQueue)
	p.wg.Wait()
	close(p.resultChan)

	p.logger.Info("Worker pool stopped")
}

func (p *WorkerPool) Running() bool {
	return p.running.Load()
}

func (p *WorkerPool) QueueSize() int {
	return len(p.jobQueue)
}

func (p *WorkerPool) ActiveWorkers() int {
	return p.workerCount
}

func (p *WorkerPool) LogStatus() {
	p.logger.Info("Worker pool status",
		zap.Int("queue_size", p.QueueSize()),
		zap.Int("active_workers", p.workerCount),
		zap.Int("queue_capacity", cap(p.jobQueue)),
		zap.Bool("running", p.running.Load()),
	)
}

// SetTimeout sets the job execution timeout. A timeout of 0 means no timeout.
func (p *WorkerPool) SetTimeout(timeout time.Duration) {
	p.timeout = timeout
}

// GetTimeout returns the current job execution timeout.
func (p *WorkerPool) GetTimeout() time.Duration {
	return p.timeout
}

// GetJob retrieves a job by its ID from the registry.
func (p *WorkerPool) GetJob(jobID string) (Job, bool) {
	return p.registry.Get(jobID)
}

// CancelJob cancels a job by its ID. Returns an error if the job is not found.
func (p *WorkerPool) CancelJob(jobID string) error {
	job, exists := p.registry.Get(jobID)
	if !exists {
		return fmt.Errorf("job not found: %s", jobID)
	}
	job.Cancel()
	p.logger.Info("Job cancelled",
		zap.String("job_id", jobID),
		zap.Uint("video_id", job.GetVideoID()),
		zap.String("phase", job.GetPhase()),
	)
	return nil
}

// Registry returns the job registry (for advanced use cases).
func (p *WorkerPool) Registry() *JobRegistry {
	return p.registry
}
