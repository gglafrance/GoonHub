package data

import (
	"fmt"
	"time"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type JobHistoryRepository interface {
	Create(record *JobHistory) error
	UpdateStatus(jobID string, status string, errorMessage *string, completedAt *time.Time) error
	ListAll(page, limit int, status string) ([]JobHistory, int64, error)
	ListRecentFailed(limit int, since time.Duration) ([]JobHistory, error)
	ListActive() ([]JobHistory, error)
	// History purge (finished jobs only, never pending/running)
	PurgeFinishedBatch(statuses []string, before time.Time, limit int) (int64, error)
	CountPurgeable(statuses []string, before time.Time) (map[string]int64, error)
	UpdateProgress(jobID string, progress int) error
	UpdateRetryInfo(jobID string, retryCount, maxRetries int, nextRetryAt *time.Time) error
	GetRetryableJobs() ([]JobHistory, error)
	MarkNotRetryable(jobID string) error
	GetByJobID(jobID string) (*JobHistory, error)
	IncrementRetryCount(jobID string) error

	// DB-backed job queue methods
	CreatePending(record *JobHistory) error
	CreateBatch(records []*JobHistory) error
	ClaimPendingJobs(phase string, limit int, opts ClaimOptions) ([]JobHistory, error)
	CountPendingByPhase() (map[string]int, error)
	ExistsPendingOrRunning(sceneID uint, phase string) (bool, error)
	MarkOrphanedRunningAsFailed(olderThan time.Duration) (int64, error)

	// Graceful shutdown methods
	ResetJobsToPending(jobIDs []string) (int64, error)
	MarkRunningAsInterrupted() (int64, error)
	MarkStuckPendingJobsAsFailed(olderThan time.Duration, excludePhases []string) (int64, error)
	// RequeueInterruptedJobs puts back to pending, at startup, the jobs the previous run
	// interrupted (see the method for the exact rules).
	RequeueInterruptedJobs() (int64, error)

	// Scene-specific methods
	CancelPendingJobsForScene(sceneID uint) (int64, error)
	CancelPendingJob(jobID string) error

	// Bulk cancellation
	CancelPendingJobsByPhases(phases []string, reason string) (int64, error)
	CancelClaimedJobs(jobIDs []string, reason string) (int64, error)

	// Job execution tracking
	UpdateStartedAt(jobID string, startedAt time.Time) error

	// Monitoring methods
	CountRecentFailedByPhase(since time.Duration) (map[string]int, error)

	// Bulk operations
	GetFailedJobs() ([]JobHistory, error)
	DeleteByStatus(status string) (int64, error)
}

type JobHistoryRepositoryImpl struct {
	DB *gorm.DB
}

func NewJobHistoryRepository(db *gorm.DB) *JobHistoryRepositoryImpl {
	return &JobHistoryRepositoryImpl{DB: db}
}

func (r *JobHistoryRepositoryImpl) Create(record *JobHistory) error {
	return r.DB.Create(record).Error
}

func (r *JobHistoryRepositoryImpl) UpdateStatus(jobID string, status string, errorMessage *string, completedAt *time.Time) error {
	updates := map[string]any{
		"status": status,
	}
	if errorMessage != nil {
		updates["error_message"] = *errorMessage
	}
	if completedAt != nil {
		updates["completed_at"] = *completedAt
	}
	return r.DB.Model(&JobHistory{}).Where("job_id = ?", jobID).Updates(updates).Error
}

func (r *JobHistoryRepositoryImpl) UpdateStartedAt(jobID string, startedAt time.Time) error {
	return r.DB.Model(&JobHistory{}).Where("job_id = ?", jobID).Update("started_at", startedAt).Error
}

func (r *JobHistoryRepositoryImpl) ListAll(page, limit int, status string) ([]JobHistory, int64, error) {
	var records []JobHistory
	var total int64

	offset := (page - 1) * limit

	query := r.DB.Model(&JobHistory{})
	if status != "" {
		query = query.Where("status = ?", status)
	} else {
		query = query.Where("status != ?", "running")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	listQuery := r.DB.Model(&JobHistory{})
	if status != "" {
		listQuery = listQuery.Where("status = ?", status)
	} else {
		listQuery = listQuery.Where("status != ?", "running")
	}

	if err := listQuery.Limit(limit).Offset(offset).Order("started_at desc").Find(&records).Error; err != nil {
		return nil, 0, err
	}

	return records, total, nil
}

func (r *JobHistoryRepositoryImpl) ListRecentFailed(limit int, since time.Duration) ([]JobHistory, error) {
	var records []JobHistory
	cutoff := time.Now().Add(-since)

	if err := r.DB.Where("status = ? AND completed_at >= ?", JobStatusFailed, cutoff).
		Order("completed_at desc").
		Limit(limit).
		Find(&records).Error; err != nil {
		return nil, err
	}

	return records, nil
}

func (r *JobHistoryRepositoryImpl) ListActive() ([]JobHistory, error) {
	var records []JobHistory
	if err := r.DB.Where("status = ?", "running").Order("started_at desc").Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

func (r *JobHistoryRepositoryImpl) UpdateProgress(jobID string, progress int) error {
	return r.DB.Model(&JobHistory{}).Where("job_id = ?", jobID).Update("progress", progress).Error
}

func (r *JobHistoryRepositoryImpl) UpdateRetryInfo(jobID string, retryCount, maxRetries int, nextRetryAt *time.Time) error {
	updates := map[string]any{
		"retry_count": retryCount,
		"max_retries": maxRetries,
	}
	if nextRetryAt != nil {
		updates["next_retry_at"] = *nextRetryAt
	} else {
		updates["next_retry_at"] = nil
	}
	return r.DB.Model(&JobHistory{}).Where("job_id = ?", jobID).Updates(updates).Error
}

func (r *JobHistoryRepositoryImpl) GetRetryableJobs() ([]JobHistory, error) {
	var jobs []JobHistory
	now := time.Now()
	if err := r.DB.Where("status = ? AND is_retryable = ? AND next_retry_at IS NOT NULL AND next_retry_at <= ?",
		"failed", true, now).
		Order("next_retry_at asc").
		Find(&jobs).Error; err != nil {
		return nil, err
	}
	return jobs, nil
}

func (r *JobHistoryRepositoryImpl) MarkNotRetryable(jobID string) error {
	return r.DB.Model(&JobHistory{}).Where("job_id = ?", jobID).Updates(map[string]any{
		"is_retryable":  false,
		"next_retry_at": nil,
	}).Error
}

func (r *JobHistoryRepositoryImpl) GetByJobID(jobID string) (*JobHistory, error) {
	var job JobHistory
	if err := r.DB.Where("job_id = ?", jobID).First(&job).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *JobHistoryRepositoryImpl) IncrementRetryCount(jobID string) error {
	return r.DB.Model(&JobHistory{}).Where("job_id = ?", jobID).
		UpdateColumn("retry_count", gorm.Expr("retry_count + 1")).Error
}

// CreatePending creates a job with status='pending'
func (r *JobHistoryRepositoryImpl) CreatePending(record *JobHistory) error {
	record.Status = JobStatusPending
	return r.DB.Create(record).Error
}

// CreateBatch inserts multiple pending jobs efficiently
func (r *JobHistoryRepositoryImpl) CreateBatch(records []*JobHistory) error {
	if len(records) == 0 {
		return nil
	}
	for _, record := range records {
		record.Status = JobStatusPending
	}
	return r.DB.CreateInBatches(records, 100).Error
}

// ClaimOptions tunes the order in which ClaimPendingJobs picks pending jobs.
type ClaimOptions struct {
	// NewSceneSince, when not zero, claims the jobs of scenes created at or after this time
	// before the other jobs of the same priority ("new scenes first").
	NewSceneSince time.Time
}

// ClaimPendingJobs atomically claims up to 'limit' pending jobs for a phase.
// Uses FOR UPDATE SKIP LOCKED, sets status='running' and StartedAt.
// Order: job priority (higher first), then, with opts.NewSceneSince, jobs of recently created
// scenes, then the oldest job first.
func (r *JobHistoryRepositoryImpl) ClaimPendingJobs(phase string, limit int, opts ClaimOptions) ([]JobHistory, error) {
	var jobs []JobHistory

	silentDB := r.DB.Session(&gorm.Session{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	err := silentDB.Transaction(func(tx *gorm.DB) error {
		// Select pending jobs with lock, skipping already locked rows
		var selectErr error
		if opts.NewSceneSince.IsZero() {
			selectErr = tx.Raw(`
				SELECT * FROM job_history
				WHERE phase = ? AND status = 'pending'
				ORDER BY priority DESC, created_at ASC
				LIMIT ?
				FOR UPDATE SKIP LOCKED
			`, phase, limit).Scan(&jobs).Error
		} else {
			// A job whose scene is gone sorts with the old ones; the feeder fails it.
			selectErr = tx.Raw(`
				SELECT jh.* FROM job_history jh
				LEFT JOIN scenes s ON s.id = jh.scene_id
				WHERE jh.phase = ? AND jh.status = 'pending'
				ORDER BY jh.priority DESC,
					(s.created_at IS NOT NULL AND s.created_at >= ?) DESC,
					jh.created_at ASC
				LIMIT ?
				FOR UPDATE OF jh SKIP LOCKED
			`, phase, opts.NewSceneSince, limit).Scan(&jobs).Error
		}
		if selectErr != nil {
			return selectErr
		}

		if len(jobs) == 0 {
			return nil
		}

		// Collect IDs to update
		ids := make([]uint, len(jobs))
		for i, job := range jobs {
			ids[i] = job.ID
		}

		// Update status to running and set started_at
		now := time.Now()
		if err := tx.Model(&JobHistory{}).
			Where("id IN ?", ids).
			Updates(map[string]any{
				"status":     JobStatusRunning,
				"started_at": now,
			}).Error; err != nil {
			return err
		}

		// Update the returned jobs to reflect the new status
		for i := range jobs {
			jobs[i].Status = JobStatusRunning
			jobs[i].StartedAt = now
		}

		return nil
	})

	return jobs, err
}

// CountPendingByPhase returns pending count per phase
func (r *JobHistoryRepositoryImpl) CountPendingByPhase() (map[string]int, error) {
	type phaseCount struct {
		Phase string
		Count int
	}

	var counts []phaseCount
	if err := r.DB.Model(&JobHistory{}).
		Select("phase, COUNT(*) as count").
		Where("status = ?", JobStatusPending).
		Group("phase").
		Scan(&counts).Error; err != nil {
		return nil, err
	}

	result := make(map[string]int)
	for _, c := range counts {
		result[c.Phase] = c.Count
	}

	return result, nil
}

// ExistsPendingOrRunning checks if scene+phase already has a pending or running job
func (r *JobHistoryRepositoryImpl) ExistsPendingOrRunning(sceneID uint, phase string) (bool, error) {
	var count int64
	if err := r.DB.Model(&JobHistory{}).
		Where("scene_id = ? AND phase = ? AND status IN ?", sceneID, phase, []string{JobStatusPending, JobStatusRunning}).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// MarkOrphanedRunningAsFailed marks jobs that have been running for too long as failed.
// These are likely orphaned jobs from a previous server crash.
func (r *JobHistoryRepositoryImpl) MarkOrphanedRunningAsFailed(olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	errMsg := JobErrorOrphanRecovered

	result := r.DB.Model(&JobHistory{}).
		// Remote leases (worker_id set) are not orphans: the lease expiry handles them.
		Where("status = ? AND started_at < ? AND worker_id IS NULL", JobStatusRunning, cutoff).
		Updates(map[string]any{
			"status":        JobStatusFailed,
			"error_message": errMsg,
			"completed_at":  time.Now(),
			"is_retryable":  true,
		})

	return result.RowsAffected, result.Error
}

// ResetJobsToPending resets jobs by their IDs back to pending status.
// Used during graceful shutdown to reclaim jobs that were in channel buffers.
// Note: We keep the original started_at value since the column is NOT NULL.
// When the job is re-claimed, ClaimPendingJobs will update started_at.
func (r *JobHistoryRepositoryImpl) ResetJobsToPending(jobIDs []string) (int64, error) {
	if len(jobIDs) == 0 {
		return 0, nil
	}

	result := r.DB.Model(&JobHistory{}).
		Where("job_id IN ? AND worker_id IS NULL", jobIDs).
		Update("status", JobStatusPending)

	return result.RowsAffected, result.Error
}

// MarkRunningAsInterrupted marks all currently running jobs as failed due to server shutdown.
// These jobs will be retryable so the retry scheduler can pick them up.
func (r *JobHistoryRepositoryImpl) MarkRunningAsInterrupted() (int64, error) {
	errMsg := JobErrorInterruptedByShutdown
	now := time.Now()

	// Remote leases are kept: they survive a server restart (startup lease grace).
	result := r.DB.Model(&JobHistory{}).
		Where("status = ? AND worker_id IS NULL", JobStatusRunning).
		Updates(map[string]any{
			"status":        JobStatusFailed,
			"error_message": errMsg,
			"completed_at":  now,
			"is_retryable":  true,
		})

	return result.RowsAffected, result.Error
}

// MarkStuckPendingJobsAsFailed marks pending jobs that have been stuck for too long as failed.
// This handles edge cases where jobs got stuck in pending state.
// Phases in excludePhases are left alone (their jobs may wait for a time window on purpose).
func (r *JobHistoryRepositoryImpl) MarkStuckPendingJobsAsFailed(olderThan time.Duration, excludePhases []string) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	errMsg := JobErrorStuckPendingRecovered

	query := r.DB.Model(&JobHistory{}).
		Where("status = ? AND created_at < ?", JobStatusPending, cutoff)
	if len(excludePhases) > 0 {
		query = query.Where("phase NOT IN ?", excludePhases)
	}
	result := query.
		Updates(map[string]any{
			"status":        JobStatusFailed,
			"error_message": errMsg,
			"completed_at":  time.Now(),
			"is_retryable":  true,
		})

	return result.RowsAffected, result.Error
}

// RequeueInterruptedJobs puts back to pending the jobs the previous run did not let finish: the
// jobs failed by the server shutdown (JobErrorInterruptedByShutdown), the running jobs and the
// pending jobs recovered at boot (JobErrorOrphanRecovered, JobErrorStuckPendingRecovered).
// Rules:
//   - a job is put back once: a job that ran and was interrupted again after being put back stays
//     failed (it may be what brings the server down); a stuck pending job never ran and is always
//     put back;
//   - only the latest job of its (scene, phase): a job queued since (by hand, by a trigger) wins;
//   - never when the (scene, phase) already has a pending or running job (active-job dedup);
//   - never for a job retried by hand (is_retryable false) or a scene deleted or in the trash.
//
// The retry count is kept (an interruption is not an attempt) and so are the priority and the
// creation date (the job keeps its place in the queue).
func (r *JobHistoryRepositoryImpl) RequeueInterruptedJobs() (int64, error) {
	result := r.DB.Exec(`
		UPDATE job_history jh SET
			status = ?,
			error_message = NULL,
			completed_at = NULL,
			next_retry_at = NULL,
			progress = 0,
			startup_requeues = jh.startup_requeues + CASE WHEN jh.error_message = ? THEN 0 ELSE 1 END
		FROM scenes s
		WHERE s.id = jh.scene_id AND s.deleted_at IS NULL AND s.trashed_at IS NULL
			AND jh.status = ? AND jh.is_retryable
			AND jh.error_message IN (?, ?, ?)
			AND (jh.startup_requeues = 0 OR jh.error_message = ?)
			AND NOT EXISTS (
				SELECT 1 FROM job_history later
				WHERE later.scene_id = jh.scene_id AND later.phase = jh.phase
					AND (later.created_at > jh.created_at OR (later.created_at = jh.created_at AND later.id > jh.id))
			)
			AND NOT EXISTS (
				SELECT 1 FROM job_history active
				WHERE active.scene_id = jh.scene_id AND active.phase = jh.phase AND active.status IN (?, ?)
			)
	`,
		JobStatusPending,
		JobErrorStuckPendingRecovered,
		JobStatusFailed,
		JobErrorInterruptedByShutdown, JobErrorOrphanRecovered, JobErrorStuckPendingRecovered,
		JobErrorStuckPendingRecovered,
		JobStatusPending, JobStatusRunning,
	)
	return result.RowsAffected, result.Error
}

// CancelPendingJobsForScene cancels all pending jobs for a scene (marks them as cancelled).
func (r *JobHistoryRepositoryImpl) CancelPendingJobsForScene(sceneID uint) (int64, error) {
	result := r.DB.Model(&JobHistory{}).
		Where("scene_id = ? AND status = ?", sceneID, JobStatusPending).
		Updates(map[string]any{
			"status":        "cancelled",
			"error_message": "Scene moved to trash",
			"completed_at":  time.Now(),
			"is_retryable":  false,
		})

	return result.RowsAffected, result.Error
}

// CancelPendingJob cancels a single pending job by job ID.
// Returns an error if the job is not found or not in pending state.
func (r *JobHistoryRepositoryImpl) CancelPendingJob(jobID string) error {
	now := time.Now()
	result := r.DB.Model(&JobHistory{}).
		Where("job_id = ? AND status = ?", jobID, JobStatusPending).
		Updates(map[string]any{
			"status":       JobStatusCancelled,
			"completed_at": now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("job not found or not in pending state: %s", jobID)
	}
	return nil
}

// CountRecentFailedByPhase returns the count of failed jobs per phase within a time window.
func (r *JobHistoryRepositoryImpl) CountRecentFailedByPhase(since time.Duration) (map[string]int, error) {
	type phaseCount struct {
		Phase string
		Count int
	}

	cutoff := time.Now().Add(-since)
	var counts []phaseCount
	if err := r.DB.Model(&JobHistory{}).
		Select("phase, COUNT(*) as count").
		Where("status = ? AND completed_at >= ?", JobStatusFailed, cutoff).
		Group("phase").
		Scan(&counts).Error; err != nil {
		return nil, err
	}

	result := make(map[string]int)
	for _, c := range counts {
		result[c.Phase] = c.Count
	}

	return result, nil
}

// GetFailedJobs returns all jobs with status 'failed'.
func (r *JobHistoryRepositoryImpl) GetFailedJobs() ([]JobHistory, error) {
	var jobs []JobHistory
	if err := r.DB.Where("status = ?", JobStatusFailed).
		Order("completed_at desc").
		Find(&jobs).Error; err != nil {
		return nil, err
	}
	return jobs, nil
}

// DeleteByStatus deletes all jobs with the given status and returns the number of rows affected.
func (r *JobHistoryRepositoryImpl) DeleteByStatus(status string) (int64, error) {
	result := r.DB.Where("status = ?", status).Delete(&JobHistory{})
	return result.RowsAffected, result.Error
}

// PurgeableStatuses are the finished job statuses the history purge may delete. Pending and
// running jobs are the job queue itself and are never purged.
var PurgeableStatuses = map[string]bool{
	JobStatusCompleted: true,
	JobStatusCancelled: true,
	JobStatusFailed:    true,
	JobStatusTimedOut:  true,
}

// purgeableScope restricts a query to finished jobs of the given statuses started before a date.
// Failed jobs with a retry still scheduled are kept: the retry scheduler reads them.
func purgeableScope(db *gorm.DB, statuses []string, before time.Time) (*gorm.DB, error) {
	if len(statuses) == 0 {
		return nil, fmt.Errorf("no status to purge")
	}
	for _, st := range statuses {
		if !PurgeableStatuses[st] {
			return nil, fmt.Errorf("status %q cannot be purged", st)
		}
	}
	return db.Model(&JobHistory{}).
		Where("status IN ? AND started_at < ?", statuses, before).
		Where("NOT (status = ? AND is_retryable AND next_retry_at IS NOT NULL)", JobStatusFailed), nil
}

// PurgeFinishedBatch deletes up to limit finished jobs of the given statuses started before the
// given date, in one statement, and returns the number of deleted rows. Call it in a loop until
// it returns less than limit: small batches keep the locks short on a large table.
func (r *JobHistoryRepositoryImpl) PurgeFinishedBatch(statuses []string, before time.Time, limit int) (int64, error) {
	if limit <= 0 {
		return 0, fmt.Errorf("purge batch size must be positive")
	}
	scope, err := purgeableScope(r.DB, statuses, before)
	if err != nil {
		return 0, err
	}
	ids := scope.Select("id").Limit(limit)
	result := r.DB.Where("id IN (?)", ids).Delete(&JobHistory{})
	return result.RowsAffected, result.Error
}

// CountPurgeable counts per status the finished jobs PurgeFinishedBatch would delete.
func (r *JobHistoryRepositoryImpl) CountPurgeable(statuses []string, before time.Time) (map[string]int64, error) {
	scope, err := purgeableScope(r.DB, statuses, before)
	if err != nil {
		return nil, err
	}
	var counts []struct {
		Status string
		Count  int64
	}
	if err := scope.Select("status, COUNT(*) AS count").Group("status").Scan(&counts).Error; err != nil {
		return nil, err
	}
	result := make(map[string]int64, len(counts))
	for _, c := range counts {
		result[c.Status] = c.Count
	}
	return result, nil
}

// CancelPendingJobsByPhases cancels every pending job of the given phases in a single UPDATE and
// returns the number of cancelled jobs. Rows being claimed by the feeder at the same time are
// re-checked by PostgreSQL after their lock is released: they are running by then and skipped.
func (r *JobHistoryRepositoryImpl) CancelPendingJobsByPhases(phases []string, reason string) (int64, error) {
	if len(phases) == 0 {
		return 0, nil
	}
	result := r.DB.Model(&JobHistory{}).
		Where("status = ? AND phase IN ?", JobStatusPending, phases).
		Updates(map[string]any{
			"status":        JobStatusCancelled,
			"error_message": reason,
			"completed_at":  time.Now(),
			"is_retryable":  false,
		})
	return result.RowsAffected, result.Error
}

// cancelClaimedChunk bounds the number of IDs per UPDATE statement.
const cancelClaimedChunk = 1000

// CancelClaimedJobs cancels jobs claimed by the feeder but never started (their row is
// 'running' while they wait in a worker pool queue). Rows in any other state are left alone.
func (r *JobHistoryRepositoryImpl) CancelClaimedJobs(jobIDs []string, reason string) (int64, error) {
	var total int64
	now := time.Now()
	for start := 0; start < len(jobIDs); start += cancelClaimedChunk {
		end := min(start+cancelClaimedChunk, len(jobIDs))
		result := r.DB.Model(&JobHistory{}).
			Where("job_id IN ? AND status = ? AND worker_id IS NULL", jobIDs[start:end], JobStatusRunning).
			Updates(map[string]any{
				"status":        JobStatusCancelled,
				"error_message": reason,
				"completed_at":  now,
				"is_retryable":  false,
			})
		if result.Error != nil {
			return total, result.Error
		}
		total += result.RowsAffected
	}
	return total, nil
}
