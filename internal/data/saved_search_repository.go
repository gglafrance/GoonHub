package data

import (
	"gorm.io/gorm"
)

type SavedSearchRepository interface {
	Create(search *SavedSearch) error
	GetByID(id uint) (*SavedSearch, error)
	GetByUUID(uuid string) (*SavedSearch, error)
	Update(search *SavedSearch) error
	Delete(id uint) error
	ListByUserID(userID uint) ([]SavedSearch, error)
}

type SavedSearchRepositoryImpl struct {
	DB *gorm.DB
}

func NewSavedSearchRepository(db *gorm.DB) *SavedSearchRepositoryImpl {
	return &SavedSearchRepositoryImpl{DB: db}
}

func (r *SavedSearchRepositoryImpl) Create(search *SavedSearch) error {
	return r.DB.Create(search).Error
}

func (r *SavedSearchRepositoryImpl) GetByID(id uint) (*SavedSearch, error) {
	var search SavedSearch
	if err := r.DB.First(&search, id).Error; err != nil {
		return nil, err
	}
	return &search, nil
}

func (r *SavedSearchRepositoryImpl) GetByUUID(uuid string) (*SavedSearch, error) {
	var search SavedSearch
	if err := r.DB.Where("uuid = ?", uuid).First(&search).Error; err != nil {
		return nil, err
	}
	return &search, nil
}

func (r *SavedSearchRepositoryImpl) Update(search *SavedSearch) error {
	return r.DB.Save(search).Error
}

func (r *SavedSearchRepositoryImpl) Delete(id uint) error {
	result := r.DB.Delete(&SavedSearch{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *SavedSearchRepositoryImpl) ListByUserID(userID uint) ([]SavedSearch, error) {
	var searches []SavedSearch
	err := r.DB.
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&searches).Error
	if err != nil {
		return nil, err
	}
	return searches, nil
}

// Ensure SavedSearchRepositoryImpl implements SavedSearchRepository
var _ SavedSearchRepository = (*SavedSearchRepositoryImpl)(nil)
