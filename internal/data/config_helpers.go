package data

import (
	"gorm.io/gorm"
)

// SingletonConfigRepository provides common methods for singleton config repositories.
// Singleton configs have a single row with ID=1.
// This helper reduces boilerplate for Get operations.
type SingletonConfigRepository[T any] struct {
	DB *gorm.DB
}

// Get retrieves the singleton config record, returning nil if not found.
func (r *SingletonConfigRepository[T]) Get() (*T, error) {
	var record T
	err := r.DB.First(&record).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

// PhaseConfigRepository provides common methods for phase-keyed config repositories.
// Phase configs have multiple rows keyed by the "phase" column.
// This helper reduces boilerplate for GetAll and GetByPhase operations.
type PhaseConfigRepository[T any] struct {
	DB *gorm.DB
}

// GetAll retrieves all phase config records.
func (r *PhaseConfigRepository[T]) GetAll() ([]T, error) {
	var records []T
	err := r.DB.Order("phase").Find(&records).Error
	if err != nil {
		return nil, err
	}
	return records, nil
}

// GetByPhase retrieves a config record by phase, returning nil if not found.
func (r *PhaseConfigRepository[T]) GetByPhase(phase string) (*T, error) {
	var record T
	err := r.DB.Where("phase = ?", phase).First(&record).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}
