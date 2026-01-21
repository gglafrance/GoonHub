package data

import "gorm.io/gorm"

type VideoRepository interface {
	Create(video *Video) error
	List(page, limit int) ([]Video, int64, error)
	GetByID(id uint) (*Video, error)
	UpdateMetadata(id uint, duration int, width, height int, thumbnailPath string, framePaths string, frameCount int, frameInterval int) error
	UpdateProcessingStatus(id uint, status string, errorMsg string) error
	GetPendingProcessing() ([]Video, error)
	Delete(id uint) error
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

func (r *SQLiteVideoRepository) UpdateMetadata(id uint, duration int, width, height int, thumbnailPath string, framePaths string, frameCount int, frameInterval int) error {
	updates := map[string]interface{}{
		"duration":          duration,
		"width":             width,
		"height":            height,
		"thumbnail_path":    thumbnailPath,
		"frame_paths":       framePaths,
		"frame_count":       frameCount,
		"frame_interval":    frameInterval,
		"processing_status": "completed",
	}
	return r.DB.Model(&Video{}).Where("id = ?", id).Updates(updates).Error
}

func (r *SQLiteVideoRepository) UpdateProcessingStatus(id uint, status string, errorMsg string) error {
	updates := map[string]interface{}{
		"processing_status": status,
	}
	if errorMsg != "" {
		updates["processing_error"] = errorMsg
	}
	return r.DB.Model(&Video{}).Where("id = ?", id).Updates(updates).Error
}

func (r *SQLiteVideoRepository) GetPendingProcessing() ([]Video, error) {
	var videos []Video
	if err := r.DB.Where("processing_status = ?", "pending").Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

func (r *SQLiteVideoRepository) Delete(id uint) error {
	var video Video
	if err := r.DB.First(&video, id).Error; err != nil {
		return err
	}
	return r.DB.Delete(&video).Error
}
