package data

import (
	"time"

	"gorm.io/gorm"
)

type JobHistoryRepository interface {
	Create(record *JobHistory) error
	UpdateStatus(jobID string, status string, errorMessage *string, completedAt *time.Time) error
	ListAll(page, limit int) ([]JobHistory, int64, error)
	ListActive() ([]JobHistory, error)
	DeleteOlderThan(before time.Time) (int64, error)
	UpdateProgress(jobID string, progress int) error
	UpdateRetryInfo(jobID string, retryCount, maxRetries int, nextRetryAt *time.Time) error
	GetRetryableJobs() ([]JobHistory, error)
	MarkNotRetryable(jobID string) error
	GetByJobID(jobID string) (*JobHistory, error)
	IncrementRetryCount(jobID string) error

	// DB-backed job queue methods
	CreatePending(record *JobHistory) error
	CreateBatch(records []*JobHistory) error
	ClaimPendingJobs(phase string, limit int) ([]JobHistory, error)
	CountPendingByPhase() (map[string]int, error)
	ExistsPendingOrRunning(sceneID uint, phase string) (bool, error)
	MarkOrphanedRunningAsFailed(olderThan time.Duration) (int64, error)
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

func (r *JobHistoryRepositoryImpl) ListAll(page, limit int) ([]JobHistory, int64, error) {
	var records []JobHistory
	var total int64

	offset := (page - 1) * limit

	if err := r.DB.Model(&JobHistory{}).Where("status != ?", "running").Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := r.DB.Where("status != ?", "running").Limit(limit).Offset(offset).Order("started_at desc").Find(&records).Error; err != nil {
		return nil, 0, err
	}

	return records, total, nil
}

func (r *JobHistoryRepositoryImpl) ListActive() ([]JobHistory, error) {
	var records []JobHistory
	if err := r.DB.Where("status = ?", "running").Order("started_at desc").Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

func (r *JobHistoryRepositoryImpl) DeleteOlderThan(before time.Time) (int64, error) {
	result := r.DB.Where("started_at < ? AND status != ?", before, "running").Delete(&JobHistory{})
	return result.RowsAffected, result.Error
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

// ClaimPendingJobs atomically claims up to 'limit' pending jobs for a phase.
// Uses FOR UPDATE SKIP LOCKED, sets status='running' and StartedAt.
func (r *JobHistoryRepositoryImpl) ClaimPendingJobs(phase string, limit int) ([]JobHistory, error) {
	var jobs []JobHistory

	err := r.DB.Transaction(func(tx *gorm.DB) error {
		// Select pending jobs with lock, skipping already locked rows
		if err := tx.Raw(`
			SELECT * FROM job_history
			WHERE phase = ? AND status = 'pending'
			ORDER BY priority DESC, created_at ASC
			LIMIT ?
			FOR UPDATE SKIP LOCKED
		`, phase, limit).Scan(&jobs).Error; err != nil {
			return err
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
	errMsg := "Orphaned job recovered after server restart"

	result := r.DB.Model(&JobHistory{}).
		Where("status = ? AND started_at < ?", JobStatusRunning, cutoff).
		Updates(map[string]any{
			"status":        JobStatusFailed,
			"error_message": errMsg,
			"completed_at":  time.Now(),
			"is_retryable":  true,
		})

	return result.RowsAffected, result.Error
}
