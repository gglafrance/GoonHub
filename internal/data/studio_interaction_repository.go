package data

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type StudioInteractionRepository interface {
	UpsertRating(userID, studioID uint, rating float64) error
	DeleteRating(userID, studioID uint) error
	GetRating(userID, studioID uint) (*UserStudioRating, error)
	SetLike(userID, studioID uint) error
	DeleteLike(userID, studioID uint) error
	IsLiked(userID, studioID uint) (bool, error)
	GetAllInteractions(userID, studioID uint) (*StudioInteractions, error)
	GetLikedStudioIDs(userID uint) ([]uint, error)
}

type StudioInteractionRepositoryImpl struct {
	DB *gorm.DB
}

func NewStudioInteractionRepository(db *gorm.DB) *StudioInteractionRepositoryImpl {
	return &StudioInteractionRepositoryImpl{DB: db}
}

func (r *StudioInteractionRepositoryImpl) UpsertRating(userID, studioID uint, rating float64) error {
	record := UserStudioRating{
		UserID:   userID,
		StudioID: studioID,
		Rating:   rating,
	}
	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "studio_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"rating", "updated_at"}),
	}).Create(&record).Error
}

func (r *StudioInteractionRepositoryImpl) DeleteRating(userID, studioID uint) error {
	return r.DB.Where("user_id = ? AND studio_id = ?", userID, studioID).Delete(&UserStudioRating{}).Error
}

func (r *StudioInteractionRepositoryImpl) GetRating(userID, studioID uint) (*UserStudioRating, error) {
	var rating UserStudioRating
	err := r.DB.Where("user_id = ? AND studio_id = ?", userID, studioID).First(&rating).Error
	if err != nil {
		return nil, err
	}
	return &rating, nil
}

func (r *StudioInteractionRepositoryImpl) SetLike(userID, studioID uint) error {
	like := UserStudioLike{
		UserID:   userID,
		StudioID: studioID,
	}
	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "studio_id"}},
		DoNothing: true,
	}).Create(&like).Error
}

func (r *StudioInteractionRepositoryImpl) DeleteLike(userID, studioID uint) error {
	return r.DB.Where("user_id = ? AND studio_id = ?", userID, studioID).Delete(&UserStudioLike{}).Error
}

func (r *StudioInteractionRepositoryImpl) IsLiked(userID, studioID uint) (bool, error) {
	var count int64
	err := r.DB.Model(&UserStudioLike{}).Where("user_id = ? AND studio_id = ?", userID, studioID).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *StudioInteractionRepositoryImpl) GetAllInteractions(userID, studioID uint) (*StudioInteractions, error) {
	result := &StudioInteractions{}

	// Get rating
	var rating UserStudioRating
	err := r.DB.Where("user_id = ? AND studio_id = ?", userID, studioID).First(&rating).Error
	if err == nil {
		result.Rating = rating.Rating
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// Check if liked
	var likeCount int64
	err = r.DB.Model(&UserStudioLike{}).Where("user_id = ? AND studio_id = ?", userID, studioID).Count(&likeCount).Error
	if err != nil {
		return nil, err
	}
	result.Liked = likeCount > 0

	return result, nil
}

func (r *StudioInteractionRepositoryImpl) GetLikedStudioIDs(userID uint) ([]uint, error) {
	var ids []uint
	err := r.DB.Model(&UserStudioLike{}).Where("user_id = ?", userID).Pluck("studio_id", &ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// Ensure StudioInteractionRepositoryImpl implements StudioInteractionRepository
var _ StudioInteractionRepository = (*StudioInteractionRepositoryImpl)(nil)
