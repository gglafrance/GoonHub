package processing

import (
	"goonhub/internal/data"
)

// EventPublisher publishes scene events
type EventPublisher interface {
	Publish(event SceneEvent)
}

// SceneEvent represents an event related to scene processing
type SceneEvent struct {
	Type    string
	SceneID uint
	Data    map[string]any
}

// JobHistoryRecorder records job history events
type JobHistoryRecorder interface {
	RecordJobStart(jobID string, sceneID uint, sceneTitle string, phase string)
	RecordJobStartWithRetry(jobID string, sceneID uint, sceneTitle string, phase string, maxRetries int, retryCount int)
	RecordJobComplete(jobID string)
	RecordJobCancelled(jobID string)
	RecordJobFailedWithRetry(jobID string, sceneID uint, phase string, err error)
}

// JobQueueRecorder extends JobHistoryRecorder with DB-backed queue methods
type JobQueueRecorder interface {
	JobHistoryRecorder
	// CreatePendingJob creates a job with status='pending' in the database
	CreatePendingJob(jobID string, sceneID uint, sceneTitle string, phase string) error
	// ExistsPendingOrRunning checks if a pending or running job exists for scene+phase
	ExistsPendingOrRunning(sceneID uint, phase string) (bool, error)
}

// SceneIndexer handles search index updates for scenes
type SceneIndexer interface {
	UpdateSceneIndex(scene *data.Scene) error
}
