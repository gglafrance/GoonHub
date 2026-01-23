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
	UpdatePassword(userID uint, hashedPassword string) error
	UpdateUsername(userID uint, newUsername string) error
}

type UserSettingsRepository interface {
	GetByUserID(userID uint) (*UserSettings, error)
	Upsert(settings *UserSettings) error
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
	UpdateMetadata(id uint, duration int, width, height int, thumbnailPath string, spriteSheetPath string, vttPath string, spriteSheetCount int, thumbnailWidth int, thumbnailHeight int) error
	UpdateProcessingStatus(id uint, status string, errorMsg string) error
	GetPendingProcessing() ([]Video, error)
	Delete(id uint) error
}

type VideoRepositoryImpl struct {
	DB *gorm.DB
}

func NewVideoRepository(db *gorm.DB) *VideoRepositoryImpl {
	return &VideoRepositoryImpl{DB: db}
}

func (r *VideoRepositoryImpl) Create(video *Video) error {
	return r.DB.Create(video).Error
}

func (r *VideoRepositoryImpl) List(page, limit int) ([]Video, int64, error) {
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

func (r *VideoRepositoryImpl) GetByID(id uint) (*Video, error) {
	var video Video
	if err := r.DB.First(&video, id).Error; err != nil {
		return nil, err
	}
	return &video, nil
}

func (r *VideoRepositoryImpl) UpdateMetadata(id uint, duration int, width, height int, thumbnailPath string, spriteSheetPath string, vttPath string, spriteSheetCount int, thumbnailWidth int, thumbnailHeight int) error {
	updates := map[string]interface{}{
		"duration":           duration,
		"width":              width,
		"height":             height,
		"thumbnail_path":     thumbnailPath,
		"sprite_sheet_path":  spriteSheetPath,
		"vtt_path":           vttPath,
		"sprite_sheet_count": spriteSheetCount,
		"thumbnail_width":    thumbnailWidth,
		"thumbnail_height":   thumbnailHeight,
		"processing_status":  "completed",
	}
	return r.DB.Model(&Video{}).Where("id = ?", id).Updates(updates).Error
}

func (r *VideoRepositoryImpl) UpdateProcessingStatus(id uint, status string, errorMsg string) error {
	updates := map[string]interface{}{
		"processing_status": status,
	}
	if errorMsg != "" {
		updates["processing_error"] = errorMsg
	}
	return r.DB.Model(&Video{}).Where("id = ?", id).Updates(updates).Error
}

func (r *VideoRepositoryImpl) GetPendingProcessing() ([]Video, error) {
	var videos []Video
	if err := r.DB.Where("processing_status = ?", "pending").Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

func (r *VideoRepositoryImpl) Delete(id uint) error {
	var video Video
	if err := r.DB.First(&video, id).Error; err != nil {
		return err
	}
	return r.DB.Delete(&video).Error
}

type UserRepositoryImpl struct {
	DB *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepositoryImpl {
	return &UserRepositoryImpl{DB: db}
}

func (r *UserRepositoryImpl) Create(user *User) error {
	return r.DB.Create(user).Error
}

func (r *UserRepositoryImpl) GetByUsername(username string) (*User, error) {
	var user User
	if err := r.DB.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepositoryImpl) GetByID(id uint) (*User, error) {
	var user User
	if err := r.DB.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepositoryImpl) Exists(username string) (bool, error) {
	var count int64
	if err := r.DB.Model(&User{}).Where("username = ?", username).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *UserRepositoryImpl) UpdatePassword(userID uint, hashedPassword string) error {
	return r.DB.Model(&User{}).Where("id = ?", userID).Update("password", hashedPassword).Error
}

func (r *UserRepositoryImpl) UpdateUsername(userID uint, newUsername string) error {
	return r.DB.Model(&User{}).Where("id = ?", userID).Update("username", newUsername).Error
}

type UserSettingsRepositoryImpl struct {
	DB *gorm.DB
}

func NewUserSettingsRepository(db *gorm.DB) *UserSettingsRepositoryImpl {
	return &UserSettingsRepositoryImpl{DB: db}
}

func (r *UserSettingsRepositoryImpl) GetByUserID(userID uint) (*UserSettings, error) {
	var settings UserSettings
	if err := r.DB.Where("user_id = ?", userID).First(&settings).Error; err != nil {
		return nil, err
	}
	return &settings, nil
}

func (r *UserSettingsRepositoryImpl) Upsert(settings *UserSettings) error {
	return r.DB.Save(settings).Error
}

type RevokedTokenRepositoryImpl struct {
	DB *gorm.DB
}

func NewRevokedTokenRepository(db *gorm.DB) *RevokedTokenRepositoryImpl {
	return &RevokedTokenRepositoryImpl{DB: db}
}

func (r *RevokedTokenRepositoryImpl) Create(token *RevokedToken) error {
	return r.DB.Create(token).Error
}

func (r *RevokedTokenRepositoryImpl) IsRevoked(tokenHash string) (bool, error) {
	var count int64
	if err := r.DB.Model(&RevokedToken{}).Where("token_hash = ? AND expires_at > ?", tokenHash, time.Now()).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *RevokedTokenRepositoryImpl) CleanupExpired() error {
	return r.DB.Where("expires_at <= ?", time.Now()).Delete(&RevokedToken{}).Error
}
