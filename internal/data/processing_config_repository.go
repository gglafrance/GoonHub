package data

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ProcessingConfigRecord struct {
	ID                  int       `gorm:"primaryKey" json:"id"`
	MaxFrameDimensionSm int       `gorm:"column:max_frame_dimension_sm" json:"max_frame_dimension_sm"`
	MaxFrameDimensionLg int       `gorm:"column:max_frame_dimension_lg" json:"max_frame_dimension_lg"`
	FrameQualitySm      int       `gorm:"column:frame_quality_sm" json:"frame_quality_sm"`
	FrameQualityLg      int       `gorm:"column:frame_quality_lg" json:"frame_quality_lg"`
	FrameQualitySprites int       `gorm:"column:frame_quality_sprites" json:"frame_quality_sprites"`
	SpritesConcurrency  int       `gorm:"column:sprites_concurrency" json:"sprites_concurrency"`
	UpdatedAt           time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (ProcessingConfigRecord) TableName() string {
	return "processing_config"
}

type ProcessingConfigRepository interface {
	Get() (*ProcessingConfigRecord, error)
	Upsert(record *ProcessingConfigRecord) error
}

type ProcessingConfigRepositoryImpl struct {
	DB *gorm.DB
}

func NewProcessingConfigRepository(db *gorm.DB) *ProcessingConfigRepositoryImpl {
	return &ProcessingConfigRepositoryImpl{DB: db}
}

func (r *ProcessingConfigRepositoryImpl) Get() (*ProcessingConfigRecord, error) {
	var record ProcessingConfigRecord
	err := r.DB.First(&record).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

func (r *ProcessingConfigRepositoryImpl) Upsert(record *ProcessingConfigRecord) error {
	record.ID = 1
	record.UpdatedAt = time.Now()
	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"max_frame_dimension_sm", "max_frame_dimension_lg", "frame_quality_sm", "frame_quality_lg", "frame_quality_sprites", "sprites_concurrency", "updated_at"}),
	}).Create(record).Error
}
