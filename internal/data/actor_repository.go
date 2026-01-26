package data

import (
	"gorm.io/gorm"
)

type ActorRepository interface {
	Create(actor *Actor) error
	GetByID(id uint) (*Actor, error)
	GetByUUID(uuid string) (*Actor, error)
	Update(actor *Actor) error
	Delete(id uint) error
	List(page, limit int) ([]ActorWithCount, int64, error)
	Search(query string, page, limit int) ([]ActorWithCount, int64, error)

	// Video associations
	GetVideoActors(videoID uint) ([]Actor, error)
	SetVideoActors(videoID uint, actorIDs []uint) error
	GetActorVideos(actorID uint, page, limit int) ([]Video, int64, error)
	GetVideoCount(actorID uint) (int64, error)
}

type ActorRepositoryImpl struct {
	DB *gorm.DB
}

func NewActorRepository(db *gorm.DB) *ActorRepositoryImpl {
	return &ActorRepositoryImpl{DB: db}
}

func (r *ActorRepositoryImpl) Create(actor *Actor) error {
	return r.DB.Create(actor).Error
}

func (r *ActorRepositoryImpl) GetByID(id uint) (*Actor, error) {
	var actor Actor
	if err := r.DB.First(&actor, id).Error; err != nil {
		return nil, err
	}
	return &actor, nil
}

func (r *ActorRepositoryImpl) GetByUUID(uuid string) (*Actor, error) {
	var actor Actor
	if err := r.DB.Where("uuid = ?", uuid).First(&actor).Error; err != nil {
		return nil, err
	}
	return &actor, nil
}

func (r *ActorRepositoryImpl) Update(actor *Actor) error {
	return r.DB.Save(actor).Error
}

func (r *ActorRepositoryImpl) Delete(id uint) error {
	result := r.DB.Delete(&Actor{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *ActorRepositoryImpl) List(page, limit int) ([]ActorWithCount, int64, error) {
	var actors []ActorWithCount
	var total int64

	offset := (page - 1) * limit

	if err := r.DB.Model(&Actor{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := r.DB.
		Table("actors").
		Select("actors.*, COALESCE(COUNT(video_actors.id), 0) as video_count").
		Joins("LEFT JOIN video_actors ON video_actors.actor_id = actors.id").
		Where("actors.deleted_at IS NULL").
		Group("actors.id").
		Order("actors.name ASC").
		Limit(limit).
		Offset(offset).
		Find(&actors).Error
	if err != nil {
		return nil, 0, err
	}

	return actors, total, nil
}

func (r *ActorRepositoryImpl) Search(query string, page, limit int) ([]ActorWithCount, int64, error) {
	var actors []ActorWithCount
	var total int64

	offset := (page - 1) * limit
	searchPattern := "%" + query + "%"

	countQuery := r.DB.Model(&Actor{}).Where("name ILIKE ?", searchPattern)
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := r.DB.
		Table("actors").
		Select("actors.*, COALESCE(COUNT(video_actors.id), 0) as video_count").
		Joins("LEFT JOIN video_actors ON video_actors.actor_id = actors.id").
		Where("actors.deleted_at IS NULL").
		Where("actors.name ILIKE ?", searchPattern).
		Group("actors.id").
		Order("actors.name ASC").
		Limit(limit).
		Offset(offset).
		Find(&actors).Error
	if err != nil {
		return nil, 0, err
	}

	return actors, total, nil
}

func (r *ActorRepositoryImpl) GetVideoActors(videoID uint) ([]Actor, error) {
	var actors []Actor
	err := r.DB.
		Joins("JOIN video_actors ON video_actors.actor_id = actors.id").
		Where("video_actors.video_id = ?", videoID).
		Where("actors.deleted_at IS NULL").
		Order("actors.name ASC").
		Find(&actors).Error
	if err != nil {
		return nil, err
	}
	return actors, nil
}

func (r *ActorRepositoryImpl) SetVideoActors(videoID uint, actorIDs []uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("video_id = ?", videoID).Delete(&VideoActor{}).Error; err != nil {
			return err
		}

		if len(actorIDs) == 0 {
			return nil
		}

		videoActors := make([]VideoActor, len(actorIDs))
		for i, actorID := range actorIDs {
			videoActors[i] = VideoActor{
				VideoID: videoID,
				ActorID: actorID,
			}
		}

		return tx.Create(&videoActors).Error
	})
}

func (r *ActorRepositoryImpl) GetActorVideos(actorID uint, page, limit int) ([]Video, int64, error) {
	var videos []Video
	var total int64

	offset := (page - 1) * limit

	countQuery := r.DB.
		Model(&Video{}).
		Joins("JOIN video_actors ON video_actors.video_id = videos.id").
		Where("video_actors.actor_id = ?", actorID).
		Where("videos.deleted_at IS NULL")
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := r.DB.
		Joins("JOIN video_actors ON video_actors.video_id = videos.id").
		Where("video_actors.actor_id = ?", actorID).
		Where("videos.deleted_at IS NULL").
		Order("videos.created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&videos).Error
	if err != nil {
		return nil, 0, err
	}

	return videos, total, nil
}

func (r *ActorRepositoryImpl) GetVideoCount(actorID uint) (int64, error) {
	var count int64
	err := r.DB.
		Model(&VideoActor{}).
		Joins("JOIN videos ON videos.id = video_actors.video_id").
		Where("video_actors.actor_id = ?", actorID).
		Where("videos.deleted_at IS NULL").
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}
