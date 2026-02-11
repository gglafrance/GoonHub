package data

import (
	"time"

	"github.com/lib/pq"
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

type SceneSearchParams struct {
	Page             int
	Limit            int
	Query            string
	TagIDs           []uint
	Actors           []string
	Studio           string
	MinDuration      int
	MaxDuration      int
	MinDate          *time.Time
	MaxDate          *time.Time
	MinHeight        int
	MaxHeight        int
	Sort             string
	UserID           uint
	Liked            *bool
	MinRating        float64
	MaxRating        float64
	MinJizzCount     int
	MaxJizzCount     int
	SceneIDs         []uint   // Pre-filter to specific scene IDs (e.g., folder search)
	MatchingStrategy string   // Meilisearch matching strategy: "last", "all", or "frequency"
	MarkerLabels     []string // Filter to scenes with markers having these labels (user-specific)
	Origin           string   // Filter by origin (web, dvd, personal, stash, unknown)
	Type             string   // Filter by type (standard, jav, hentai, amateur, professional, vr, compilation, pmv)
	HasPornDBID      *bool    // nil = no filter, true = has, false = missing
	Seed             int64    // Random shuffle seed (0 = auto-generate)
}

// ScanLookupEntry is a lightweight struct for move detection during scans.
// It avoids loading full Scene objects when only a few fields are needed.
type ScanLookupEntry struct {
	ID               uint
	StoredPath       string
	Size             int64
	OriginalFilename string
	IsDeleted        bool
}

// ScenePathInfo is a lightweight struct for missing file detection during scans.
type ScenePathInfo struct {
	ID            uint
	StoredPath    string
	StoragePathID uint
	Title         string
}

type SceneRepository interface {
	Create(scene *Scene) error
	CreateInBatches(scenes []*Scene, batchSize int) error
	List(page, limit int) ([]Scene, int64, error)
	GetByID(id uint) (*Scene, error)
	GetByIDs(ids []uint) ([]Scene, error)
	GetAll() ([]Scene, error)
	GetAllWithStoragePath() ([]Scene, error)
	GetAllStoredPathSet() (map[string]struct{}, error)
	GetScanLookupEntries() ([]ScanLookupEntry, error)
	GetScenePathsForMissingDetection() ([]ScenePathInfo, error)
	GetDistinctStudios() ([]string, error)
	GetDistinctActors() ([]string, error)
	UpdateMetadata(id uint, duration int, width, height int, thumbnailPath string, spriteSheetPath string, vttPath string, spriteSheetCount int, thumbnailWidth int, thumbnailHeight int) error
	UpdateBasicMetadata(id uint, duration int, width, height int, frameRate float64, bitRate int64, videoCodec, audioCodec string) error
	UpdateThumbnail(id uint, thumbnailPath string, thumbnailWidth, thumbnailHeight int) error
	UpdateSprites(id uint, spriteSheetPath, vttPath string, spriteSheetCount int) error
	UpdatePreviewVideoPath(id uint, previewVideoPath string) error
	UpdateProcessingStatus(id uint, status string, errorMsg string) error
	UpdateIsCorrupted(id uint, isCorrupted bool) error
	GetPendingProcessing() ([]Scene, error)
	GetScenesNeedingPhase(phase string) ([]Scene, error)
	Delete(id uint) error
	UpdateDetails(id uint, title, description string, releaseDate *time.Time) error
	UpdateSceneMetadata(id uint, title, description, studio string, releaseDate *time.Time, porndbSceneID string) error
	ExistsByStoredPath(path string) (bool, error)
	GetByStoredPath(path string) (*Scene, error)
	MarkAsMissing(id uint) error
	Restore(id uint) error
	UpdateStoredPath(id uint, newPath string, storagePathID *uint) error
	GetBySizeAndFilename(size int64, filename string) (*Scene, error)
	BulkUpdateStudio(sceneIDs []uint, studio string) error
	UpdateActors(id uint, actors []string) error
	UpdateOriginAndType(id uint, origin, sceneType string) error

	// Trash management
	MoveToTrash(id uint) (*time.Time, error)
	RestoreFromTrash(id uint) error
	HardDelete(id uint) (*Scene, error)
	ListTrashed(page, limit int) ([]Scene, int64, error)
	CountTrashed() (int64, error)
	GetExpiredTrashScenes(retentionDays int) ([]Scene, error)
	GetByIDIncludingTrashed(id uint) (*Scene, error)

	// PornDB filtering
	GetSceneIDsWithPornDBID() ([]uint, error)
	GetSceneIDsWithoutPornDBID() ([]uint, error)

	ListPopular(limit int) ([]Scene, error)
}

type SceneRepositoryImpl struct {
	DB *gorm.DB
}

func NewSceneRepository(db *gorm.DB) *SceneRepositoryImpl {
	return &SceneRepositoryImpl{DB: db}
}

func (r *SceneRepositoryImpl) Create(scene *Scene) error {
	return r.DB.Create(scene).Error
}

func (r *SceneRepositoryImpl) List(page, limit int) ([]Scene, int64, error) {
	var scenes []Scene
	var total int64

	offset := (page - 1) * limit

	if err := r.DB.Model(&Scene{}).Where("trashed_at IS NULL").Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := r.DB.Where("trashed_at IS NULL").Limit(limit).Offset(offset).Order("created_at desc").Find(&scenes).Error; err != nil {
		return nil, 0, err
	}

	return scenes, total, nil
}

func (r *SceneRepositoryImpl) GetByID(id uint) (*Scene, error) {
	var scene Scene
	if err := r.DB.Where("trashed_at IS NULL").First(&scene, id).Error; err != nil {
		return nil, err
	}
	return &scene, nil
}

func (r *SceneRepositoryImpl) GetByIDs(ids []uint) ([]Scene, error) {
	if len(ids) == 0 {
		return []Scene{}, nil
	}

	var scenes []Scene
	if err := r.DB.Where("id IN ? AND trashed_at IS NULL", ids).Find(&scenes).Error; err != nil {
		return nil, err
	}

	// Preserve the order of IDs
	idToScene := make(map[uint]Scene, len(scenes))
	for _, s := range scenes {
		idToScene[s.ID] = s
	}

	result := make([]Scene, 0, len(ids))
	for _, id := range ids {
		if s, ok := idToScene[id]; ok {
			result = append(result, s)
		}
	}

	return result, nil
}

func (r *SceneRepositoryImpl) GetAll() ([]Scene, error) {
	var scenes []Scene
	if err := r.DB.Where("trashed_at IS NULL").Find(&scenes).Error; err != nil {
		return nil, err
	}
	return scenes, nil
}

func (r *SceneRepositoryImpl) UpdateMetadata(id uint, duration int, width, height int, thumbnailPath string, spriteSheetPath string, vttPath string, spriteSheetCount int, thumbnailWidth int, thumbnailHeight int) error {
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
	return r.DB.Model(&Scene{}).Where("id = ?", id).Updates(updates).Error
}

func (r *SceneRepositoryImpl) UpdateBasicMetadata(id uint, duration int, width, height int, frameRate float64, bitRate int64, videoCodec, audioCodec string) error {
	updates := map[string]interface{}{
		"duration":    duration,
		"width":       width,
		"height":      height,
		"frame_rate":  frameRate,
		"bit_rate":    bitRate,
		"video_codec": videoCodec,
		"audio_codec": audioCodec,
	}
	return r.DB.Model(&Scene{}).Where("id = ?", id).Updates(updates).Error
}

func (r *SceneRepositoryImpl) UpdateThumbnail(id uint, thumbnailPath string, thumbnailWidth, thumbnailHeight int) error {
	updates := map[string]interface{}{
		"thumbnail_path":   thumbnailPath,
		"thumbnail_width":  thumbnailWidth,
		"thumbnail_height": thumbnailHeight,
	}
	return r.DB.Model(&Scene{}).Where("id = ?", id).Updates(updates).Error
}

func (r *SceneRepositoryImpl) UpdateSprites(id uint, spriteSheetPath, vttPath string, spriteSheetCount int) error {
	updates := map[string]interface{}{
		"sprite_sheet_path":  spriteSheetPath,
		"vtt_path":           vttPath,
		"sprite_sheet_count": spriteSheetCount,
	}
	return r.DB.Model(&Scene{}).Where("id = ?", id).Updates(updates).Error
}

func (r *SceneRepositoryImpl) UpdatePreviewVideoPath(id uint, previewVideoPath string) error {
	return r.DB.Model(&Scene{}).Where("id = ?", id).Update("preview_video_path", previewVideoPath).Error
}

func (r *SceneRepositoryImpl) UpdateProcessingStatus(id uint, status string, errorMsg string) error {
	updates := map[string]interface{}{
		"processing_status": status,
	}
	if errorMsg != "" {
		updates["processing_error"] = errorMsg
	}
	return r.DB.Model(&Scene{}).Where("id = ?", id).Updates(updates).Error
}

func (r *SceneRepositoryImpl) UpdateIsCorrupted(id uint, isCorrupted bool) error {
	return r.DB.Model(&Scene{}).Where("id = ?", id).Update("is_corrupted", isCorrupted).Error
}

func (r *SceneRepositoryImpl) GetPendingProcessing() ([]Scene, error) {
	var scenes []Scene
	if err := r.DB.Where("processing_status = ? AND trashed_at IS NULL", "pending").Find(&scenes).Error; err != nil {
		return nil, err
	}
	return scenes, nil
}

func (r *SceneRepositoryImpl) GetScenesNeedingPhase(phase string) ([]Scene, error) {
	var scenes []Scene

	baseQuery := r.DB.Model(&Scene{}).
		Where("deleted_at IS NULL").
		Where("trashed_at IS NULL").
		Where("NOT EXISTS (SELECT 1 FROM job_history jh WHERE jh.scene_id = scenes.id AND jh.phase = ? AND jh.status IN ('pending', 'running'))", phase)

	switch phase {
	case "metadata":
		baseQuery = baseQuery.Where("duration = 0")
	case "thumbnail":
		baseQuery = baseQuery.Where("thumbnail_path = ''").Where("duration > 0")
	case "sprites":
		baseQuery = baseQuery.Where("sprite_sheet_path = ''").Where("duration > 0")
	case "animated_thumbnails":
		// Scenes that have markers without animated thumbnails OR missing scene preview video
		var animScenes []Scene
		err := r.DB.Raw(`
			SELECT DISTINCT s.* FROM scenes s
			WHERE s.duration > 0 AND s.deleted_at IS NULL AND s.trashed_at IS NULL
			AND (
				(s.preview_video_path = '' OR s.preview_video_path IS NULL)
				OR EXISTS (
					SELECT 1 FROM user_scene_markers m
					WHERE m.scene_id = s.id
					AND (m.animated_thumbnail_path = '' OR m.animated_thumbnail_path IS NULL)
				)
			)
		`).Find(&animScenes).Error
		if err != nil {
			return nil, err
		}
		return animScenes, nil
	default:
		return nil, nil
	}

	if err := baseQuery.Find(&scenes).Error; err != nil {
		return nil, err
	}
	return scenes, nil
}

func (r *SceneRepositoryImpl) Delete(id uint) error {
	var scene Scene
	if err := r.DB.Where("trashed_at IS NULL").First(&scene, id).Error; err != nil {
		return err
	}
	return r.DB.Delete(&scene).Error
}

func (r *SceneRepositoryImpl) UpdateDetails(id uint, title, description string, releaseDate *time.Time) error {
	updates := map[string]interface{}{"title": title, "description": description}
	if releaseDate != nil {
		if releaseDate.IsZero() {
			updates["release_date"] = nil
		} else {
			updates["release_date"] = releaseDate
		}
	}
	return r.DB.Model(&Scene{}).Where("id = ?", id).Updates(updates).Error
}

func (r *SceneRepositoryImpl) UpdateSceneMetadata(id uint, title, description, studio string, releaseDate *time.Time, porndbSceneID string) error {
	updates := map[string]interface{}{"title": title, "description": description, "studio": studio, "porndb_scene_id": porndbSceneID}
	if releaseDate != nil {
		updates["release_date"] = releaseDate
	}
	return r.DB.Model(&Scene{}).Where("id = ?", id).Updates(updates).Error
}

func (r *SceneRepositoryImpl) GetDistinctStudios() ([]string, error) {
	var studios []string
	err := r.DB.Model(&Scene{}).
		Where("studio != '' AND deleted_at IS NULL").
		Distinct("studio").
		Order("studio ASC").
		Pluck("studio", &studios).Error
	if err != nil {
		return nil, err
	}
	return studios, nil
}

func (r *SceneRepositoryImpl) GetDistinctActors() ([]string, error) {
	var actors []string
	// Get actor names from the actors table (those with at least one scene)
	err := r.DB.Raw(`
		SELECT DISTINCT a.name
		FROM actors a
		INNER JOIN scene_actors sa ON sa.actor_id = a.id
		INNER JOIN scenes s ON s.id = sa.scene_id AND s.deleted_at IS NULL
		ORDER BY a.name ASC
	`).Scan(&actors).Error
	if err != nil {
		return nil, err
	}
	return actors, nil
}

func (r *SceneRepositoryImpl) ExistsByStoredPath(path string) (bool, error) {
	var count int64
	if err := r.DB.Model(&Scene{}).Where("stored_path = ? AND trashed_at IS NULL", path).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *SceneRepositoryImpl) GetByStoredPath(path string) (*Scene, error) {
	var scene Scene
	if err := r.DB.Where("stored_path = ? AND trashed_at IS NULL", path).First(&scene).Error; err != nil {
		return nil, err
	}
	return &scene, nil
}

func (r *SceneRepositoryImpl) GetAllWithStoragePath() ([]Scene, error) {
	var scenes []Scene
	if err := r.DB.Where("storage_path_id IS NOT NULL AND trashed_at IS NULL").Find(&scenes).Error; err != nil {
		return nil, err
	}
	return scenes, nil
}

func (r *SceneRepositoryImpl) CreateInBatches(scenes []*Scene, batchSize int) error {
	if len(scenes) == 0 {
		return nil
	}
	return r.DB.CreateInBatches(scenes, batchSize).Error
}

func (r *SceneRepositoryImpl) GetAllStoredPathSet() (map[string]struct{}, error) {
	var paths []string
	if err := r.DB.Model(&Scene{}).Where("storage_path_id IS NOT NULL AND trashed_at IS NULL").Pluck("stored_path", &paths).Error; err != nil {
		return nil, err
	}
	result := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		result[p] = struct{}{}
	}
	return result, nil
}

func (r *SceneRepositoryImpl) GetScanLookupEntries() ([]ScanLookupEntry, error) {
	var entries []ScanLookupEntry
	if err := r.DB.Unscoped().Model(&Scene{}).
		Select("id, stored_path, size, original_filename, (deleted_at IS NOT NULL) as is_deleted").
		Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

func (r *SceneRepositoryImpl) GetScenePathsForMissingDetection() ([]ScenePathInfo, error) {
	var entries []ScenePathInfo
	if err := r.DB.Model(&Scene{}).
		Select("id, stored_path, storage_path_id, title").
		Where("storage_path_id IS NOT NULL AND trashed_at IS NULL").
		Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

func (r *SceneRepositoryImpl) MarkAsMissing(id uint) error {
	// Soft delete the scene - sets deleted_at to current timestamp
	return r.DB.Delete(&Scene{}, id).Error
}

func (r *SceneRepositoryImpl) Restore(id uint) error {
	// Restore a soft-deleted scene by clearing deleted_at
	return r.DB.Unscoped().Model(&Scene{}).Where("id = ?", id).Update("deleted_at", nil).Error
}

func (r *SceneRepositoryImpl) UpdateStoredPath(id uint, newPath string, storagePathID *uint) error {
	updates := map[string]interface{}{
		"stored_path": newPath,
	}
	if storagePathID != nil {
		updates["storage_path_id"] = *storagePathID
	}
	return r.DB.Model(&Scene{}).Where("id = ?", id).Updates(updates).Error
}

func (r *SceneRepositoryImpl) GetBySizeAndFilename(size int64, filename string) (*Scene, error) {
	var scene Scene
	// Use Unscoped to include soft-deleted records - allows finding moved files that were previously marked as missing
	err := r.DB.Unscoped().Where("size = ? AND original_filename = ?", size, filename).First(&scene).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &scene, nil
}

func (r *SceneRepositoryImpl) BulkUpdateStudio(sceneIDs []uint, studio string) error {
	if len(sceneIDs) == 0 {
		return nil
	}
	return r.DB.Model(&Scene{}).Where("id IN ?", sceneIDs).Update("studio", studio).Error
}

func (r *SceneRepositoryImpl) UpdateActors(id uint, actors []string) error {
	return r.DB.Model(&Scene{}).Where("id = ?", id).Update("actors", pq.StringArray(actors)).Error
}

func (r *SceneRepositoryImpl) UpdateOriginAndType(id uint, origin, sceneType string) error {
	updates := map[string]interface{}{}
	if origin != "" {
		updates["origin"] = origin
	}
	if sceneType != "" {
		updates["type"] = sceneType
	}
	if len(updates) == 0 {
		return nil
	}
	return r.DB.Model(&Scene{}).Where("id = ?", id).Updates(updates).Error
}

func (r *SceneRepositoryImpl) MoveToTrash(id uint) (*time.Time, error) {
	now := time.Now()
	if err := r.DB.Model(&Scene{}).Where("id = ?", id).Update("trashed_at", now).Error; err != nil {
		return nil, err
	}
	return &now, nil
}

func (r *SceneRepositoryImpl) RestoreFromTrash(id uint) error {
	return r.DB.Model(&Scene{}).Where("id = ?", id).Update("trashed_at", nil).Error
}

func (r *SceneRepositoryImpl) HardDelete(id uint) (*Scene, error) {
	var scene Scene
	// Use Unscoped to find even soft-deleted scenes, and include trashed
	if err := r.DB.Unscoped().First(&scene, id).Error; err != nil {
		return nil, err
	}
	// Permanently delete
	if err := r.DB.Unscoped().Delete(&scene).Error; err != nil {
		return nil, err
	}
	return &scene, nil
}

func (r *SceneRepositoryImpl) ListTrashed(page, limit int) ([]Scene, int64, error) {
	var scenes []Scene
	var total int64

	offset := (page - 1) * limit

	if err := r.DB.Model(&Scene{}).Where("trashed_at IS NOT NULL").Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := r.DB.Where("trashed_at IS NOT NULL").
		Limit(limit).Offset(offset).Order("trashed_at desc").Find(&scenes).Error; err != nil {
		return nil, 0, err
	}

	return scenes, total, nil
}

func (r *SceneRepositoryImpl) CountTrashed() (int64, error) {
	var count int64
	if err := r.DB.Model(&Scene{}).Where("trashed_at IS NOT NULL").Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *SceneRepositoryImpl) GetExpiredTrashScenes(retentionDays int) ([]Scene, error) {
	var scenes []Scene
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	if err := r.DB.Where("trashed_at IS NOT NULL AND trashed_at < ?", cutoff).Find(&scenes).Error; err != nil {
		return nil, err
	}
	return scenes, nil
}

func (r *SceneRepositoryImpl) GetByIDIncludingTrashed(id uint) (*Scene, error) {
	var scene Scene
	// Use Unscoped to include soft-deleted, and query trashed scenes too
	if err := r.DB.Unscoped().First(&scene, id).Error; err != nil {
		return nil, err
	}
	return &scene, nil
}

func (r *SceneRepositoryImpl) GetSceneIDsWithPornDBID() ([]uint, error) {
	var ids []uint
	err := r.DB.Model(&Scene{}).
		Where("porndb_scene_id IS NOT NULL AND porndb_scene_id != '' AND trashed_at IS NULL").
		Pluck("id", &ids).Error
	return ids, err
}

func (r *SceneRepositoryImpl) GetSceneIDsWithoutPornDBID() ([]uint, error) {
	var ids []uint
	err := r.DB.Model(&Scene{}).
		Where("(porndb_scene_id IS NULL OR porndb_scene_id = '') AND trashed_at IS NULL").
		Pluck("id", &ids).Error
	return ids, err
}

func (r *SceneRepositoryImpl) ListPopular(limit int) ([]Scene, error) {
	var scenes []Scene
	err := r.DB.Where("trashed_at IS NULL").
		Order("view_count DESC").
		Limit(limit).
		Find(&scenes).Error
	if err != nil {
		return nil, err
	}
	return scenes, nil
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
