package data

import (
	"time"

	"gorm.io/gorm"
)

type DLQRepository interface {
	Create(entry *DLQEntry) error
	GetByJobID(jobID string) (*DLQEntry, error)
	ListPending(page, limit int) ([]DLQEntry, int64, error)
	ListByStatus(status string, page, limit int) ([]DLQEntry, int64, error)
	UpdateStatus(jobID string, status string) error
	MarkAbandoned(jobID string) error
	Delete(jobID string) error
	CountByStatus(status string) (int64, error)
	AutoAbandon(olderThan time.Duration) (int64, error)
}

type DLQRepositoryImpl struct {
	DB *gorm.DB
}

func NewDLQRepository(db *gorm.DB) *DLQRepositoryImpl {
	return &DLQRepositoryImpl{DB: db}
}

func (r *DLQRepositoryImpl) Create(entry *DLQEntry) error {
	return r.DB.Create(entry).Error
}

func (r *DLQRepositoryImpl) GetByJobID(jobID string) (*DLQEntry, error) {
	var entry DLQEntry
	if err := r.DB.Where("job_id = ?", jobID).First(&entry).Error; err != nil {
		return nil, err
	}
	return &entry, nil
}

func (r *DLQRepositoryImpl) ListPending(page, limit int) ([]DLQEntry, int64, error) {
	return r.ListByStatus("pending_review", page, limit)
}

func (r *DLQRepositoryImpl) ListByStatus(status string, page, limit int) ([]DLQEntry, int64, error) {
	var entries []DLQEntry
	var total int64

	offset := (page - 1) * limit

	query := r.DB.Model(&DLQEntry{})
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Limit(limit).Offset(offset).Order("created_at desc").Find(&entries).Error; err != nil {
		return nil, 0, err
	}

	return entries, total, nil
}

func (r *DLQRepositoryImpl) UpdateStatus(jobID string, status string) error {
	return r.DB.Model(&DLQEntry{}).Where("job_id = ?", jobID).Updates(map[string]any{
		"status":     status,
		"updated_at": time.Now(),
	}).Error
}

func (r *DLQRepositoryImpl) MarkAbandoned(jobID string) error {
	now := time.Now()
	return r.DB.Model(&DLQEntry{}).Where("job_id = ?", jobID).Updates(map[string]any{
		"status":       "abandoned",
		"abandoned_at": now,
		"updated_at":   now,
	}).Error
}

func (r *DLQRepositoryImpl) Delete(jobID string) error {
	return r.DB.Where("job_id = ?", jobID).Delete(&DLQEntry{}).Error
}

func (r *DLQRepositoryImpl) CountByStatus(status string) (int64, error) {
	var count int64
	query := r.DB.Model(&DLQEntry{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *DLQRepositoryImpl) AutoAbandon(olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	now := time.Now()
	result := r.DB.Model(&DLQEntry{}).
		Where("status = ? AND created_at < ?", "pending_review", cutoff).
		Updates(map[string]any{
			"status":       "abandoned",
			"abandoned_at": now,
			"updated_at":   now,
		})
	return result.RowsAffected, result.Error
}
