package data

import "gorm.io/gorm"

type ShareLinkRepository interface {
	Create(link *ShareLink) error
	GetByToken(token string) (*ShareLink, error)
	ListBySceneAndUser(sceneID, userID uint) ([]ShareLink, error)
	Delete(id, userID uint) error
	IncrementViewCount(id uint) error
}

type ShareLinkRepositoryImpl struct {
	DB *gorm.DB
}

func NewShareLinkRepository(db *gorm.DB) *ShareLinkRepositoryImpl {
	return &ShareLinkRepositoryImpl{DB: db}
}

func (r *ShareLinkRepositoryImpl) Create(link *ShareLink) error {
	return r.DB.Create(link).Error
}

func (r *ShareLinkRepositoryImpl) GetByToken(token string) (*ShareLink, error) {
	var link ShareLink
	if err := r.DB.Where("token = ?", token).First(&link).Error; err != nil {
		return nil, err
	}
	return &link, nil
}

func (r *ShareLinkRepositoryImpl) ListBySceneAndUser(sceneID, userID uint) ([]ShareLink, error) {
	var links []ShareLink
	err := r.DB.Where("scene_id = ? AND user_id = ?", sceneID, userID).
		Order("created_at DESC").
		Find(&links).Error
	if err != nil {
		return nil, err
	}
	return links, nil
}

func (r *ShareLinkRepositoryImpl) Delete(id, userID uint) error {
	result := r.DB.Where("id = ? AND user_id = ?", id, userID).Delete(&ShareLink{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *ShareLinkRepositoryImpl) IncrementViewCount(id uint) error {
	return r.DB.Model(&ShareLink{}).Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error
}
