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
