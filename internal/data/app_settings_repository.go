package data

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AppSettingsRecord struct {
	ID                 int       `gorm:"primaryKey" json:"id"`
	TrashRetentionDays int       `gorm:"column:trash_retention_days" json:"trash_retention_days"`
	ServeOGMetadata    bool      `gorm:"column:serve_og_metadata" json:"serve_og_metadata"`
	UpdatedAt          time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (AppSettingsRecord) TableName() string {
	return "app_settings"
}

type AppSettingsRepository interface {
	Get() (*AppSettingsRecord, error)
	Upsert(record *AppSettingsRecord) error
}

type AppSettingsRepositoryImpl struct {
	DB *gorm.DB
}

func NewAppSettingsRepository(db *gorm.DB) *AppSettingsRepositoryImpl {
	return &AppSettingsRepositoryImpl{DB: db}
}

func (r *AppSettingsRepositoryImpl) Get() (*AppSettingsRecord, error) {
	var record AppSettingsRecord
	err := r.DB.First(&record).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Return default values if no record exists
			return &AppSettingsRecord{
				ID:                 1,
				TrashRetentionDays: 7,
				ServeOGMetadata:    true,
				UpdatedAt:          time.Now(),
			}, nil
		}
		return nil, err
	}
	return &record, nil
}

func (r *AppSettingsRepositoryImpl) Upsert(record *AppSettingsRecord) error {
	record.ID = 1
	record.UpdatedAt = time.Now()
	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"trash_retention_days", "serve_og_metadata", "updated_at"}),
	}).Create(record).Error
}
