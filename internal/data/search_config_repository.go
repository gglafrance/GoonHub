package data

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SearchConfigRecord struct {
	ID           int       `gorm:"primaryKey" json:"id"`
	MaxTotalHits int64     `gorm:"column:max_total_hits" json:"max_total_hits"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (SearchConfigRecord) TableName() string {
	return "search_config"
}

type SearchConfigRepository interface {
	Get() (*SearchConfigRecord, error)
	Upsert(record *SearchConfigRecord) error
}

type SearchConfigRepositoryImpl struct {
	DB *gorm.DB
}

func NewSearchConfigRepository(db *gorm.DB) *SearchConfigRepositoryImpl {
	return &SearchConfigRepositoryImpl{DB: db}
}

func (r *SearchConfigRepositoryImpl) Get() (*SearchConfigRecord, error) {
	var record SearchConfigRecord
	err := r.DB.First(&record).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

func (r *SearchConfigRepositoryImpl) Upsert(record *SearchConfigRecord) error {
	record.ID = 1
	record.UpdatedAt = time.Now()
	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"max_total_hits", "updated_at"}),
	}).Create(record).Error
}
