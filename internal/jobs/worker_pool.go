package jobs

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// maxRetireTokens bounds the number of pending worker retirements (far above any pool size).
const maxRetireTokens = 1024

type WorkerPool struct {
	workerCount    atomic.Int32 // target number of workers
	jobQueue       chan Job
	resultChan     chan JobResult
	wg             sync.WaitGroup
	ctx            context.Context
	cancel         context.CancelFunc
	running        atomic.Bool
	activeCount    atomic.Int32 // Number of jobs currently being executed by workers
	logger         *zap.Logger
	registry       *JobRegistry
	timeout        time.Duration
	onJobExecuting func(jobID string) // called when a worker picks up a job for execution

	// sizeMu serializes Start, Stop and Resize (worker goroutines are added and retired here).
	sizeMu       sync.Mutex
	nextWorkerID int
	retire       chan struct{} // one token = one worker exits after its current job

	// Optional cross-pool limit on executing jobs, and the priority of this pool for its slots.
	limiter      *SlotLimiter
	slotPriority atomic.Int32

	// Jobs taken from the queue but never started (pool stopped while waiting for a slot).
	unstartedMu sync.Mutex
	unstarted   []string
}

func NewWorkerPool(workerCount int, queueSize int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	p := &WorkerPool{
		jobQueue:   make(chan Job, queueSize),
		resultChan: make(chan JobResult, queueSize),
		ctx:        ctx,
		cancel:     cancel,
		logger:     zap.NewNop(),
		registry:   NewJobRegistry(),
		timeout:    0, // no timeout by default
		retire:     make(chan struct{}, maxRetireTokens),
	}
	p.workerCount.Store(int32(workerCount))
	return p
}

// SetSlotLimiter makes the workers take a slot from a limiter shared with other pools before
// executing a job. Must be called before Start.
func (p *WorkerPool) SetSlotLimiter(limiter *SlotLimiter) {
	p.limiter = limiter
}

// SetSlotPriority sets the priority of the workers of this pool when they wait for a slot.
func (p *WorkerPool) SetSlotPriority(priority int) {
	p.slotPriority.Store(int32(priority))
}

func (p *WorkerPool) SetLogger(logger *zap.Logger) {
	p.logger = logger.With(zap.String("component", "worker_pool"))
}

// SetOnJobExecuting sets a callback invoked when a worker picks up a job for execution.
// Used to update the DB started_at timestamp to reflect actual execution start.
func (p *WorkerPool) SetOnJobExecuting(fn func(jobID string)) {
	p.onJobExecuting = fn
}

func (p *WorkerPool) Start() {
	p.sizeMu.Lock()
	defer p.sizeMu.Unlock()
	if !p.running.CompareAndSwap(false, true) {
		return
	}

	count := int(p.workerCount.Load())
	p.logger.Info("Starting worker pool",
		zap.Int("worker_count", count),
		zap.Int("queue_size", cap(p.jobQueue)),
	)

	for i := 0; i < count; i++ {
		p.spawnWorkerLocked()
	}

	p.logger.Info("All workers started and ready")
}

func (p *WorkerPool) spawnWorkerLocked() {
	id := p.nextWorkerID
	p.nextWorkerID++
	p.wg.Add(1)
	go p.worker(id)
}

// Resize changes the number of workers without interrupting anything: new workers start right
// away, and when shrinking, idle workers exit first while busy ones finish their current job.
// Jobs already queued stay queued.
func (p *WorkerPool) Resize(workerCount int) {
	if workerCount < 1 {
		workerCount = 1
	}
	p.sizeMu.Lock()
	defer p.sizeMu.Unlock()

	current := int(p.workerCount.Load())
	if workerCount == current {
		return
	}
	p.workerCount.Store(int32(workerCount))
	if !p.running.Load() {
		return // Start will use the new count
	}

	if workerCount > current {
		missing := workerCount - current
		// Cancel pending retirements first: those workers are still alive.
	reclaim:
		for missing > 0 {
			select {
			case <-p.retire:
				missing--
			default:
				break reclaim
			}
		}
		for ; missing > 0; missing-- {
			p.spawnWorkerLocked()
		}
	} else {
		for i := 0; i < current-workerCount; i++ {
			select {
			case p.retire <- struct{}{}:
			default:
				p.logger.Warn("Too many pending worker retirements, pool shrinks later than asked")
			}
		}
	}

	p.logger.Info("Resized worker pool",
		zap.Int("from", current),
		zap.Int("to", workerCount),
	)
}

func (p *WorkerPool) worker(id int) {
	defer p.wg.Done()
	p.logger.Debug("Worker started", zap.Int("worker_id", id))

	for {
		select {
		case <-p.ctx.Done():
			p.logger.Debug("Worker shutting down", zap.Int("worker_id", id))
			return
		case <-p.retire:
			p.logger.Debug("Worker retired after pool resize", zap.Int("worker_id", id))
			return
		case job := <-p.jobQueue:
			if job == nil {
				return
			}

			if p.limiter != nil {
				err := p.limiter.Acquire(p.ctx, int(p.slotPriority.Load()))
				if err == nil && p.ctx.Err() != nil {
					p.limiter.Release()
					err = p.ctx.Err()
				}
				if err != nil {
					// Pool stopping: the job never started, report it as reclaimable.
					p.registry.Unregister(job.GetID())
					p.unstartedMu.Lock()
					p.unstarted = append(p.unstarted, job.GetID())
					p.unstartedMu.Unlock()
					return
				}
			}

			p.activeCount.Add(1)
			p.registry.MarkExecuting(job.GetID())

			if p.onJobExecuting != nil {
				p.onJobExecuting(job.GetID())
			}

			p.logger.Info("Worker accepted job",
				zap.Int("worker_id", id),
				zap.String("job_id", job.GetID()),
				zap.String("job_status", string(job.GetStatus())),
				zap.Int("queue_depth", p.QueueSize()),
			)

			result := p.executeJob(id, job)
			if p.limiter != nil {
				p.limiter.Release()
			}

			select {
			case p.resultChan <- result:
			case <-p.ctx.Done():
				return
			}
		}
	}
}

// executeJob runs a single job with panic recovery, ensuring activeCount is always decremented.
func (p *WorkerPool) executeJob(workerID int, job Job) (result JobResult) {
	defer func() {
		p.activeCount.Add(-1)
		if r := recover(); r != nil {
			p.registry.Unregister(job.GetID())
			result = JobResult{
				JobID:   job.GetID(),
				SceneID: job.GetSceneID(),
				Phase:   job.GetPhase(),
				Status:  JobStatusFailed,
				Error:   fmt.Errorf("job panicked: %v", r),
			}
			p.logger.Error("Worker job panicked",
				zap.Int("worker_id", workerID),
				zap.String("job_id", job.GetID()),
				zap.String("phase", job.GetPhase()),
				zap.Uint("scene_id", job.GetSceneID()),
				zap.Any("panic", r),
			)
		}
	}()

	result = JobResult{
		JobID:   job.GetID(),
		SceneID: job.GetSceneID(),
		Phase:   job.GetPhase(),
	}

	// Create execution context with optional timeout
	var execCtx context.Context
	var execCancel context.CancelFunc
	timeout := p.timeout
	if scaler, ok := job.(TimeoutScaler); ok && timeout > 0 {
		timeout = scaler.JobTimeout(timeout)
	}
	if timeout > 0 {
		execCtx, execCancel = context.WithTimeout(p.ctx, timeout)
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
				zap.Int("worker_id", workerID),
				zap.String("job_id", job.GetID()),
				zap.String("phase", job.GetPhase()),
				zap.Uint("scene_id", job.GetSceneID()),
				zap.Duration("timeout", p.timeout),
			)
		} else if jobStatus == JobStatusCancelled {
			result.Status = JobStatusCancelled
			result.Error = err
			p.logger.Warn("Worker job cancelled",
				zap.Int("worker_id", workerID),
				zap.String("job_id", job.GetID()),
				zap.String("phase", job.GetPhase()),
				zap.Uint("scene_id", job.GetSceneID()),
			)
		} else {
			result.Status = JobStatusFailed
			result.Error = err
			p.logger.Error("Worker job failed",
				zap.Int("worker_id", workerID),
				zap.String("job_id", job.GetID()),
				zap.String("phase", job.GetPhase()),
				zap.Uint("scene_id", job.GetSceneID()),
				zap.Error(err),
			)
		}
	} else {
		result.Status = JobStatusCompleted
		result.Data = job
		p.logger.Debug("Worker job completed",
			zap.Int("worker_id", workerID),
			zap.String("job_id", job.GetID()),
			zap.String("phase", job.GetPhase()),
			zap.Uint("scene_id", job.GetSceneID()),
		)
	}

	return result
}

func (p *WorkerPool) Submit(job Job) error {
	if !p.running.Load() {
		return fmt.Errorf("worker pool is stopped")
	}

	// Check for duplicate job (same scene+phase already in progress)
	if existingJobID := p.registry.Register(job); existingJobID != "" {
		return &DuplicateJobError{
			SceneID:       job.GetSceneID(),
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
	p.sizeMu.Lock()
	stopped := p.running.CompareAndSwap(true, false)
	p.sizeMu.Unlock()
	if !stopped {
		return
	}

	p.logger.Info("Stopping worker pool gracefully",
		zap.Int("pending_jobs", p.QueueSize()),
		zap.Int("active_workers", p.ActiveWorkers()),
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

// ActiveJobCount returns the number of jobs currently being executed by workers.
func (p *WorkerPool) ActiveJobCount() int {
	return int(p.activeCount.Load())
}

func (p *WorkerPool) ActiveWorkers() int {
	return int(p.workerCount.Load())
}

func (p *WorkerPool) LogStatus() {
	p.logger.Info("Worker pool status",
		zap.Int("queue_size", p.QueueSize()),
		zap.Int("active_workers", p.ActiveWorkers()),
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

// GetExecutingJob retrieves a job by its ID only if it is actively being executed by a worker.
// Jobs that are registered but still sitting in the channel buffer are not returned.
func (p *WorkerPool) GetExecutingJob(jobID string) (Job, bool) {
	job, exists := p.registry.Get(jobID)
	if !exists {
		return nil, false
	}
	if !p.registry.IsExecuting(jobID) {
		return nil, false
	}
	return job, true
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
		zap.Uint("scene_id", job.GetSceneID()),
		zap.String("phase", job.GetPhase()),
	)
	return nil
}

// Registry returns the job registry (for advanced use cases).
func (p *WorkerPool) Registry() *JobRegistry {
	return p.registry
}

// GracefulStop performs graceful shutdown:
// 1. Stops accepting new jobs (sets running to false)
// 2. Waits for in-flight workers to finish (up to timeout)
// 3. Drains channel buffer and returns those job IDs
// The returned job IDs are jobs that were in the channel buffer but never executed.
func (p *WorkerPool) GracefulStop(timeout time.Duration) []string {
	p.sizeMu.Lock()
	stopped := p.running.CompareAndSwap(true, false)
	p.sizeMu.Unlock()
	if !stopped {
		return nil
	}

	p.logger.Info("Starting graceful shutdown of worker pool",
		zap.Int("pending_jobs", p.QueueSize()),
		zap.Int("active_workers", p.ActiveWorkers()),
		zap.Duration("timeout", timeout),
	)

	// Cancel context to signal workers to stop accepting new jobs from channel
	p.cancel()

	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.logger.Info("All workers finished gracefully")
	case <-time.After(timeout):
		p.logger.Warn("Timeout waiting for workers, proceeding with buffer drain",
			zap.Duration("timeout", timeout),
		)
	}

	// Drain channel buffer to get job IDs that were never executed
	bufferedJobIDs := p.drainBuffer()
	p.unstartedMu.Lock()
	bufferedJobIDs = append(bufferedJobIDs, p.unstarted...)
	p.unstarted = nil
	p.unstartedMu.Unlock()

	close(p.jobQueue)
	close(p.resultChan)

	p.logger.Info("Worker pool graceful shutdown complete",
		zap.Int("buffered_jobs_reclaimed", len(bufferedJobIDs)),
	)

	return bufferedJobIDs
}

// DrainQueued removes the jobs waiting in the queue (not yet picked up by a worker) and returns
// their IDs, without stopping the pool. Running jobs are not affected.
func (p *WorkerPool) DrainQueued() []string {
	if !p.running.Load() {
		return nil
	}
	return p.drainBuffer()
}

// drainBuffer extracts all jobs from the channel buffer without executing them.
// Returns the job IDs of all buffered jobs.
func (p *WorkerPool) drainBuffer() []string {
	var jobIDs []string

	// Non-blocking drain of the channel
	for {
		select {
		case job := <-p.jobQueue:
			if job != nil {
				jobIDs = append(jobIDs, job.GetID())
				// Unregister from registry since we're reclaiming
				p.registry.Unregister(job.GetID())
			}
		default:
			// Channel is empty
			return jobIDs
		}
	}
}

// CancelRegistered cancels the jobs of the pool that are registered but not executing (taken from
// the queue by a worker still waiting for a slot of the global cap) and, with includeExecuting,
// the jobs being executed. A cancelled job that has not started returns right away when a worker
// runs it, and its result goes through the usual cancelled path. Call DrainQueued first to take
// the queued jobs out instead. Returns the number of waiting and executing jobs cancelled.
func (p *WorkerPool) CancelRegistered(includeExecuting bool) (waiting, executing int) {
	waitingJobs, executingJobs := p.registry.Snapshot()
	for _, job := range waitingJobs {
		job.Cancel()
	}
	waiting = len(waitingJobs)
	if includeExecuting {
		for _, job := range executingJobs {
			job.Cancel()
		}
		executing = len(executingJobs)
	}
	if waiting > 0 || executing > 0 {
		p.logger.Info("Cancelled registered jobs",
			zap.Int("waiting", waiting),
			zap.Int("executing", executing),
		)
	}
	return waiting, executing
}
