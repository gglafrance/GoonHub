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
	RecordJobComplete(jobID string, sceneID uint, phase string)
	RecordJobCancelled(jobID string)
	RecordJobFailedWithRetry(jobID string, sceneID uint, phase string, err error)
	UpdateProgress(jobID string, progress int)
}

// JobQueueRecorder extends JobHistoryRecorder with DB-backed queue methods
type JobQueueRecorder interface {
	JobHistoryRecorder
	// CreatePendingJob creates a job with status='pending' in the database
	CreatePendingJob(jobID string, sceneID uint, sceneTitle string, phase string, forceTarget string) error
	// CreatePendingJobWithPriority creates a pending job with a specific priority (higher = processed first)
	CreatePendingJobWithPriority(jobID string, sceneID uint, sceneTitle string, phase string, priority int, forceTarget string) error
	// CreatePendingJobWithRetry creates a pending job with retry tracking information
	CreatePendingJobWithRetry(jobID string, sceneID uint, sceneTitle string, phase string, retryCount, maxRetries int, forceTarget string) error
	// ExistsPendingOrRunning checks if a pending or running job exists for scene+phase
	ExistsPendingOrRunning(sceneID uint, phase string) (bool, error)
}

// FailingChecker tells which scenes are failing for a phase (consecutive failures on an
// unchanged file): the "missing" bulk runs skip them.
type FailingChecker interface {
	FailingSceneIDs(phase string) (map[uint]bool, error)
}

// SceneIndexer handles search index updates for scenes
type SceneIndexer interface {
	UpdateSceneIndex(scene *data.Scene) error
}
