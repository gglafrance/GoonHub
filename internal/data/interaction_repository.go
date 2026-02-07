package data

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SceneInteractions holds all interaction data for a scene
type SceneInteractions struct {
	Rating      float64
	Liked       bool
	JizzedCount int
}

type InteractionRepository interface {
	UpsertRating(userID, sceneID uint, rating float64) error
	DeleteRating(userID, sceneID uint) error
	GetRating(userID, sceneID uint) (*UserSceneRating, error)
	GetRatingsBySceneIDs(userID uint, sceneIDs []uint) (map[uint]float64, error)
	SetLike(userID, sceneID uint) error
	DeleteLike(userID, sceneID uint) error
	IsLiked(userID, sceneID uint) (bool, error)
	IncrementJizzed(userID, sceneID uint) (int, error)
	GetJizzedCount(userID, sceneID uint) (int, error)
	GetAllInteractions(userID, sceneID uint) (*SceneInteractions, error)
	GetLikedSceneIDs(userID uint) ([]uint, error)
	GetRatedSceneIDs(userID uint, minRating, maxRating float64) ([]uint, error)
	GetJizzedSceneIDs(userID uint, minCount, maxCount int) ([]uint, error)
	GetLikesBySceneIDs(userID uint, sceneIDs []uint) (map[uint]bool, error)
	GetJizzCountsBySceneIDs(userID uint, sceneIDs []uint) (map[uint]int, error)
}

type InteractionRepositoryImpl struct {
	DB *gorm.DB
}

func NewInteractionRepository(db *gorm.DB) *InteractionRepositoryImpl {
	return &InteractionRepositoryImpl{DB: db}
}

func (r *InteractionRepositoryImpl) UpsertRating(userID, sceneID uint, rating float64) error {
	record := UserSceneRating{
		UserID:  userID,
		SceneID: sceneID,
		Rating:  rating,
	}
	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "scene_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"rating", "updated_at"}),
	}).Create(&record).Error
}

func (r *InteractionRepositoryImpl) DeleteRating(userID, sceneID uint) error {
	return r.DB.Where("user_id = ? AND scene_id = ?", userID, sceneID).Delete(&UserSceneRating{}).Error
}

func (r *InteractionRepositoryImpl) GetRating(userID, sceneID uint) (*UserSceneRating, error) {
	var rating UserSceneRating
	err := r.DB.Where("user_id = ? AND scene_id = ?", userID, sceneID).First(&rating).Error
	if err != nil {
		return nil, err
	}
	return &rating, nil
}

func (r *InteractionRepositoryImpl) GetRatingsBySceneIDs(userID uint, sceneIDs []uint) (map[uint]float64, error) {
	if len(sceneIDs) == 0 {
		return make(map[uint]float64), nil
	}

	var ratings []UserSceneRating
	err := r.DB.Where("user_id = ? AND scene_id IN ?", userID, sceneIDs).Find(&ratings).Error
	if err != nil {
		return nil, err
	}

	result := make(map[uint]float64)
	for _, r := range ratings {
		result[r.SceneID] = r.Rating
	}
	return result, nil
}

func (r *InteractionRepositoryImpl) SetLike(userID, sceneID uint) error {
	like := UserSceneLike{
		UserID:  userID,
		SceneID: sceneID,
	}
	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "scene_id"}},
		DoNothing: true,
	}).Create(&like).Error
}

func (r *InteractionRepositoryImpl) DeleteLike(userID, sceneID uint) error {
	return r.DB.Where("user_id = ? AND scene_id = ?", userID, sceneID).Delete(&UserSceneLike{}).Error
}

func (r *InteractionRepositoryImpl) IsLiked(userID, sceneID uint) (bool, error) {
	var count int64
	err := r.DB.Model(&UserSceneLike{}).Where("user_id = ? AND scene_id = ?", userID, sceneID).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *InteractionRepositoryImpl) IncrementJizzed(userID, sceneID uint) (int, error) {
	record := UserSceneJizzed{
		UserID:  userID,
		SceneID: sceneID,
		Count:   1,
	}
	result := r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "scene_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"count":      gorm.Expr("user_scene_jizzed.count + 1"),
			"updated_at": gorm.Expr("NOW()"),
		}),
	}).Create(&record)
	if result.Error != nil {
		return 0, result.Error
	}

	// Fetch the current count
	var updated UserSceneJizzed
	err := r.DB.Where("user_id = ? AND scene_id = ?", userID, sceneID).First(&updated).Error
	if err != nil {
		return 0, err
	}
	return updated.Count, nil
}

func (r *InteractionRepositoryImpl) GetJizzedCount(userID, sceneID uint) (int, error) {
	var record UserSceneJizzed
	err := r.DB.Where("user_id = ? AND scene_id = ?", userID, sceneID).First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return record.Count, nil
}

func (r *InteractionRepositoryImpl) GetAllInteractions(userID, sceneID uint) (*SceneInteractions, error) {
	result := &SceneInteractions{}

	// Get rating
	var rating UserSceneRating
	err := r.DB.Where("user_id = ? AND scene_id = ?", userID, sceneID).First(&rating).Error
	if err == nil {
		result.Rating = rating.Rating
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// Check if liked
	var likeCount int64
	err = r.DB.Model(&UserSceneLike{}).Where("user_id = ? AND scene_id = ?", userID, sceneID).Count(&likeCount).Error
	if err != nil {
		return nil, err
	}
	result.Liked = likeCount > 0

	// Get jizzed count
	var jizzed UserSceneJizzed
	err = r.DB.Where("user_id = ? AND scene_id = ?", userID, sceneID).First(&jizzed).Error
	if err == nil {
		result.JizzedCount = jizzed.Count
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return result, nil
}

// Ensure gorm.ErrRecordNotFound is accessible for callers
func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

func (r *InteractionRepositoryImpl) GetLikedSceneIDs(userID uint) ([]uint, error) {
	var ids []uint
	err := r.DB.Model(&UserSceneLike{}).
		Where("user_id = ?", userID).
		Pluck("scene_id", &ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *InteractionRepositoryImpl) GetRatedSceneIDs(userID uint, minRating, maxRating float64) ([]uint, error) {
	var ids []uint
	query := r.DB.Model(&UserSceneRating{}).Where("user_id = ?", userID)

	if minRating > 0 {
		query = query.Where("rating >= ?", minRating)
	}
	if maxRating > 0 {
		query = query.Where("rating <= ?", maxRating)
	}

	err := query.Pluck("scene_id", &ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *InteractionRepositoryImpl) GetJizzedSceneIDs(userID uint, minCount, maxCount int) ([]uint, error) {
	var ids []uint
	query := r.DB.Model(&UserSceneJizzed{}).Where("user_id = ?", userID)

	if minCount > 0 {
		query = query.Where("count >= ?", minCount)
	}
	if maxCount > 0 {
		query = query.Where("count <= ?", maxCount)
	}

	err := query.Pluck("scene_id", &ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *InteractionRepositoryImpl) GetLikesBySceneIDs(userID uint, sceneIDs []uint) (map[uint]bool, error) {
	if len(sceneIDs) == 0 {
		return make(map[uint]bool), nil
	}

	var likes []UserSceneLike
	err := r.DB.Where("user_id = ? AND scene_id IN ?", userID, sceneIDs).Find(&likes).Error
	if err != nil {
		return nil, err
	}

	result := make(map[uint]bool)
	for _, l := range likes {
		result[l.SceneID] = true
	}
	return result, nil
}

func (r *InteractionRepositoryImpl) GetJizzCountsBySceneIDs(userID uint, sceneIDs []uint) (map[uint]int, error) {
	if len(sceneIDs) == 0 {
		return make(map[uint]int), nil
	}

	var records []UserSceneJizzed
	err := r.DB.Where("user_id = ? AND scene_id IN ?", userID, sceneIDs).Find(&records).Error
	if err != nil {
		return nil, err
	}

	result := make(map[uint]int)
	for _, j := range records {
		result[j.SceneID] = j.Count
	}
	return result, nil
}

// Ensure InteractionRepositoryImpl implements InteractionRepository
var _ InteractionRepository = (*InteractionRepositoryImpl)(nil)
