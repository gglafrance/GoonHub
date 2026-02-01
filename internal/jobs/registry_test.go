package jobs

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

// registryTestJob is a minimal Job implementation for registry testing
type registryTestJob struct {
	id      string
	sceneID uint
	phase   string
}

func newRegistryTestJob(id string, sceneID uint, phase string) *registryTestJob {
	return &registryTestJob{id: id, sceneID: sceneID, phase: phase}
}

func (j *registryTestJob) Execute() error                            { return nil }
func (j *registryTestJob) ExecuteWithContext(ctx context.Context) error { return nil }
func (j *registryTestJob) Cancel()                                   {}
func (j *registryTestJob) GetID() string                             { return j.id }
func (j *registryTestJob) GetSceneID() uint                          { return j.sceneID }
func (j *registryTestJob) GetPhase() string                          { return j.phase }
func (j *registryTestJob) GetStatus() JobStatus                      { return JobStatusPending }
func (j *registryTestJob) GetError() error                           { return nil }

func TestRegistry_RegisterAndGet(t *testing.T) {
	registry := NewJobRegistry()

	job := newRegistryTestJob("job-1", 100, "metadata")

	// Register should return empty string for new job
	existingID := registry.Register(job)
	if existingID != "" {
		t.Fatalf("expected empty string for new job, got %s", existingID)
	}

	// Get should return the registered job
	retrieved, ok := registry.Get("job-1")
	if !ok {
		t.Fatal("expected to find job by ID")
	}
	if retrieved.GetID() != "job-1" {
		t.Fatalf("expected job ID 'job-1', got %s", retrieved.GetID())
	}

	// Count should be 1
	if registry.Count() != 1 {
		t.Fatalf("expected count 1, got %d", registry.Count())
	}
}

func TestRegistry_RegisterDuplicate(t *testing.T) {
	registry := NewJobRegistry()

	job1 := newRegistryTestJob("job-1", 100, "metadata")
	job2 := newRegistryTestJob("job-2", 100, "metadata") // Same scene+phase

	// Register first job
	existingID := registry.Register(job1)
	if existingID != "" {
		t.Fatalf("expected empty string for first job, got %s", existingID)
	}

	// Register second job with same scene+phase should return existing job ID
	existingID = registry.Register(job2)
	if existingID != "job-1" {
		t.Fatalf("expected existing job ID 'job-1', got %s", existingID)
	}

	// Only one job should be registered
	if registry.Count() != 1 {
		t.Fatalf("expected count 1, got %d", registry.Count())
	}

	// The registered job should still be job-1
	retrieved, ok := registry.Get("job-1")
	if !ok {
		t.Fatal("expected to find job-1")
	}
	if retrieved.GetID() != "job-1" {
		t.Fatalf("expected job ID 'job-1', got %s", retrieved.GetID())
	}

	// job-2 should not be in registry
	_, ok = registry.Get("job-2")
	if ok {
		t.Fatal("job-2 should not be in registry")
	}
}

func TestRegistry_RegisterDifferentPhase(t *testing.T) {
	registry := NewJobRegistry()

	job1 := newRegistryTestJob("job-1", 100, "metadata")
	job2 := newRegistryTestJob("job-2", 100, "thumbnail") // Same scene, different phase

	// Register first job
	existingID := registry.Register(job1)
	if existingID != "" {
		t.Fatalf("expected empty string for first job, got %s", existingID)
	}

	// Register second job with different phase should succeed
	existingID = registry.Register(job2)
	if existingID != "" {
		t.Fatalf("expected empty string for job with different phase, got %s", existingID)
	}

	// Both jobs should be registered
	if registry.Count() != 2 {
		t.Fatalf("expected count 2, got %d", registry.Count())
	}
}

func TestRegistry_Unregister(t *testing.T) {
	registry := NewJobRegistry()

	job := newRegistryTestJob("job-1", 100, "metadata")

	registry.Register(job)
	if registry.Count() != 1 {
		t.Fatalf("expected count 1, got %d", registry.Count())
	}

	// Unregister the job
	registry.Unregister("job-1")

	if registry.Count() != 0 {
		t.Fatalf("expected count 0, got %d", registry.Count())
	}

	// Get should return false
	_, ok := registry.Get("job-1")
	if ok {
		t.Fatal("expected job to be unregistered")
	}

	// GetByScenePhase should return false
	_, ok = registry.GetByScenePhase(100, "metadata")
	if ok {
		t.Fatal("expected scene+phase to be unregistered")
	}
}

func TestRegistry_UnregisterAllowsResubmit(t *testing.T) {
	registry := NewJobRegistry()

	job1 := newRegistryTestJob("job-1", 100, "metadata")
	job2 := newRegistryTestJob("job-2", 100, "metadata")

	// Register and unregister first job
	registry.Register(job1)
	registry.Unregister("job-1")

	// Should be able to register new job for same scene+phase
	existingID := registry.Register(job2)
	if existingID != "" {
		t.Fatalf("expected empty string after unregister, got %s", existingID)
	}

	// job-2 should be registered
	retrieved, ok := registry.Get("job-2")
	if !ok {
		t.Fatal("expected to find job-2")
	}
	if retrieved.GetID() != "job-2" {
		t.Fatalf("expected job ID 'job-2', got %s", retrieved.GetID())
	}
}

func TestRegistry_GetByScenePhase(t *testing.T) {
	registry := NewJobRegistry()

	job := newRegistryTestJob("job-1", 100, "metadata")
	registry.Register(job)

	// GetByScenePhase should return the job
	retrieved, ok := registry.GetByScenePhase(100, "metadata")
	if !ok {
		t.Fatal("expected to find job by scene+phase")
	}
	if retrieved.GetID() != "job-1" {
		t.Fatalf("expected job ID 'job-1', got %s", retrieved.GetID())
	}

	// Different scene should not find the job
	_, ok = registry.GetByScenePhase(200, "metadata")
	if ok {
		t.Fatal("expected no job for different scene")
	}

	// Different phase should not find the job
	_, ok = registry.GetByScenePhase(100, "thumbnail")
	if ok {
		t.Fatal("expected no job for different phase")
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	registry := NewJobRegistry()
	var wg sync.WaitGroup

	// Concurrent registrations
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			job := newRegistryTestJob(fmt.Sprintf("job-%d", id), uint(id), "metadata")
			registry.Register(job)
		}(i)
	}
	wg.Wait()

	if registry.Count() != 100 {
		t.Fatalf("expected 100 jobs, got %d", registry.Count())
	}

	// Concurrent reads and unregistrations
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(id int) {
			defer wg.Done()
			registry.Get(fmt.Sprintf("job-%d", id))
		}(i)
		go func(id int) {
			defer wg.Done()
			registry.Unregister(fmt.Sprintf("job-%d", id))
		}(i)
	}
	wg.Wait()

	if registry.Count() != 0 {
		t.Fatalf("expected 0 jobs after unregister, got %d", registry.Count())
	}
}

func TestRegistry_UnregisterNonExistent(t *testing.T) {
	registry := NewJobRegistry()

	// Unregistering a non-existent job should not panic
	registry.Unregister("non-existent")

	// Registry should remain empty
	if registry.Count() != 0 {
		t.Fatalf("expected count 0, got %d", registry.Count())
	}
}
