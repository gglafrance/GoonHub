package jobs

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

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
				JobID: job.GetID(),
			}

			if err := job.Execute(); err != nil {
				result.Status = JobStatusFailed
				result.Error = err
				p.logger.Error("Worker job failed",
					zap.Int("worker_id", id),
					zap.String("job_id", job.GetID()),
					zap.Error(err),
				)
			} else {
				result.Status = JobStatusCompleted
				p.logger.Debug("Worker job completed",
					zap.Int("worker_id", id),
					zap.String("job_id", job.GetID()),
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
	select {
	case <-p.ctx.Done():
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
