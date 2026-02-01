package jobs

import (
	"fmt"
	"sync"
)

// JobRegistry provides thread-safe tracking of jobs for deduplication and cancellation.
type JobRegistry struct {
	mu           sync.RWMutex
	byID         map[string]Job    // job_id -> Job
	byScenePhase map[string]string // "sceneID:phase" -> job_id
}

// NewJobRegistry creates a new JobRegistry.
func NewJobRegistry() *JobRegistry {
	return &JobRegistry{
		byID:         make(map[string]Job),
		byScenePhase: make(map[string]string),
	}
}

// Register adds a job to the registry. Returns the existing job ID if a job
// for the same scene+phase is already registered (duplicate), otherwise returns
// an empty string.
func (r *JobRegistry) Register(job Job) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := scenePhaseKey(job.GetSceneID(), job.GetPhase())

	// Check for existing job with same scene+phase
	if existingJobID, exists := r.byScenePhase[key]; exists {
		return existingJobID
	}

	// Register the new job
	r.byID[job.GetID()] = job
	r.byScenePhase[key] = job.GetID()
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

	key := scenePhaseKey(job.GetSceneID(), job.GetPhase())
	delete(r.byID, jobID)
	delete(r.byScenePhase, key)
}

// Get retrieves a job by its ID.
func (r *JobRegistry) Get(jobID string) (Job, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	job, exists := r.byID[jobID]
	return job, exists
}

// GetByScenePhase retrieves a job by scene ID and phase.
func (r *JobRegistry) GetByScenePhase(sceneID uint, phase string) (Job, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := scenePhaseKey(sceneID, phase)
	jobID, exists := r.byScenePhase[key]
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

// scenePhaseKey generates a unique key for scene+phase combination.
func scenePhaseKey(sceneID uint, phase string) string {
	return fmt.Sprintf("%d:%s", sceneID, phase)
}
