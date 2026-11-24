package data

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// WorkerRepository stores the remote workers and the singleton worker settings.
type WorkerRepository interface {
	Create(w *Worker) error
	GetByID(id uint) (*Worker, error)
	List() ([]Worker, error)
	// Update sets the given columns of a worker (updated_at is set too).
	Update(id uint, fields map[string]any) error
	Delete(id uint) error
	// IncrementCounter adds n to one of the WorkerCounter* columns.
	IncrementCounter(id uint, counter string, n int) error
	// RecordError stores the last error reported for a worker.
	RecordError(id uint, message string, at time.Time) error

	GetSettings() (*WorkerSettings, error)
	// EnsureMediaSecret stores secret as the media URL secret unless one is already stored,
	// and returns the stored one (the first start generates it, later starts reuse it).
	EnsureMediaSecret(secret string) (string, error)
	UpdatePhaseRouting(routing []byte) error
}

// WorkerRepositoryImpl is the GORM implementation of WorkerRepository.
type WorkerRepositoryImpl struct {
	DB *gorm.DB
}

// NewWorkerRepository creates a WorkerRepository.
func NewWorkerRepository(db *gorm.DB) *WorkerRepositoryImpl {
	return &WorkerRepositoryImpl{DB: db}
}

func (r *WorkerRepositoryImpl) Create(w *Worker) error {
	return r.DB.Create(w).Error
}

func (r *WorkerRepositoryImpl) GetByID(id uint) (*Worker, error) {
	var w Worker
	if err := r.DB.First(&w, id).Error; err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *WorkerRepositoryImpl) List() ([]Worker, error) {
	var workers []Worker
	if err := r.DB.Order("id ASC").Find(&workers).Error; err != nil {
		return nil, err
	}
	return workers, nil
}

func (r *WorkerRepositoryImpl) Update(id uint, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	updates := make(map[string]any, len(fields)+1)
	for k, v := range fields {
		updates[k] = v
	}
	updates["updated_at"] = time.Now()
	result := r.DB.Model(&Worker{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *WorkerRepositoryImpl) Delete(id uint) error {
	result := r.DB.Delete(&Worker{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

var workerCounters = map[string]bool{
	WorkerCounterCompleted: true,
	WorkerCounterFailed:    true,
	WorkerCounterExpired:   true,
	WorkerCounterReleased:  true,
}

func (r *WorkerRepositoryImpl) IncrementCounter(id uint, counter string, n int) error {
	if !workerCounters[counter] {
		return fmt.Errorf("unknown worker counter %q", counter)
	}
	if n == 0 {
		return nil
	}
	return r.DB.Model(&Worker{}).Where("id = ?", id).
		UpdateColumn(counter, gorm.Expr(counter+" + ?", n)).Error
}

func (r *WorkerRepositoryImpl) RecordError(id uint, message string, at time.Time) error {
	return r.DB.Model(&Worker{}).Where("id = ?", id).
		UpdateColumns(map[string]any{"last_error": message, "last_error_at": at}).Error
}

func (r *WorkerRepositoryImpl) GetSettings() (*WorkerSettings, error) {
	var s WorkerSettings
	if err := r.DB.Where("id = 1").Attrs(WorkerSettings{ID: 1}).FirstOrCreate(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *WorkerRepositoryImpl) EnsureMediaSecret(secret string) (string, error) {
	if _, err := r.GetSettings(); err != nil {
		return "", err
	}
	if err := r.DB.Model(&WorkerSettings{}).
		Where("id = 1 AND media_secret = ''").
		Updates(map[string]any{"media_secret": secret, "updated_at": time.Now()}).Error; err != nil {
		return "", err
	}
	s, err := r.GetSettings()
	if err != nil {
		return "", err
	}
	return s.MediaSecret, nil
}

func (r *WorkerRepositoryImpl) UpdatePhaseRouting(routing []byte) error {
	if _, err := r.GetSettings(); err != nil {
		return err
	}
	return r.DB.Model(&WorkerSettings{}).Where("id = 1").
		Updates(map[string]any{"phase_routing": routing, "updated_at": time.Now()}).Error
}
