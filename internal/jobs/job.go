package jobs

type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

type Job interface {
	Execute() error
	Cancel()
	GetID() string
	GetStatus() JobStatus
	GetError() error
}

type JobResult struct {
	JobID  string
	Status JobStatus
	Error  error
}
