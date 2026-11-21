package data

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// ImportRepository provides the write primitives used by external importers
// (admin import API). Writes are meant to be grouped with RunInTransaction so
// that a scene and all of its associations are imported atomically.
type ImportRepository interface {
	// RunInTransaction runs fn with a repository bound to a single DB transaction.
	// If fn returns an error, every write made through the tx repository is rolled back.
	RunInTransaction(fn func(tx ImportRepository) error) error

	// Scenes
	CreateScene(scene *Scene) error
	UpdateSceneFields(sceneID uint, fields map[string]any) error
	SetSceneTags(sceneID uint, tagIDs []uint) error
	SetSceneActors(sceneID uint, actorIDs []uint) error

	// User interactions
	UpsertRating(userID, sceneID uint, rating float64) error
	SetLiked(userID, sceneID uint, liked bool) error
	SetJizzedCount(userID, sceneID uint, count int) error
	// InsertWatchIfAbsent inserts the watch unless an entry already exists for the same
	// (user, scene, watched_at truncated to the second). Returns true if inserted.
	InsertWatchIfAbsent(watch *UserSceneWatch) (bool, error)

	// Markers
	// FindMarker returns the oldest marker matching the natural key, or (nil, nil) if none.
	FindMarker(userID, sceneID uint, timestamp int, label string) (*UserSceneMarker, error)
	CreateMarker(marker *UserSceneMarker) error
	UpdateMarkerColor(markerID uint, color string) error
	ApplyLabelTagsToMarker(userID, markerID uint, label string) error
	AddMarkerTags(markerID uint, tagIDs []uint) error
	SetMarkerTags(markerID uint, tagIDs []uint) error

	// Summary (read-only)
	// GetScenesForSummary returns the non-deleted scenes (trashed ones included) among ids,
	// with only the columns needed for the import summary loaded.
	GetScenesForSummary(ids []uint) ([]Scene, error)
	CountMarkersByScenes(userID uint, sceneIDs []uint) (map[uint]int64, error)
	CountWatchesByScenes(userID uint, sceneIDs []uint) (map[uint]int64, error)
}

type ImportRepositoryImpl struct {
	DB *gorm.DB
}

func NewImportRepository(db *gorm.DB) *ImportRepositoryImpl {
	return &ImportRepositoryImpl{DB: db}
}

func (r *ImportRepositoryImpl) RunInTransaction(fn func(tx ImportRepository) error) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		return fn(&ImportRepositoryImpl{DB: tx})
	})
}

func (r *ImportRepositoryImpl) CreateScene(scene *Scene) error {
	return r.DB.Create(scene).Error
}

func (r *ImportRepositoryImpl) UpdateSceneFields(sceneID uint, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	return r.DB.Model(&Scene{}).Where("id = ?", sceneID).Updates(fields).Error
}

func (r *ImportRepositoryImpl) SetSceneTags(sceneID uint, tagIDs []uint) error {
	return (&TagRepositoryImpl{DB: r.DB}).SetSceneTags(sceneID, tagIDs)
}

func (r *ImportRepositoryImpl) SetSceneActors(sceneID uint, actorIDs []uint) error {
	return (&ActorRepositoryImpl{DB: r.DB}).SetSceneActors(sceneID, actorIDs)
}

func (r *ImportRepositoryImpl) UpsertRating(userID, sceneID uint, rating float64) error {
	return (&InteractionRepositoryImpl{DB: r.DB}).UpsertRating(userID, sceneID, rating)
}

func (r *ImportRepositoryImpl) SetLiked(userID, sceneID uint, liked bool) error {
	interactions := &InteractionRepositoryImpl{DB: r.DB}
	if liked {
		return interactions.SetLike(userID, sceneID)
	}
	return interactions.DeleteLike(userID, sceneID)
}

func (r *ImportRepositoryImpl) SetJizzedCount(userID, sceneID uint, count int) error {
	return (&InteractionRepositoryImpl{DB: r.DB}).SetJizzedCount(userID, sceneID, count)
}

func (r *ImportRepositoryImpl) InsertWatchIfAbsent(watch *UserSceneWatch) (bool, error) {
	second := watch.WatchedAt.UTC().Truncate(time.Second)
	var count int64
	err := r.DB.Model(&UserSceneWatch{}).
		Where("user_id = ? AND scene_id = ? AND watched_at >= ? AND watched_at < ?",
			watch.UserID, watch.SceneID, second, second.Add(time.Second)).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	if count > 0 {
		return false, nil
	}
	if err := r.DB.Create(watch).Error; err != nil {
		return false, err
	}
	return true, nil
}

func (r *ImportRepositoryImpl) FindMarker(userID, sceneID uint, timestamp int, label string) (*UserSceneMarker, error) {
	var marker UserSceneMarker
	err := r.DB.
		Where("user_id = ? AND scene_id = ? AND timestamp = ? AND label = ?", userID, sceneID, timestamp, label).
		Order("id ASC").
		First(&marker).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &marker, nil
}

func (r *ImportRepositoryImpl) CreateMarker(marker *UserSceneMarker) error {
	return r.DB.Create(marker).Error
}

func (r *ImportRepositoryImpl) UpdateMarkerColor(markerID uint, color string) error {
	return r.DB.Model(&UserSceneMarker{}).Where("id = ?", markerID).Update("color", color).Error
}

func (r *ImportRepositoryImpl) ApplyLabelTagsToMarker(userID, markerID uint, label string) error {
	return (&MarkerRepositoryImpl{DB: r.DB}).ApplyLabelTagsToMarker(userID, markerID, label)
}

func (r *ImportRepositoryImpl) AddMarkerTags(markerID uint, tagIDs []uint) error {
	return (&MarkerRepositoryImpl{DB: r.DB}).AddMarkerTags(markerID, tagIDs)
}

func (r *ImportRepositoryImpl) SetMarkerTags(markerID uint, tagIDs []uint) error {
	return (&MarkerRepositoryImpl{DB: r.DB}).SetMarkerTags(markerID, tagIDs)
}

func (r *ImportRepositoryImpl) GetScenesForSummary(ids []uint) ([]Scene, error) {
	if len(ids) == 0 {
		return []Scene{}, nil
	}
	var scenes []Scene
	err := r.DB.
		Select("id", "title", "stored_path", "studio_id", "view_count", "trashed_at").
		Where("id IN ?", ids).
		Find(&scenes).Error
	if err != nil {
		return nil, err
	}
	return scenes, nil
}

type sceneCountRow struct {
	SceneID uint
	Count   int64
}

func (r *ImportRepositoryImpl) countByScenes(model any, userID uint, sceneIDs []uint) (map[uint]int64, error) {
	result := make(map[uint]int64, len(sceneIDs))
	if len(sceneIDs) == 0 {
		return result, nil
	}
	var rows []sceneCountRow
	err := r.DB.Model(model).
		Select("scene_id, COUNT(*) AS count").
		Where("user_id = ? AND scene_id IN ?", userID, sceneIDs).
		Group("scene_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.SceneID] = row.Count
	}
	return result, nil
}

func (r *ImportRepositoryImpl) CountMarkersByScenes(userID uint, sceneIDs []uint) (map[uint]int64, error) {
	return r.countByScenes(&UserSceneMarker{}, userID, sceneIDs)
}

func (r *ImportRepositoryImpl) CountWatchesByScenes(userID uint, sceneIDs []uint) (map[uint]int64, error) {
	return r.countByScenes(&UserSceneWatch{}, userID, sceneIDs)
}
