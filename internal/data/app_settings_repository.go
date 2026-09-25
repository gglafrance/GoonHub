package data

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AppSettingsRecord struct {
	ID                 int     `gorm:"primaryKey" json:"id"`
	TrashRetentionDays int     `gorm:"column:trash_retention_days" json:"trash_retention_days"`
	ServeOGMetadata    bool    `gorm:"column:serve_og_metadata" json:"serve_og_metadata"`
	ShortsMaxDuration  int     `gorm:"column:shorts_max_duration;default:60" json:"shorts_max_duration"`
	ShortsSaveDir      *string `gorm:"column:shorts_save_dir" json:"shorts_save_dir"`
	// ShortsSearchDefault is what the search page's Shorts filter starts on
	ShortsSearchDefault string    `gorm:"column:shorts_search_default;default:hide" json:"shorts_search_default"`
	UpdatedAt           time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (AppSettingsRecord) TableName() string {
	return "app_settings"
}

type AppSettingsRepository interface {
	Get() (*AppSettingsRecord, error)
	Upsert(record *AppSettingsRecord) error
	UpdateShortsSettings(maxDuration int, saveDir *string, searchDefault string) error
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
				ID:                  1,
				TrashRetentionDays:  7,
				ServeOGMetadata:     true,
				ShortsMaxDuration:   60,
				ShortsSearchDefault: "hide",
				UpdatedAt:           time.Now(),
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

// UpdateShortsSettings writes only the shorts columns. Upsert leaves them out on
// purpose: the app settings PUT binds the whole record, and a client that only
// sends its own fields must not reset these.
func (r *AppSettingsRepositoryImpl) UpdateShortsSettings(maxDuration int, saveDir *string, searchDefault string) error {
	record := &AppSettingsRecord{
		ID:                  1,
		TrashRetentionDays:  7,
		ServeOGMetadata:     true,
		ShortsMaxDuration:   maxDuration,
		ShortsSaveDir:       saveDir,
		ShortsSearchDefault: searchDefault,
		UpdatedAt:           time.Now(),
	}
	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"shorts_max_duration", "shorts_save_dir", "shorts_search_default", "updated_at"}),
	}).Create(record).Error
}
