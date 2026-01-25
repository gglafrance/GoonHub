package data

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RetryConfigRepository interface {
	GetAll() ([]RetryConfigRecord, error)
	GetByPhase(phase string) (*RetryConfigRecord, error)
	Upsert(record *RetryConfigRecord) error
}

type RetryConfigRepositoryImpl struct {
	DB *gorm.DB
}

func NewRetryConfigRepository(db *gorm.DB) *RetryConfigRepositoryImpl {
	return &RetryConfigRepositoryImpl{DB: db}
}

func (r *RetryConfigRepositoryImpl) GetAll() ([]RetryConfigRecord, error) {
	var records []RetryConfigRecord
	if err := r.DB.Order("phase").Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

func (r *RetryConfigRepositoryImpl) GetByPhase(phase string) (*RetryConfigRecord, error) {
	var record RetryConfigRecord
	if err := r.DB.Where("phase = ?", phase).First(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *RetryConfigRepositoryImpl) Upsert(record *RetryConfigRecord) error {
	record.UpdatedAt = time.Now()
	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "phase"}},
		DoUpdates: clause.AssignmentColumns([]string{"max_retries", "initial_delay_seconds", "max_delay_seconds", "backoff_factor", "updated_at"}),
	}).Create(record).Error
}
