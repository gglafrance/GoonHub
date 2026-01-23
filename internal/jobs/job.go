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
