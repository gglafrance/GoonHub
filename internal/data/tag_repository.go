package data

import "gorm.io/gorm"

type TagWithCount struct {
	Tag
	VideoCount int64 `json:"video_count"`
}

type TagRepository interface {
	List() ([]Tag, error)
	ListWithCounts() ([]TagWithCount, error)
	GetByID(id uint) (*Tag, error)
	Create(tag *Tag) error
	Delete(id uint) error
	GetVideoTags(videoID uint) ([]Tag, error)
	SetVideoTags(videoID uint, tagIDs []uint) error
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
		Select("tags.*, COALESCE(COUNT(video_tags.id), 0) as video_count").
		Joins("LEFT JOIN video_tags ON video_tags.tag_id = tags.id").
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

func (r *TagRepositoryImpl) GetVideoTags(videoID uint) ([]Tag, error) {
	var tags []Tag
	err := r.DB.
		Joins("JOIN video_tags ON video_tags.tag_id = tags.id").
		Where("video_tags.video_id = ?", videoID).
		Order("tags.name asc").
		Find(&tags).Error
	if err != nil {
		return nil, err
	}
	return tags, nil
}

func (r *TagRepositoryImpl) SetVideoTags(videoID uint, tagIDs []uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("video_id = ?", videoID).Delete(&VideoTag{}).Error; err != nil {
			return err
		}

		if len(tagIDs) == 0 {
			return nil
		}

		videoTags := make([]VideoTag, len(tagIDs))
		for i, tagID := range tagIDs {
			videoTags[i] = VideoTag{
				VideoID: videoID,
				TagID:   tagID,
			}
		}

		return tx.Create(&videoTags).Error
	})
}
