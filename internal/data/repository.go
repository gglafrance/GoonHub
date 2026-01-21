package data

import "gorm.io/gorm"

type VideoRepository interface {
	Create(video *Video) error
	List(page, limit int) ([]Video, int64, error)
	GetByID(id uint) (*Video, error)
}

type SQLiteVideoRepository struct {
	DB *gorm.DB
}

func NewSQLiteVideoRepository(db *gorm.DB) *SQLiteVideoRepository {
	return &SQLiteVideoRepository{DB: db}
}

func (r *SQLiteVideoRepository) Create(video *Video) error {
	return r.DB.Create(video).Error
}

func (r *SQLiteVideoRepository) List(page, limit int) ([]Video, int64, error) {
	var videos []Video
	var total int64

	offset := (page - 1) * limit

	if err := r.DB.Model(&Video{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := r.DB.Limit(limit).Offset(offset).Order("created_at desc").Find(&videos).Error; err != nil {
		return nil, 0, err
	}

	return videos, total, nil
}

func (r *SQLiteVideoRepository) GetByID(id uint) (*Video, error) {
	var video Video
	if err := r.DB.First(&video, id).Error; err != nil {
		return nil, err
	}
	return &video, nil
}
