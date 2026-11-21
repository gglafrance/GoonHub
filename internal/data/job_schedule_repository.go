package data

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// JobScheduleConfigRecord holds the global job scheduling settings (singleton row, id = 1).
type JobScheduleConfigRecord struct {
	ID                   int       `gorm:"primaryKey" json:"id"`
	Timezone             string    `gorm:"column:timezone" json:"timezone"`
	QuietHoursStart      string    `gorm:"column:quiet_hours_start" json:"quiet_hours_start"`
	QuietHoursEnd        string    `gorm:"column:quiet_hours_end" json:"quiet_hours_end"`
	MaxConcurrentJobs    int       `gorm:"column:max_concurrent_jobs" json:"max_concurrent_jobs"`
	NewScenesFirst       bool      `gorm:"column:new_scenes_first" json:"new_scenes_first"`
	NewScenesWindowHours int       `gorm:"column:new_scenes_window_hours" json:"new_scenes_window_hours"`
	UpdatedAt            time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (JobScheduleConfigRecord) TableName() string {
	return "job_schedule_config"
}

// JobPhaseScheduleRecord holds the scheduling settings of one processing phase.
type JobPhaseScheduleRecord struct {
	ID            int       `gorm:"primaryKey" json:"id"`
	Phase         string    `gorm:"column:phase;uniqueIndex" json:"phase"`
	Priority      int       `gorm:"column:priority" json:"priority"`
	WindowEnabled bool      `gorm:"column:window_enabled" json:"window_enabled"`
	WindowStart   string    `gorm:"column:window_start" json:"window_start"`
	WindowEnd     string    `gorm:"column:window_end" json:"window_end"`
	QuietWorkers  int       `gorm:"column:quiet_workers" json:"quiet_workers"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (JobPhaseScheduleRecord) TableName() string {
	return "job_phase_schedules"
}

// JobScheduleRepository stores the job scheduling settings.
type JobScheduleRepository interface {
	// GetConfig returns the global settings, or nil when the row does not exist.
	GetConfig() (*JobScheduleConfigRecord, error)
	// ListPhases returns the per-phase settings.
	ListPhases() ([]JobPhaseScheduleRecord, error)
	// Save writes the global and per-phase settings in one transaction.
	Save(cfg *JobScheduleConfigRecord, phases []JobPhaseScheduleRecord) error
}

type JobScheduleRepositoryImpl struct {
	DB *gorm.DB
}

func NewJobScheduleRepository(db *gorm.DB) *JobScheduleRepositoryImpl {
	return &JobScheduleRepositoryImpl{DB: db}
}

func (r *JobScheduleRepositoryImpl) GetConfig() (*JobScheduleConfigRecord, error) {
	var record JobScheduleConfigRecord
	if err := r.DB.First(&record, 1).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

func (r *JobScheduleRepositoryImpl) ListPhases() ([]JobPhaseScheduleRecord, error) {
	var records []JobPhaseScheduleRecord
	if err := r.DB.Order("id ASC").Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

func (r *JobScheduleRepositoryImpl) Save(cfg *JobScheduleConfigRecord, phases []JobPhaseScheduleRecord) error {
	now := time.Now()
	return r.DB.Transaction(func(tx *gorm.DB) error {
		cfg.ID = 1
		cfg.UpdatedAt = now
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"timezone", "quiet_hours_start", "quiet_hours_end", "max_concurrent_jobs",
				"new_scenes_first", "new_scenes_window_hours", "updated_at",
			}),
		}).Create(cfg).Error; err != nil {
			return err
		}

		for i := range phases {
			p := &phases[i]
			p.ID = 0
			p.UpdatedAt = now
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "phase"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"priority", "window_enabled", "window_start", "window_end", "quiet_workers", "updated_at",
				}),
			}).Create(p).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
