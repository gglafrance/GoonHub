package data

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ActorInteractionRepository interface {
	UpsertRating(userID, actorID uint, rating float64) error
	DeleteRating(userID, actorID uint) error
	GetRating(userID, actorID uint) (*UserActorRating, error)
	SetLike(userID, actorID uint) error
	DeleteLike(userID, actorID uint) error
	IsLiked(userID, actorID uint) (bool, error)
	GetAllInteractions(userID, actorID uint) (*ActorInteractions, error)
	GetLikedActorIDs(userID uint) ([]uint, error)
}

type ActorInteractionRepositoryImpl struct {
	DB *gorm.DB
}

func NewActorInteractionRepository(db *gorm.DB) *ActorInteractionRepositoryImpl {
	return &ActorInteractionRepositoryImpl{DB: db}
}

func (r *ActorInteractionRepositoryImpl) UpsertRating(userID, actorID uint, rating float64) error {
	record := UserActorRating{
		UserID:  userID,
		ActorID: actorID,
		Rating:  rating,
	}
	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "actor_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"rating", "updated_at"}),
	}).Create(&record).Error
}

func (r *ActorInteractionRepositoryImpl) DeleteRating(userID, actorID uint) error {
	return r.DB.Where("user_id = ? AND actor_id = ?", userID, actorID).Delete(&UserActorRating{}).Error
}

func (r *ActorInteractionRepositoryImpl) GetRating(userID, actorID uint) (*UserActorRating, error) {
	var rating UserActorRating
	err := r.DB.Where("user_id = ? AND actor_id = ?", userID, actorID).First(&rating).Error
	if err != nil {
		return nil, err
	}
	return &rating, nil
}

func (r *ActorInteractionRepositoryImpl) SetLike(userID, actorID uint) error {
	like := UserActorLike{
		UserID:  userID,
		ActorID: actorID,
	}
	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "actor_id"}},
		DoNothing: true,
	}).Create(&like).Error
}

func (r *ActorInteractionRepositoryImpl) DeleteLike(userID, actorID uint) error {
	return r.DB.Where("user_id = ? AND actor_id = ?", userID, actorID).Delete(&UserActorLike{}).Error
}

func (r *ActorInteractionRepositoryImpl) IsLiked(userID, actorID uint) (bool, error) {
	var count int64
	err := r.DB.Model(&UserActorLike{}).Where("user_id = ? AND actor_id = ?", userID, actorID).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *ActorInteractionRepositoryImpl) GetAllInteractions(userID, actorID uint) (*ActorInteractions, error) {
	result := &ActorInteractions{}

	// Get rating
	var rating UserActorRating
	err := r.DB.Where("user_id = ? AND actor_id = ?", userID, actorID).First(&rating).Error
	if err == nil {
		result.Rating = rating.Rating
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// Check if liked
	var likeCount int64
	err = r.DB.Model(&UserActorLike{}).Where("user_id = ? AND actor_id = ?", userID, actorID).Count(&likeCount).Error
	if err != nil {
		return nil, err
	}
	result.Liked = likeCount > 0

	return result, nil
}

func (r *ActorInteractionRepositoryImpl) GetLikedActorIDs(userID uint) ([]uint, error) {
	var ids []uint
	err := r.DB.Model(&UserActorLike{}).Where("user_id = ?", userID).Pluck("actor_id", &ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// Ensure ActorInteractionRepositoryImpl implements ActorInteractionRepository
var _ ActorInteractionRepository = (*ActorInteractionRepositoryImpl)(nil)
