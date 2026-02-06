package data

import (
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ PlaylistRepository = (*PlaylistRepositoryImpl)(nil)

type PlaylistRepositoryImpl struct {
	DB *gorm.DB
}

func NewPlaylistRepository(db *gorm.DB) *PlaylistRepositoryImpl {
	return &PlaylistRepositoryImpl{DB: db}
}

func (r *PlaylistRepositoryImpl) Create(playlist *Playlist) error {
	return r.DB.Create(playlist).Error
}

func (r *PlaylistRepositoryImpl) GetByUUID(uuid string) (*Playlist, error) {
	var playlist Playlist
	if err := r.DB.Preload("User").Where("uuid = ?", uuid).First(&playlist).Error; err != nil {
		return nil, err
	}
	return &playlist, nil
}

func (r *PlaylistRepositoryImpl) GetByID(id uint) (*Playlist, error) {
	var playlist Playlist
	if err := r.DB.Preload("User").First(&playlist, id).Error; err != nil {
		return nil, err
	}
	return &playlist, nil
}

func (r *PlaylistRepositoryImpl) Update(playlist *Playlist) error {
	return r.DB.Save(playlist).Error
}

func (r *PlaylistRepositoryImpl) Delete(id uint) error {
	result := r.DB.Delete(&Playlist{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *PlaylistRepositoryImpl) List(params PlaylistListParams) ([]Playlist, int64, error) {
	query := r.DB.Model(&Playlist{}).Preload("User")

	// Owner filter
	switch params.Owner {
	case "me":
		query = query.Where("user_id = ?", params.UserID)
	default:
		// "all": own playlists + public from others
		query = query.Where("(user_id = ? OR visibility = 'public')", params.UserID)
	}

	// Visibility filter
	if params.Visibility != "" {
		if params.Visibility == "private" {
			// Only own private playlists
			query = query.Where("user_id = ? AND visibility = 'private'", params.UserID)
		} else {
			query = query.Where("visibility = ?", params.Visibility)
		}
	}

	// Tag filter via subquery
	if len(params.TagIDs) > 0 {
		query = query.Where("id IN (?)",
			r.DB.Table("playlist_tags").
				Select("playlist_id").
				Where("tag_id IN ?", params.TagIDs).
				Group("playlist_id").
				Having("COUNT(DISTINCT tag_id) = ?", len(params.TagIDs)),
		)
	}

	// Count total before pagination
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Sorting
	switch params.Sort {
	case "created_at_asc":
		query = query.Order("created_at ASC")
	case "name_asc":
		query = query.Order("name ASC")
	case "name_desc":
		query = query.Order("name DESC")
	case "updated_at_desc":
		query = query.Order("updated_at DESC")
	case "scene_count_desc":
		query = query.Select("playlists.*, (SELECT COUNT(*) FROM playlist_scenes WHERE playlist_scenes.playlist_id = playlists.id) as scene_count_sort").
			Order("scene_count_sort DESC")
	default:
		query = query.Order("created_at DESC")
	}

	// Pagination
	if params.Limit <= 0 {
		params.Limit = 20
	}
	if params.Page <= 0 {
		params.Page = 1
	}
	offset := (params.Page - 1) * params.Limit
	query = query.Offset(offset).Limit(params.Limit)

	var playlists []Playlist
	if err := query.Find(&playlists).Error; err != nil {
		return nil, 0, err
	}

	return playlists, total, nil
}

func (r *PlaylistRepositoryImpl) AddScenes(playlistID uint, sceneIDs []uint) error {
	if len(sceneIDs) == 0 {
		return nil
	}

	return r.DB.Transaction(func(tx *gorm.DB) error {
		// Get current max position
		var maxPos int
		err := tx.Model(&PlaylistScene{}).
			Where("playlist_id = ?", playlistID).
			Select("COALESCE(MAX(position), -1)").
			Scan(&maxPos).Error
		if err != nil {
			return err
		}

		scenes := make([]PlaylistScene, len(sceneIDs))
		for i, sceneID := range sceneIDs {
			scenes[i] = PlaylistScene{
				PlaylistID: playlistID,
				SceneID:    sceneID,
				Position:   maxPos + 1 + i,
				AddedAt:    time.Now(),
			}
		}

		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&scenes)
		if result.Error != nil {
			return result.Error
		}

		// If fewer rows were created than requested, some were duplicates
		if result.RowsAffected < int64(len(sceneIDs)) && len(sceneIDs) == 1 {
			return duplicateSentinel
		}

		return nil
	})
}

// duplicateSentinel is an internal error used to signal duplicate scene addition
var duplicateSentinel = &duplicateError{}

type duplicateError struct{}

func (e *duplicateError) Error() string { return "duplicate" }

// ErrDuplicateSceneSentinel returns the duplicate scene sentinel error for testing.
func ErrDuplicateSceneSentinel() error { return duplicateSentinel }

// IsDuplicateScene checks if an error is a duplicate scene error from AddScenes
func IsDuplicateScene(err error) bool {
	_, ok := err.(*duplicateError)
	return ok
}

func (r *PlaylistRepositoryImpl) RemoveScene(playlistID uint, sceneID uint) error {
	result := r.DB.Where("playlist_id = ? AND scene_id = ?", playlistID, sceneID).Delete(&PlaylistScene{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *PlaylistRepositoryImpl) ReorderScenes(playlistID uint, sceneIDs []uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		for i, sceneID := range sceneIDs {
			result := tx.Model(&PlaylistScene{}).
				Where("playlist_id = ? AND scene_id = ?", playlistID, sceneID).
				Update("position", i)
			if result.Error != nil {
				return result.Error
			}
		}
		return nil
	})
}

func (r *PlaylistRepositoryImpl) GetPlaylistScenes(playlistID uint) ([]PlaylistScene, error) {
	var scenes []PlaylistScene
	err := r.DB.
		Preload("Scene").
		Where("playlist_id = ?", playlistID).
		Order("position ASC").
		Find(&scenes).Error
	if err != nil {
		return nil, err
	}
	return scenes, nil
}

func (r *PlaylistRepositoryImpl) GetMaxPosition(playlistID uint) (int, error) {
	var maxPos int
	err := r.DB.Model(&PlaylistScene{}).
		Where("playlist_id = ?", playlistID).
		Select("COALESCE(MAX(position), -1)").
		Scan(&maxPos).Error
	return maxPos, err
}

func (r *PlaylistRepositoryImpl) GetPlaylistTags(playlistID uint) ([]Tag, error) {
	var tags []Tag
	err := r.DB.
		Joins("JOIN playlist_tags ON playlist_tags.tag_id = tags.id").
		Where("playlist_tags.playlist_id = ?", playlistID).
		Order("tags.name ASC").
		Find(&tags).Error
	if err != nil {
		return nil, err
	}
	return tags, nil
}

func (r *PlaylistRepositoryImpl) SetPlaylistTags(playlistID uint, tagIDs []uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("playlist_id = ?", playlistID).Delete(&PlaylistTag{}).Error; err != nil {
			return err
		}

		if len(tagIDs) == 0 {
			return nil
		}

		playlistTags := make([]PlaylistTag, len(tagIDs))
		for i, tagID := range tagIDs {
			playlistTags[i] = PlaylistTag{
				PlaylistID: playlistID,
				TagID:      tagID,
			}
		}

		return tx.Create(&playlistTags).Error
	})
}

func (r *PlaylistRepositoryImpl) ToggleLike(userID uint, playlistID uint) (bool, error) {
	var existing PlaylistLike
	err := r.DB.Where("user_id = ? AND playlist_id = ?", userID, playlistID).First(&existing).Error

	if err == nil {
		// Exists, remove
		if err := r.DB.Where("user_id = ? AND playlist_id = ?", userID, playlistID).Delete(&PlaylistLike{}).Error; err != nil {
			return false, err
		}
		return false, nil
	}

	if err != gorm.ErrRecordNotFound {
		return false, err
	}

	// Doesn't exist, create
	like := PlaylistLike{
		UserID:     userID,
		PlaylistID: playlistID,
		CreatedAt:  time.Now(),
	}
	if err := r.DB.Create(&like).Error; err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "UNIQUE") {
			// Race condition, already liked
			return true, nil
		}
		return false, err
	}

	return true, nil
}

func (r *PlaylistRepositoryImpl) GetLikeStatus(userID uint, playlistID uint) (bool, error) {
	var count int64
	err := r.DB.Model(&PlaylistLike{}).
		Where("user_id = ? AND playlist_id = ?", userID, playlistID).
		Count(&count).Error
	return count > 0, err
}

func (r *PlaylistRepositoryImpl) GetLikeCount(playlistID uint) (int64, error) {
	var count int64
	err := r.DB.Model(&PlaylistLike{}).
		Where("playlist_id = ?", playlistID).
		Count(&count).Error
	return count, err
}

func (r *PlaylistRepositoryImpl) GetProgress(userID uint, playlistID uint) (*PlaylistProgress, error) {
	var progress PlaylistProgress
	err := r.DB.Where("user_id = ? AND playlist_id = ?", userID, playlistID).First(&progress).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &progress, nil
}

func (r *PlaylistRepositoryImpl) UpsertProgress(userID uint, playlistID uint, sceneID uint, positionS float64) error {
	now := time.Now()
	result := r.DB.Where("user_id = ? AND playlist_id = ?", userID, playlistID).First(&PlaylistProgress{})

	if result.Error == gorm.ErrRecordNotFound {
		// Insert
		return r.DB.Create(&PlaylistProgress{
			UserID:        userID,
			PlaylistID:    playlistID,
			LastSceneID:   &sceneID,
			LastPositionS: positionS,
			UpdatedAt:     now,
		}).Error
	}
	if result.Error != nil {
		return result.Error
	}

	// Update
	return r.DB.Model(&PlaylistProgress{}).
		Where("user_id = ? AND playlist_id = ?", userID, playlistID).
		Updates(map[string]any{
			"last_scene_id":   sceneID,
			"last_position_s": positionS,
			"updated_at":      now,
		}).Error
}

func (r *PlaylistRepositoryImpl) GetSceneCount(playlistID uint) (int64, error) {
	var count int64
	err := r.DB.Model(&PlaylistScene{}).Where("playlist_id = ?", playlistID).Count(&count).Error
	return count, err
}

func (r *PlaylistRepositoryImpl) GetTotalDuration(playlistID uint) (int64, error) {
	var total int64
	err := r.DB.
		Table("playlist_scenes").
		Select("COALESCE(SUM(scenes.duration), 0)").
		Joins("JOIN scenes ON scenes.id = playlist_scenes.scene_id").
		Where("playlist_scenes.playlist_id = ? AND scenes.deleted_at IS NULL", playlistID).
		Scan(&total).Error
	return total, err
}

func (r *PlaylistRepositoryImpl) GetThumbnailScenes(playlistID uint, limit int) ([]Scene, error) {
	var scenes []Scene
	err := r.DB.
		Joins("JOIN playlist_scenes ON playlist_scenes.scene_id = scenes.id").
		Where("playlist_scenes.playlist_id = ? AND scenes.deleted_at IS NULL AND scenes.thumbnail_path IS NOT NULL AND scenes.thumbnail_path != ''", playlistID).
		Order("playlist_scenes.position ASC").
		Limit(limit).
		Find(&scenes).Error
	if err != nil {
		return nil, err
	}
	return scenes, nil
}
