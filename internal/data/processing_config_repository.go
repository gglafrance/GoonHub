package data

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ProcessingConfigRecord struct {
	ID                     int       `gorm:"primaryKey" json:"id"`
	MaxFrameDimensionSm    int       `gorm:"column:max_frame_dimension_sm" json:"max_frame_dimension_sm"`
	MaxFrameDimensionLg    int       `gorm:"column:max_frame_dimension_lg" json:"max_frame_dimension_lg"`
	FrameQualitySm         int       `gorm:"column:frame_quality_sm" json:"frame_quality_sm"`
	FrameQualityLg         int       `gorm:"column:frame_quality_lg" json:"frame_quality_lg"`
	FrameQualitySprites    int       `gorm:"column:frame_quality_sprites" json:"frame_quality_sprites"`
	SpritesConcurrency     int       `gorm:"column:sprites_concurrency" json:"sprites_concurrency"`
	MarkerThumbnailType    string    `gorm:"column:marker_thumbnail_type" json:"marker_thumbnail_type"`
	MarkerAnimatedDuration     int       `gorm:"column:marker_animated_duration" json:"marker_animated_duration"`
	ScenePreviewEnabled        bool      `gorm:"column:scene_preview_enabled" json:"scene_preview_enabled"`
	ScenePreviewSegments       int       `gorm:"column:scene_preview_segments" json:"scene_preview_segments"`
	ScenePreviewSegmentDuration float64  `gorm:"column:scene_preview_segment_duration" json:"scene_preview_segment_duration"`
	MarkerPreviewCRF           int       `gorm:"column:marker_preview_crf" json:"marker_preview_crf"`
	ScenePreviewCRF            int       `gorm:"column:scene_preview_crf" json:"scene_preview_crf"`
	UpdatedAt                  time.Time `gorm:"column:updated_at" json:"updated_at"`
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
		DoUpdates: clause.AssignmentColumns([]string{"max_frame_dimension_sm", "max_frame_dimension_lg", "frame_quality_sm", "frame_quality_lg", "frame_quality_sprites", "sprites_concurrency", "marker_thumbnail_type", "marker_animated_duration", "scene_preview_enabled", "scene_preview_segments", "scene_preview_segment_duration", "marker_preview_crf", "scene_preview_crf", "updated_at"}),
	}).Create(record).Error
}
