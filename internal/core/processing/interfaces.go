package processing

import (
	"goonhub/internal/data"
)

// EventPublisher publishes video events
type EventPublisher interface {
	Publish(event VideoEvent)
}

// VideoEvent represents an event related to video processing
type VideoEvent struct {
	Type    string
	VideoID uint
	Data    map[string]any
}

// JobHistoryRecorder records job history events
type JobHistoryRecorder interface {
	RecordJobStart(jobID string, videoID uint, videoTitle string, phase string)
	RecordJobStartWithRetry(jobID string, videoID uint, videoTitle string, phase string, maxRetries int, retryCount int)
	RecordJobComplete(jobID string)
	RecordJobCancelled(jobID string)
	RecordJobFailedWithRetry(jobID string, videoID uint, phase string, err error)
}

// VideoIndexer handles search index updates for videos
type VideoIndexer interface {
	UpdateVideoIndex(video *data.Video) error
}
