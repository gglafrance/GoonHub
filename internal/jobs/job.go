package jobs

import (
	"context"
	"errors"
	"fmt"
)

type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
	JobStatusTimedOut  JobStatus = "timed_out"
)

// DuplicateJobError is returned when attempting to submit a job that already exists.
type DuplicateJobError struct {
	SceneID       uint
	Phase         string
	ExistingJobID string
}

func (e *DuplicateJobError) Error() string {
	return fmt.Sprintf("duplicate job: scene %d phase %s already has job %s", e.SceneID, e.Phase, e.ExistingJobID)
}

// IsDuplicateJobError checks if an error is a DuplicateJobError.
func IsDuplicateJobError(err error) bool {
	var dupErr *DuplicateJobError
	return errors.As(err, &dupErr)
}

type Job interface {
	Execute() error
	ExecuteWithContext(ctx context.Context) error
	Cancel()
	GetID() string
	GetSceneID() uint
	GetPhase() string
	GetStatus() JobStatus
	GetError() error
}

type JobResult struct {
	JobID   string
	SceneID uint
	Phase   string
	Status  JobStatus
	Error   error
	Data    any
}

// ProgressCallback is a function type for reporting job progress.
type ProgressCallback func(jobID string, progress int)

// ProgressReporter is an interface for jobs that support progress reporting.
type ProgressReporter interface {
	SetProgressCallback(callback ProgressCallback)
}
