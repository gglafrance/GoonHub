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
	Count() (int64, error)
	UpdatePassword(userID uint, hashedPassword string) error
	UpdateUsername(userID uint, newUsername string) error
	List(page, limit int) ([]User, int64, error)
	UpdateRole(userID uint, role string) error
	UpdateLastLogin(userID uint) error
	Delete(userID uint) error
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

type VideoSearchParams struct {
	Page         int
	Limit        int
	Query        string
	TagIDs       []uint
	Actors       []string
	Studio       string
	MinDuration  int
	MaxDuration  int
	MinDate      *time.Time
	MaxDate      *time.Time
	MinHeight    int
	MaxHeight    int
	Sort         string
	UserID       uint
	Liked        *bool
	MinRating    float64
	MaxRating    float64
	MinJizzCount int
	MaxJizzCount int
}

type VideoRepository interface {
	Create(video *Video) error
	List(page, limit int) ([]Video, int64, error)
	GetByID(id uint) (*Video, error)
	GetByIDs(ids []uint) ([]Video, error)
	GetAll() ([]Video, error)
	GetDistinctStudios() ([]string, error)
	GetDistinctActors() ([]string, error)
	UpdateMetadata(id uint, duration int, width, height int, thumbnailPath string, spriteSheetPath string, vttPath string, spriteSheetCount int, thumbnailWidth int, thumbnailHeight int) error
	UpdateBasicMetadata(id uint, duration int, width, height int, frameRate float64, bitRate int64, videoCodec, audioCodec string) error
	UpdateThumbnail(id uint, thumbnailPath string, thumbnailWidth, thumbnailHeight int) error
	UpdateSprites(id uint, spriteSheetPath, vttPath string, spriteSheetCount int) error
	UpdateProcessingStatus(id uint, status string, errorMsg string) error
	GetPendingProcessing() ([]Video, error)
	GetVideosNeedingPhase(phase string) ([]Video, error)
	Delete(id uint) error
	UpdateDetails(id uint, title, description string) error
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

func (r *VideoRepositoryImpl) GetByIDs(ids []uint) ([]Video, error) {
	if len(ids) == 0 {
		return []Video{}, nil
	}

	var videos []Video
	if err := r.DB.Where("id IN ?", ids).Find(&videos).Error; err != nil {
		return nil, err
	}

	// Preserve the order of IDs
	idToVideo := make(map[uint]Video, len(videos))
	for _, v := range videos {
		idToVideo[v.ID] = v
	}

	result := make([]Video, 0, len(ids))
	for _, id := range ids {
		if v, ok := idToVideo[id]; ok {
			result = append(result, v)
		}
	}

	return result, nil
}

func (r *VideoRepositoryImpl) GetAll() ([]Video, error) {
	var videos []Video
	if err := r.DB.Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
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

func (r *VideoRepositoryImpl) UpdateBasicMetadata(id uint, duration int, width, height int, frameRate float64, bitRate int64, videoCodec, audioCodec string) error {
	updates := map[string]interface{}{
		"duration":    duration,
		"width":      width,
		"height":     height,
		"frame_rate":  frameRate,
		"bit_rate":    bitRate,
		"video_codec": videoCodec,
		"audio_codec": audioCodec,
	}
	return r.DB.Model(&Video{}).Where("id = ?", id).Updates(updates).Error
}

func (r *VideoRepositoryImpl) UpdateThumbnail(id uint, thumbnailPath string, thumbnailWidth, thumbnailHeight int) error {
	updates := map[string]interface{}{
		"thumbnail_path":   thumbnailPath,
		"thumbnail_width":  thumbnailWidth,
		"thumbnail_height": thumbnailHeight,
	}
	return r.DB.Model(&Video{}).Where("id = ?", id).Updates(updates).Error
}

func (r *VideoRepositoryImpl) UpdateSprites(id uint, spriteSheetPath, vttPath string, spriteSheetCount int) error {
	updates := map[string]interface{}{
		"sprite_sheet_path":  spriteSheetPath,
		"vtt_path":           vttPath,
		"sprite_sheet_count": spriteSheetCount,
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

func (r *VideoRepositoryImpl) GetVideosNeedingPhase(phase string) ([]Video, error) {
	var videos []Video

	baseQuery := r.DB.Model(&Video{}).
		Where("processing_status != ?", "failed").
		Where("deleted_at IS NULL").
		Where("NOT EXISTS (SELECT 1 FROM job_history jh WHERE jh.video_id = videos.id AND jh.phase = ? AND jh.status = 'running')", phase)

	switch phase {
	case "metadata":
		baseQuery = baseQuery.Where("duration = 0")
	case "thumbnail":
		baseQuery = baseQuery.Where("thumbnail_path = ''").Where("duration > 0")
	case "sprites":
		baseQuery = baseQuery.Where("sprite_sheet_path = ''").Where("duration > 0")
	default:
		return nil, nil
	}

	if err := baseQuery.Find(&videos).Error; err != nil {
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

func (r *VideoRepositoryImpl) UpdateDetails(id uint, title, description string) error {
	return r.DB.Model(&Video{}).Where("id = ?", id).
		Updates(map[string]interface{}{"title": title, "description": description}).Error
}

func (r *VideoRepositoryImpl) GetDistinctStudios() ([]string, error) {
	var studios []string
	err := r.DB.Model(&Video{}).
		Where("studio != '' AND deleted_at IS NULL").
		Distinct("studio").
		Order("studio ASC").
		Pluck("studio", &studios).Error
	if err != nil {
		return nil, err
	}
	return studios, nil
}

func (r *VideoRepositoryImpl) GetDistinctActors() ([]string, error) {
	var actors []string
	err := r.DB.Raw("SELECT DISTINCT unnest(actors) AS actor FROM videos WHERE deleted_at IS NULL ORDER BY actor ASC").
		Scan(&actors).Error
	if err != nil {
		return nil, err
	}
	return actors, nil
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

func (r *UserRepositoryImpl) Count() (int64, error) {
	var count int64
	if err := r.DB.Model(&User{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *UserRepositoryImpl) UpdatePassword(userID uint, hashedPassword string) error {
	return r.DB.Model(&User{}).Where("id = ?", userID).Update("password", hashedPassword).Error
}

func (r *UserRepositoryImpl) UpdateUsername(userID uint, newUsername string) error {
	return r.DB.Model(&User{}).Where("id = ?", userID).Update("username", newUsername).Error
}

func (r *UserRepositoryImpl) List(page, limit int) ([]User, int64, error) {
	var users []User
	var total int64

	offset := (page - 1) * limit

	if err := r.DB.Model(&User{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := r.DB.Limit(limit).Offset(offset).Order("created_at desc").Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func (r *UserRepositoryImpl) UpdateRole(userID uint, role string) error {
	return r.DB.Model(&User{}).Where("id = ?", userID).Update("role", role).Error
}

func (r *UserRepositoryImpl) UpdateLastLogin(userID uint) error {
	return r.DB.Model(&User{}).Where("id = ?", userID).Update("last_login_at", time.Now()).Error
}

func (r *UserRepositoryImpl) Delete(userID uint) error {
	return r.DB.Where("id = ?", userID).Delete(&User{}).Error
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
