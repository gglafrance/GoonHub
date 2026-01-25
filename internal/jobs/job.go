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
	VideoID       uint
	Phase         string
	ExistingJobID string
}

func (e *DuplicateJobError) Error() string {
	return fmt.Sprintf("duplicate job: video %d phase %s already has job %s", e.VideoID, e.Phase, e.ExistingJobID)
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
	GetVideoID() uint
	GetPhase() string
	GetStatus() JobStatus
	GetError() error
}

type JobResult struct {
	JobID   string
	VideoID uint
	Phase   string
	Status  JobStatus
	Error   error
	Data    any
}
