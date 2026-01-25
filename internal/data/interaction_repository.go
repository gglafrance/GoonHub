package data

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type InteractionRepository interface {
	UpsertRating(userID, videoID uint, rating float64) error
	DeleteRating(userID, videoID uint) error
	GetRating(userID, videoID uint) (*UserVideoRating, error)
	SetLike(userID, videoID uint) error
	DeleteLike(userID, videoID uint) error
	IsLiked(userID, videoID uint) (bool, error)
	IncrementJizzed(userID, videoID uint) (int, error)
	GetJizzedCount(userID, videoID uint) (int, error)
	GetLikedVideoIDs(userID uint) ([]uint, error)
	GetRatedVideoIDs(userID uint, minRating, maxRating float64) ([]uint, error)
	GetJizzedVideoIDs(userID uint, minCount, maxCount int) ([]uint, error)
}

type InteractionRepositoryImpl struct {
	DB *gorm.DB
}

func NewInteractionRepository(db *gorm.DB) *InteractionRepositoryImpl {
	return &InteractionRepositoryImpl{DB: db}
}

func (r *InteractionRepositoryImpl) UpsertRating(userID, videoID uint, rating float64) error {
	record := UserVideoRating{
		UserID:  userID,
		VideoID: videoID,
		Rating:  rating,
	}
	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "video_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"rating", "updated_at"}),
	}).Create(&record).Error
}

func (r *InteractionRepositoryImpl) DeleteRating(userID, videoID uint) error {
	return r.DB.Where("user_id = ? AND video_id = ?", userID, videoID).Delete(&UserVideoRating{}).Error
}

func (r *InteractionRepositoryImpl) GetRating(userID, videoID uint) (*UserVideoRating, error) {
	var rating UserVideoRating
	err := r.DB.Where("user_id = ? AND video_id = ?", userID, videoID).First(&rating).Error
	if err != nil {
		return nil, err
	}
	return &rating, nil
}

func (r *InteractionRepositoryImpl) SetLike(userID, videoID uint) error {
	like := UserVideoLike{
		UserID:  userID,
		VideoID: videoID,
	}
	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "video_id"}},
		DoNothing: true,
	}).Create(&like).Error
}

func (r *InteractionRepositoryImpl) DeleteLike(userID, videoID uint) error {
	return r.DB.Where("user_id = ? AND video_id = ?", userID, videoID).Delete(&UserVideoLike{}).Error
}

func (r *InteractionRepositoryImpl) IsLiked(userID, videoID uint) (bool, error) {
	var count int64
	err := r.DB.Model(&UserVideoLike{}).Where("user_id = ? AND video_id = ?", userID, videoID).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *InteractionRepositoryImpl) IncrementJizzed(userID, videoID uint) (int, error) {
	record := UserVideoJizzed{
		UserID:  userID,
		VideoID: videoID,
		Count:   1,
	}
	result := r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "video_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"count":      gorm.Expr("user_video_jizzed.count + 1"),
			"updated_at": gorm.Expr("NOW()"),
		}),
	}).Create(&record)
	if result.Error != nil {
		return 0, result.Error
	}

	// Fetch the current count
	var updated UserVideoJizzed
	err := r.DB.Where("user_id = ? AND video_id = ?", userID, videoID).First(&updated).Error
	if err != nil {
		return 0, err
	}
	return updated.Count, nil
}

func (r *InteractionRepositoryImpl) GetJizzedCount(userID, videoID uint) (int, error) {
	var record UserVideoJizzed
	err := r.DB.Where("user_id = ? AND video_id = ?", userID, videoID).First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return record.Count, nil
}

// Ensure gorm.ErrRecordNotFound is accessible for callers
func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

func (r *InteractionRepositoryImpl) GetLikedVideoIDs(userID uint) ([]uint, error) {
	var ids []uint
	err := r.DB.Model(&UserVideoLike{}).
		Where("user_id = ?", userID).
		Pluck("video_id", &ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *InteractionRepositoryImpl) GetRatedVideoIDs(userID uint, minRating, maxRating float64) ([]uint, error) {
	var ids []uint
	query := r.DB.Model(&UserVideoRating{}).Where("user_id = ?", userID)

	if minRating > 0 {
		query = query.Where("rating >= ?", minRating)
	}
	if maxRating > 0 {
		query = query.Where("rating <= ?", maxRating)
	}

	err := query.Pluck("video_id", &ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *InteractionRepositoryImpl) GetJizzedVideoIDs(userID uint, minCount, maxCount int) ([]uint, error) {
	var ids []uint
	query := r.DB.Model(&UserVideoJizzed{}).Where("user_id = ?", userID)

	if minCount > 0 {
		query = query.Where("count >= ?", minCount)
	}
	if maxCount > 0 {
		query = query.Where("count <= ?", maxCount)
	}

	err := query.Pluck("video_id", &ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// Ensure InteractionRepositoryImpl implements InteractionRepository
var _ InteractionRepository = (*InteractionRepositoryImpl)(nil)
