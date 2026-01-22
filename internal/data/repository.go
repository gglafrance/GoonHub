package data

import (
	"time"

	"gorm.io/gorm"
)

type UserRepository interface {
	Create(user *User) error
	GetByUsername(username string) (*User, error)
	GetByID(id uint) (*User, error)
	Exists(username string) (bool, error)
}

type RevokedTokenRepository interface {
	Create(token *RevokedToken) error
	IsRevoked(tokenHash string) (bool, error)
	CleanupExpired() error
}

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

type SQLiteUserRepository struct {
	DB *gorm.DB
}

func NewSQLiteUserRepository(db *gorm.DB) *SQLiteUserRepository {
	return &SQLiteUserRepository{DB: db}
}

func (r *SQLiteUserRepository) Create(user *User) error {
	return r.DB.Create(user).Error
}

func (r *SQLiteUserRepository) GetByUsername(username string) (*User, error) {
	var user User
	if err := r.DB.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *SQLiteUserRepository) GetByID(id uint) (*User, error) {
	var user User
	if err := r.DB.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *SQLiteUserRepository) Exists(username string) (bool, error) {
	var count int64
	if err := r.DB.Model(&User{}).Where("username = ?", username).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

type SQLiteRevokedTokenRepository struct {
	DB *gorm.DB
}

func NewSQLiteRevokedTokenRepository(db *gorm.DB) *SQLiteRevokedTokenRepository {
	return &SQLiteRevokedTokenRepository{DB: db}
}

func (r *SQLiteRevokedTokenRepository) Create(token *RevokedToken) error {
	return r.DB.Create(token).Error
}

func (r *SQLiteRevokedTokenRepository) IsRevoked(tokenHash string) (bool, error) {
	var count int64
	if err := r.DB.Model(&RevokedToken{}).Where("token_hash = ? AND expires_at > ?", tokenHash, time.Now()).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *SQLiteRevokedTokenRepository) CleanupExpired() error {
	return r.DB.Where("expires_at <= ?", time.Now()).Delete(&RevokedToken{}).Error
}
