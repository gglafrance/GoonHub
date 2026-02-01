package data

import (
	"gorm.io/gorm"
)

type StudioRepository interface {
	Create(studio *Studio) error
	GetByID(id uint) (*Studio, error)
	GetByUUID(uuid string) (*Studio, error)
	GetByName(name string) (*Studio, error)
	Update(studio *Studio) error
	Delete(id uint) error
	List(page, limit int) ([]StudioWithCount, int64, error)
	Search(query string, page, limit int) ([]StudioWithCount, int64, error)

	// Scene associations (one-to-many: scene has one studio)
	GetSceneStudio(sceneID uint) (*Studio, error)
	SetSceneStudio(sceneID uint, studioID *uint) error
	GetStudioScenes(studioID uint, page, limit int) ([]Scene, int64, error)
	GetSceneCount(studioID uint) (int64, error)

	// Bulk operations
	BulkSetStudioForScenes(sceneIDs []uint, studioID *uint) error
}

type StudioRepositoryImpl struct {
	DB *gorm.DB
}

func NewStudioRepository(db *gorm.DB) *StudioRepositoryImpl {
	return &StudioRepositoryImpl{DB: db}
}

func (r *StudioRepositoryImpl) Create(studio *Studio) error {
	return r.DB.Create(studio).Error
}

func (r *StudioRepositoryImpl) GetByID(id uint) (*Studio, error) {
	var studio Studio
	if err := r.DB.First(&studio, id).Error; err != nil {
		return nil, err
	}
	return &studio, nil
}

func (r *StudioRepositoryImpl) GetByUUID(uuid string) (*Studio, error) {
	var studio Studio
	if err := r.DB.Where("uuid = ?", uuid).First(&studio).Error; err != nil {
		return nil, err
	}
	return &studio, nil
}

func (r *StudioRepositoryImpl) GetByName(name string) (*Studio, error) {
	var studio Studio
	if err := r.DB.Where("name = ? AND deleted_at IS NULL", name).First(&studio).Error; err != nil {
		return nil, err
	}
	return &studio, nil
}

func (r *StudioRepositoryImpl) Update(studio *Studio) error {
	return r.DB.Save(studio).Error
}

func (r *StudioRepositoryImpl) Delete(id uint) error {
	result := r.DB.Delete(&Studio{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *StudioRepositoryImpl) List(page, limit int) ([]StudioWithCount, int64, error) {
	var studios []StudioWithCount
	var total int64

	offset := (page - 1) * limit

	if err := r.DB.Model(&Studio{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := r.DB.
		Table("studios").
		Select("studios.*, COALESCE(COUNT(scenes.id), 0) as scene_count").
		Joins("LEFT JOIN scenes ON scenes.studio_id = studios.id AND scenes.deleted_at IS NULL").
		Where("studios.deleted_at IS NULL").
		Group("studios.id").
		Order("studios.name ASC").
		Limit(limit).
		Offset(offset).
		Find(&studios).Error
	if err != nil {
		return nil, 0, err
	}

	return studios, total, nil
}

func (r *StudioRepositoryImpl) Search(query string, page, limit int) ([]StudioWithCount, int64, error) {
	var studios []StudioWithCount
	var total int64

	offset := (page - 1) * limit
	searchPattern := "%" + query + "%"

	countQuery := r.DB.Model(&Studio{}).Where("name ILIKE ?", searchPattern)
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := r.DB.
		Table("studios").
		Select("studios.*, COALESCE(COUNT(scenes.id), 0) as scene_count").
		Joins("LEFT JOIN scenes ON scenes.studio_id = studios.id AND scenes.deleted_at IS NULL").
		Where("studios.deleted_at IS NULL").
		Where("studios.name ILIKE ?", searchPattern).
		Group("studios.id").
		Order("studios.name ASC").
		Limit(limit).
		Offset(offset).
		Find(&studios).Error
	if err != nil {
		return nil, 0, err
	}

	return studios, total, nil
}

func (r *StudioRepositoryImpl) GetSceneStudio(sceneID uint) (*Studio, error) {
	var scene Scene
	if err := r.DB.Select("studio_id").First(&scene, sceneID).Error; err != nil {
		return nil, err
	}

	if scene.StudioID == nil {
		return nil, nil
	}

	var studio Studio
	if err := r.DB.First(&studio, *scene.StudioID).Error; err != nil {
		return nil, err
	}
	return &studio, nil
}

func (r *StudioRepositoryImpl) SetSceneStudio(sceneID uint, studioID *uint) error {
	return r.DB.Model(&Scene{}).Where("id = ?", sceneID).Update("studio_id", studioID).Error
}

func (r *StudioRepositoryImpl) GetStudioScenes(studioID uint, page, limit int) ([]Scene, int64, error) {
	var scenes []Scene
	var total int64

	offset := (page - 1) * limit

	countQuery := r.DB.
		Model(&Scene{}).
		Where("studio_id = ?", studioID).
		Where("deleted_at IS NULL")
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := r.DB.
		Where("studio_id = ?", studioID).
		Where("deleted_at IS NULL").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&scenes).Error
	if err != nil {
		return nil, 0, err
	}

	return scenes, total, nil
}

func (r *StudioRepositoryImpl) GetSceneCount(studioID uint) (int64, error) {
	var count int64
	err := r.DB.
		Model(&Scene{}).
		Where("studio_id = ?", studioID).
		Where("deleted_at IS NULL").
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

// BulkSetStudioForScenes sets the studio for multiple scenes
func (r *StudioRepositoryImpl) BulkSetStudioForScenes(sceneIDs []uint, studioID *uint) error {
	if len(sceneIDs) == 0 {
		return nil
	}

	return r.DB.Model(&Scene{}).Where("id IN ?", sceneIDs).Update("studio_id", studioID).Error
}

// Ensure StudioRepositoryImpl implements StudioRepository
var _ StudioRepository = (*StudioRepositoryImpl)(nil)
