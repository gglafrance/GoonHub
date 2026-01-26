package jobs

import (
	"fmt"
	"sync"
)

// JobRegistry provides thread-safe tracking of jobs for deduplication and cancellation.
type JobRegistry struct {
	mu           sync.RWMutex
	byID         map[string]Job    // job_id -> Job
	byVideoPhase map[string]string // "videoID:phase" -> job_id
}

// NewJobRegistry creates a new JobRegistry.
func NewJobRegistry() *JobRegistry {
	return &JobRegistry{
		byID:         make(map[string]Job),
		byVideoPhase: make(map[string]string),
	}
}

// Register adds a job to the registry. Returns the existing job ID if a job
// for the same video+phase is already registered (duplicate), otherwise returns
// an empty string.
func (r *JobRegistry) Register(job Job) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := videoPhaseKey(job.GetVideoID(), job.GetPhase())

	// Check for existing job with same video+phase
	if existingJobID, exists := r.byVideoPhase[key]; exists {
		return existingJobID
	}

	// Register the new job
	r.byID[job.GetID()] = job
	r.byVideoPhase[key] = job.GetID()
	return ""
}

// Unregister removes a job from the registry.
func (r *JobRegistry) Unregister(jobID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	job, exists := r.byID[jobID]
	if !exists {
		return
	}

	key := videoPhaseKey(job.GetVideoID(), job.GetPhase())
	delete(r.byID, jobID)
	delete(r.byVideoPhase, key)
}

// Get retrieves a job by its ID.
func (r *JobRegistry) Get(jobID string) (Job, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	job, exists := r.byID[jobID]
	return job, exists
}

// GetByVideoPhase retrieves a job by video ID and phase.
func (r *JobRegistry) GetByVideoPhase(videoID uint, phase string) (Job, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := videoPhaseKey(videoID, phase)
	jobID, exists := r.byVideoPhase[key]
	if !exists {
		return nil, false
	}

	job, exists := r.byID[jobID]
	return job, exists
}

// Count returns the number of registered jobs.
func (r *JobRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byID)
}

// videoPhaseKey generates a unique key for video+phase combination.
func videoPhaseKey(videoID uint, phase string) string {
	return fmt.Sprintf("%d:%s", videoID, phase)
}
