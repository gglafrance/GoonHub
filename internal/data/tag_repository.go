package data

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TagWithCount struct {
	Tag
	VideoCount int64 `json:"video_count"`
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
	GetVideoTags(videoID uint) ([]Tag, error)
	GetVideoTagsMultiple(videoIDs []uint) (map[uint][]Tag, error)
	SetVideoTags(videoID uint, tagIDs []uint) error
	GetVideoIDsByTag(tagID uint, limit int) ([]uint, error)

	// Bulk operations
	BulkAddTagsToVideos(videoIDs []uint, tagIDs []uint) error
	BulkRemoveTagsFromVideos(videoIDs []uint, tagIDs []uint) error
	BulkReplaceTagsForVideos(videoIDs []uint, tagIDs []uint) error
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
		Select("tags.*, COALESCE(COUNT(videos.id), 0) as video_count").
		Joins("LEFT JOIN video_tags ON video_tags.tag_id = tags.id").
		Joins("LEFT JOIN videos ON videos.id = video_tags.video_id AND videos.deleted_at IS NULL").
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

// GetVideoTagsMultiple returns tags for multiple videos in a single query
func (r *TagRepositoryImpl) GetVideoTagsMultiple(videoIDs []uint) (map[uint][]Tag, error) {
	if len(videoIDs) == 0 {
		return make(map[uint][]Tag), nil
	}

	// Query all video_tags for the given videos with their tags
	type videoTagResult struct {
		VideoID uint
		Tag
	}

	var results []videoTagResult
	err := r.DB.
		Table("video_tags").
		Select("video_tags.video_id, tags.*").
		Joins("JOIN tags ON tags.id = video_tags.tag_id").
		Where("video_tags.video_id IN ?", videoIDs).
		Order("tags.name asc").
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	// Group by video ID
	tagsByVideo := make(map[uint][]Tag)
	for _, videoID := range videoIDs {
		tagsByVideo[videoID] = []Tag{} // Initialize all requested videos
	}
	for _, r := range results {
		tagsByVideo[r.VideoID] = append(tagsByVideo[r.VideoID], r.Tag)
	}

	return tagsByVideo, nil
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

// GetVideoIDsByTag returns video IDs that have the given tag.
func (r *TagRepositoryImpl) GetVideoIDsByTag(tagID uint, limit int) ([]uint, error) {
	var videoIDs []uint
	err := r.DB.
		Table("video_tags").
		Select("video_tags.video_id").
		Joins("JOIN videos ON videos.id = video_tags.video_id").
		Where("video_tags.tag_id = ?", tagID).
		Where("videos.deleted_at IS NULL").
		Order("videos.created_at DESC").
		Limit(limit).
		Pluck("video_id", &videoIDs).Error
	if err != nil {
		return nil, err
	}
	return videoIDs, nil
}

// BulkAddTagsToVideos adds tags to multiple videos (skips existing associations)
func (r *TagRepositoryImpl) BulkAddTagsToVideos(videoIDs []uint, tagIDs []uint) error {
	if len(videoIDs) == 0 || len(tagIDs) == 0 {
		return nil
	}

	return r.DB.Transaction(func(tx *gorm.DB) error {
		videoTags := make([]VideoTag, 0, len(videoIDs)*len(tagIDs))
		for _, videoID := range videoIDs {
			for _, tagID := range tagIDs {
				videoTags = append(videoTags, VideoTag{
					VideoID: videoID,
					TagID:   tagID,
				})
			}
		}

		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&videoTags).Error
	})
}

// BulkRemoveTagsFromVideos removes specific tags from multiple videos
func (r *TagRepositoryImpl) BulkRemoveTagsFromVideos(videoIDs []uint, tagIDs []uint) error {
	if len(videoIDs) == 0 || len(tagIDs) == 0 {
		return nil
	}

	return r.DB.
		Where("video_id IN ?", videoIDs).
		Where("tag_id IN ?", tagIDs).
		Delete(&VideoTag{}).Error
}

// BulkReplaceTagsForVideos replaces all tags for multiple videos
func (r *TagRepositoryImpl) BulkReplaceTagsForVideos(videoIDs []uint, tagIDs []uint) error {
	if len(videoIDs) == 0 {
		return nil
	}

	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("video_id IN ?", videoIDs).Delete(&VideoTag{}).Error; err != nil {
			return err
		}

		if len(tagIDs) == 0 {
			return nil
		}

		videoTags := make([]VideoTag, 0, len(videoIDs)*len(tagIDs))
		for _, videoID := range videoIDs {
			for _, tagID := range tagIDs {
				videoTags = append(videoTags, VideoTag{
					VideoID: videoID,
					TagID:   tagID,
				})
			}
		}

		return tx.Create(&videoTags).Error
	})
}
