package core

import (
	"go.uber.org/zap"
)

// JobStatus represents the aggregated job status for the header indicator
type JobStatus struct {
	TotalRunning int                    `json:"total_running"`
	TotalQueued  int                    `json:"total_queued"`
	TotalPending int                    `json:"total_pending"`
	ByPhase      map[string]PhaseStatus `json:"by_phase"`
	ActiveJobs   []ActiveJob            `json:"active_jobs"`
	MoreCount    int                    `json:"more_count"`
}

// PhaseStatus represents the running/queued/pending counts for a single processing phase
type PhaseStatus struct {
	Running int `json:"running"`
	Queued  int `json:"queued"`
	Pending int `json:"pending"`
}

// ActiveJob represents a currently running job for display in the header popup
type ActiveJob struct {
	JobID      string `json:"job_id"`
	SceneID    uint   `json:"scene_id"`
	SceneTitle string `json:"scene_title"`
	Phase      string `json:"phase"`
	StartedAt  string `json:"started_at"`
}

// JobStatusService provides aggregated job status for real-time header display
type JobStatusService struct {
	jobHistoryService *JobHistoryService
	processingService *SceneProcessingService
	logger            *zap.Logger
}

// NewJobStatusService creates a new JobStatusService
func NewJobStatusService(
	jobHistoryService *JobHistoryService,
	processingService *SceneProcessingService,
	logger *zap.Logger,
) *JobStatusService {
	return &JobStatusService{
		jobHistoryService: jobHistoryService,
		processingService: processingService,
		logger:            logger.With(zap.String("component", "job_status")),
	}
}

// GetJobStatus returns the current aggregated job status
func (s *JobStatusService) GetJobStatus() *JobStatus {
	// Get queue status (queued + active counts per phase from worker pools)
	queueStatus := s.processingService.GetQueueStatus()

	// Get pending counts from database (jobs waiting in DB queue)
	pendingByPhase, err := s.jobHistoryService.CountPendingByPhase()
	if err != nil {
		s.logger.Error("Failed to count pending jobs", zap.Error(err))
		pendingByPhase = make(map[string]int)
	}

	// Get active jobs from job history (status='running') for display in popup
	activeJobs, err := s.jobHistoryService.ListActiveJobs()
	if err != nil {
		s.logger.Error("Failed to list active jobs", zap.Error(err))
		activeJobs = nil
	}

	// Use worker pool active counts for accurate running numbers.
	// Previously we computed running = dbRunning - channelQueued, which had a race
	// condition: between reading the channel size and the DB count, the feeder could
	// claim jobs (increasing DB running count while channel count was stale).
	// Now we use the pool's atomic active counter which tracks jobs truly being
	// executed by workers, independent of the DB state.
	metadataRunning := queueStatus.MetadataActive
	thumbnailRunning := queueStatus.ThumbnailActive
	spritesRunning := queueStatus.SpritesActive

	// Build phase status map with pending counts
	byPhase := map[string]PhaseStatus{
		"metadata": {
			Running: metadataRunning,
			Queued:  queueStatus.MetadataQueued,
			Pending: pendingByPhase["metadata"],
		},
		"thumbnail": {
			Running: thumbnailRunning,
			Queued:  queueStatus.ThumbnailQueued,
			Pending: pendingByPhase["thumbnail"],
		},
		"sprites": {
			Running: spritesRunning,
			Queued:  queueStatus.SpritesQueued,
			Pending: pendingByPhase["sprites"],
		},
	}

	// Calculate totals
	totalRunning := metadataRunning + thumbnailRunning + spritesRunning
	totalQueued := queueStatus.MetadataQueued + queueStatus.ThumbnailQueued + queueStatus.SpritesQueued
	totalPending := pendingByPhase["metadata"] + pendingByPhase["thumbnail"] + pendingByPhase["sprites"]

	// Limit active jobs list to 5 for display
	const maxActiveJobs = 5
	displayJobs := make([]ActiveJob, 0, maxActiveJobs)
	moreCount := 0

	for i, job := range activeJobs {
		if i >= maxActiveJobs {
			moreCount = len(activeJobs) - maxActiveJobs
			break
		}
		displayJobs = append(displayJobs, ActiveJob{
			JobID:      job.JobID,
			SceneID:    job.SceneID,
			SceneTitle: job.SceneTitle,
			Phase:      job.Phase,
			StartedAt:  job.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	return &JobStatus{
		TotalRunning: totalRunning,
		TotalQueued:  totalQueued,
		TotalPending: totalPending,
		ByPhase:      byPhase,
		ActiveJobs:   displayJobs,
		MoreCount:    moreCount,
	}
}
