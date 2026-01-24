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

	if err := r.DB.Model(&JobHistory{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := r.DB.Limit(limit).Offset(offset).Order("started_at desc").Find(&records).Error; err != nil {
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
