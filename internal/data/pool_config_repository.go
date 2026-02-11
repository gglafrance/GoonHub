package data

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PoolConfigRecord struct {
	ID                        int       `gorm:"primaryKey" json:"id"`
	MetadataWorkers           int       `gorm:"column:metadata_workers" json:"metadata_workers"`
	ThumbnailWorkers          int       `gorm:"column:thumbnail_workers" json:"thumbnail_workers"`
	SpritesWorkers            int       `gorm:"column:sprites_workers" json:"sprites_workers"`
	AnimatedThumbnailsWorkers int       `gorm:"column:animated_thumbnails_workers" json:"animated_thumbnails_workers"`
	UpdatedAt                 time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (PoolConfigRecord) TableName() string {
	return "pool_config"
}

type PoolConfigRepository interface {
	Get() (*PoolConfigRecord, error)
	Upsert(record *PoolConfigRecord) error
}

type PoolConfigRepositoryImpl struct {
	DB *gorm.DB
}

func NewPoolConfigRepository(db *gorm.DB) *PoolConfigRepositoryImpl {
	return &PoolConfigRepositoryImpl{DB: db}
}

func (r *PoolConfigRepositoryImpl) Get() (*PoolConfigRecord, error) {
	var record PoolConfigRecord
	err := r.DB.First(&record).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

func (r *PoolConfigRepositoryImpl) Upsert(record *PoolConfigRecord) error {
	record.ID = 1
	record.UpdatedAt = time.Now()
	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"metadata_workers", "thumbnail_workers", "sprites_workers", "animated_thumbnails_workers", "updated_at"}),
	}).Create(record).Error
}
