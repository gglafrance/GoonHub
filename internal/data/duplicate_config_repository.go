package data

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DuplicateConfigRepository handles singleton duplicate detection configuration.
type DuplicateConfigRepository interface {
	Get() (*DuplicateConfigRecord, error)
	Upsert(record *DuplicateConfigRecord) error
}

type DuplicateConfigRepositoryImpl struct {
	DB *gorm.DB
}

func NewDuplicateConfigRepository(db *gorm.DB) *DuplicateConfigRepositoryImpl {
	return &DuplicateConfigRepositoryImpl{DB: db}
}

func (r *DuplicateConfigRepositoryImpl) Get() (*DuplicateConfigRecord, error) {
	var record DuplicateConfigRecord
	err := r.DB.First(&record).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

func (r *DuplicateConfigRepositoryImpl) Upsert(record *DuplicateConfigRecord) error {
	record.ID = 1
	record.UpdatedAt = time.Now()
	return r.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"enabled", "check_on_upload", "match_threshold", "hamming_distance",
			"duplicate_action", "keep_best_rules", "keep_best_enabled", "codec_preference", "sample_interval", "updated_at",
		}),
	}).Create(record).Error
}
