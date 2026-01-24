package data

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TriggerConfigRecord struct {
	ID             int       `gorm:"primaryKey" json:"id"`
	Phase          string    `gorm:"column:phase;uniqueIndex" json:"phase"`
	TriggerType    string    `gorm:"column:trigger_type" json:"trigger_type"`
	AfterPhase     *string   `gorm:"column:after_phase" json:"after_phase"`
	CronExpression *string   `gorm:"column:cron_expression" json:"cron_expression"`
	UpdatedAt      time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (TriggerConfigRecord) TableName() string {
	return "trigger_config"
}

type TriggerConfigRepository interface {
	GetAll() ([]TriggerConfigRecord, error)
	GetByPhase(phase string) (*TriggerConfigRecord, error)
	Upsert(record *TriggerConfigRecord) error
}

type TriggerConfigRepositoryImpl struct {
	DB *gorm.DB
}

func NewTriggerConfigRepository(db *gorm.DB) *TriggerConfigRepositoryImpl {
	return &TriggerConfigRepositoryImpl{DB: db}
}

func (r *TriggerConfigRepositoryImpl) GetAll() ([]TriggerConfigRecord, error) {
	var records []TriggerConfigRecord
	err := r.DB.Order("id ASC").Find(&records).Error
	if err != nil {
		return nil, err
	}
	return records, nil
}

func (r *TriggerConfigRepositoryImpl) GetByPhase(phase string) (*TriggerConfigRecord, error) {
	var record TriggerConfigRecord
	err := r.DB.Where("phase = ?", phase).First(&record).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

func (r *TriggerConfigRepositoryImpl) Upsert(record *TriggerConfigRecord) error {
	record.UpdatedAt = time.Now()
	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "phase"}},
		DoUpdates: clause.AssignmentColumns([]string{"trigger_type", "after_phase", "cron_expression", "updated_at"}),
	}).Create(record).Error
}
