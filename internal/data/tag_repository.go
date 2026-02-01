package data

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TagWithCount struct {
	Tag
	SceneCount int64 `json:"scene_count"`
}

type TagRepository interface {
	List() ([]Tag, error)
	ListWithCounts() ([]TagWithCount, error)
	GetByID(id uint) (*Tag, error)
	GetByIDs(ids []uint) ([]Tag, error)
	GetByNames(names []string) ([]Tag, error)
	GetIDsByNames(names []string) ([]uint, error)
	Create(tag *Tag) error
	Delete(id uint) error
	GetSceneTags(sceneID uint) ([]Tag, error)
	GetSceneTagsMultiple(sceneIDs []uint) (map[uint][]Tag, error)
	SetSceneTags(sceneID uint, tagIDs []uint) error
	GetSceneIDsByTag(tagID uint, limit int) ([]uint, error)

	// Bulk operations
	BulkAddTagsToScenes(sceneIDs []uint, tagIDs []uint) error
	BulkRemoveTagsFromScenes(sceneIDs []uint, tagIDs []uint) error
	BulkReplaceTagsForScenes(sceneIDs []uint, tagIDs []uint) error
}

type TagRepositoryImpl struct {
	DB *gorm.DB
}

func NewTagRepository(db *gorm.DB) *TagRepositoryImpl {
	return &TagRepositoryImpl{DB: db}
}

func (r *TagRepositoryImpl) List() ([]Tag, error) {
	var tags []Tag
	if err := r.DB.Order("name asc").Find(&tags).Error; err != nil {
		return nil, err
	}
	return tags, nil
}

func (r *TagRepositoryImpl) ListWithCounts() ([]TagWithCount, error) {
	var tags []TagWithCount
	err := r.DB.
		Table("tags").
		Select("tags.*, COALESCE(COUNT(scenes.id), 0) as scene_count").
		Joins("LEFT JOIN scene_tags ON scene_tags.tag_id = tags.id").
		Joins("LEFT JOIN scenes ON scenes.id = scene_tags.scene_id AND scenes.deleted_at IS NULL").
		Group("tags.id").
		Order("tags.name asc").
		Find(&tags).Error
	if err != nil {
		return nil, err
	}
	return tags, nil
}

func (r *TagRepositoryImpl) GetByID(id uint) (*Tag, error) {
	var tag Tag
	if err := r.DB.First(&tag, id).Error; err != nil {
		return nil, err
	}
	return &tag, nil
}

func (r *TagRepositoryImpl) GetByIDs(ids []uint) ([]Tag, error) {
	if len(ids) == 0 {
		return []Tag{}, nil
	}
	var tags []Tag
	if err := r.DB.Where("id IN ?", ids).Find(&tags).Error; err != nil {
		return nil, err
	}
	return tags, nil
}

func (r *TagRepositoryImpl) GetByNames(names []string) ([]Tag, error) {
	var tags []Tag
	if len(names) == 0 {
		return tags, nil
	}
	if err := r.DB.Where("name IN ?", names).Find(&tags).Error; err != nil {
		return nil, err
	}
	return tags, nil
}

func (r *TagRepositoryImpl) GetIDsByNames(names []string) ([]uint, error) {
	if len(names) == 0 {
		return []uint{}, nil
	}
	var ids []uint
	if err := r.DB.Model(&Tag{}).Where("name IN ?", names).Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *TagRepositoryImpl) Create(tag *Tag) error {
	return r.DB.Create(tag).Error
}

func (r *TagRepositoryImpl) Delete(id uint) error {
	result := r.DB.Delete(&Tag{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *TagRepositoryImpl) GetSceneTags(sceneID uint) ([]Tag, error) {
	var tags []Tag
	err := r.DB.
		Joins("JOIN scene_tags ON scene_tags.tag_id = tags.id").
		Where("scene_tags.scene_id = ?", sceneID).
		Order("tags.name asc").
		Find(&tags).Error
	if err != nil {
		return nil, err
	}
	return tags, nil
}

// GetSceneTagsMultiple returns tags for multiple scenes in a single query
func (r *TagRepositoryImpl) GetSceneTagsMultiple(sceneIDs []uint) (map[uint][]Tag, error) {
	if len(sceneIDs) == 0 {
		return make(map[uint][]Tag), nil
	}

	// Query all scene_tags for the given scenes with their tags
	type sceneTagResult struct {
		SceneID uint
		Tag
	}

	var results []sceneTagResult
	err := r.DB.
		Table("scene_tags").
		Select("scene_tags.scene_id, tags.*").
		Joins("JOIN tags ON tags.id = scene_tags.tag_id").
		Where("scene_tags.scene_id IN ?", sceneIDs).
		Order("tags.name asc").
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	// Group by scene ID
	tagsByScene := make(map[uint][]Tag)
	for _, sceneID := range sceneIDs {
		tagsByScene[sceneID] = []Tag{} // Initialize all requested scenes
	}
	for _, r := range results {
		tagsByScene[r.SceneID] = append(tagsByScene[r.SceneID], r.Tag)
	}

	return tagsByScene, nil
}

func (r *TagRepositoryImpl) SetSceneTags(sceneID uint, tagIDs []uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("scene_id = ?", sceneID).Delete(&SceneTag{}).Error; err != nil {
			return err
		}

		if len(tagIDs) == 0 {
			return nil
		}

		sceneTags := make([]SceneTag, len(tagIDs))
		for i, tagID := range tagIDs {
			sceneTags[i] = SceneTag{
				SceneID: sceneID,
				TagID:   tagID,
			}
		}

		return tx.Create(&sceneTags).Error
	})
}

// GetSceneIDsByTag returns scene IDs that have the given tag.
func (r *TagRepositoryImpl) GetSceneIDsByTag(tagID uint, limit int) ([]uint, error) {
	var sceneIDs []uint
	err := r.DB.
		Table("scene_tags").
		Select("scene_tags.scene_id").
		Joins("JOIN scenes ON scenes.id = scene_tags.scene_id").
		Where("scene_tags.tag_id = ?", tagID).
		Where("scenes.deleted_at IS NULL").
		Order("scenes.created_at DESC").
		Limit(limit).
		Pluck("scene_id", &sceneIDs).Error
	if err != nil {
		return nil, err
	}
	return sceneIDs, nil
}

// BulkAddTagsToScenes adds tags to multiple scenes (skips existing associations)
func (r *TagRepositoryImpl) BulkAddTagsToScenes(sceneIDs []uint, tagIDs []uint) error {
	if len(sceneIDs) == 0 || len(tagIDs) == 0 {
		return nil
	}

	return r.DB.Transaction(func(tx *gorm.DB) error {
		sceneTags := make([]SceneTag, 0, len(sceneIDs)*len(tagIDs))
		for _, sceneID := range sceneIDs {
			for _, tagID := range tagIDs {
				sceneTags = append(sceneTags, SceneTag{
					SceneID: sceneID,
					TagID:   tagID,
				})
			}
		}

		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&sceneTags).Error
	})
}

// BulkRemoveTagsFromScenes removes specific tags from multiple scenes
func (r *TagRepositoryImpl) BulkRemoveTagsFromScenes(sceneIDs []uint, tagIDs []uint) error {
	if len(sceneIDs) == 0 || len(tagIDs) == 0 {
		return nil
	}

	return r.DB.
		Where("scene_id IN ?", sceneIDs).
		Where("tag_id IN ?", tagIDs).
		Delete(&SceneTag{}).Error
}

// BulkReplaceTagsForScenes replaces all tags for multiple scenes
func (r *TagRepositoryImpl) BulkReplaceTagsForScenes(sceneIDs []uint, tagIDs []uint) error {
	if len(sceneIDs) == 0 {
		return nil
	}

	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("scene_id IN ?", sceneIDs).Delete(&SceneTag{}).Error; err != nil {
			return err
		}

		if len(tagIDs) == 0 {
			return nil
		}

		sceneTags := make([]SceneTag, 0, len(sceneIDs)*len(tagIDs))
		for _, sceneID := range sceneIDs {
			for _, tagID := range tagIDs {
				sceneTags = append(sceneTags, SceneTag{
					SceneID: sceneID,
					TagID:   tagID,
				})
			}
		}

		return tx.Create(&sceneTags).Error
	})
}
