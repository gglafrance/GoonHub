package data

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ActorRepository interface {
	Create(actor *Actor) error
	GetByID(id uint) (*Actor, error)
	GetByIDs(ids []uint) ([]Actor, error)
	GetByUUID(uuid string) (*Actor, error)
	Update(actor *Actor) error
	Delete(id uint) error
	List(page, limit int, sort string) ([]ActorWithCount, int64, error)
	Search(query string, page, limit int, sort string) ([]ActorWithCount, int64, error)

	// Video associations
	GetVideoActors(videoID uint) ([]Actor, error)
	GetVideoActorsMultiple(videoIDs []uint) (map[uint][]Actor, error)
	SetVideoActors(videoID uint, actorIDs []uint) error
	GetActorVideos(actorID uint, page, limit int) ([]Video, int64, error)
	GetVideoCount(actorID uint) (int64, error)

	// Bulk operations
	BulkAddActorsToVideos(videoIDs []uint, actorIDs []uint) error
	BulkRemoveActorsFromVideos(videoIDs []uint, actorIDs []uint) error
	BulkReplaceActorsForVideos(videoIDs []uint, actorIDs []uint) error
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

func (r *ActorRepositoryImpl) GetByIDs(ids []uint) ([]Actor, error) {
	if len(ids) == 0 {
		return []Actor{}, nil
	}
	var actors []Actor
	if err := r.DB.Where("id IN ?", ids).Find(&actors).Error; err != nil {
		return nil, err
	}
	return actors, nil
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

// actorSortMap maps sort parameter values to SQL ORDER BY clauses.
// This whitelist approach prevents SQL injection.
var actorSortMap = map[string]string{
	"name_asc":         "actors.name ASC",
	"name_desc":        "actors.name DESC",
	"video_count_asc":  "video_count ASC",
	"video_count_desc": "video_count DESC",
	"created_at_asc":   "actors.created_at ASC",
	"created_at_desc":  "actors.created_at DESC",
}

func getActorOrderClause(sort string) string {
	if clause, ok := actorSortMap[sort]; ok {
		return clause
	}
	return "actors.name ASC" // default sort
}

func (r *ActorRepositoryImpl) List(page, limit int, sort string) ([]ActorWithCount, int64, error) {
	var actors []ActorWithCount
	var total int64

	offset := (page - 1) * limit

	if err := r.DB.Model(&Actor{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	orderClause := getActorOrderClause(sort)

	err := r.DB.
		Table("actors").
		Select("actors.*, COALESCE(COUNT(videos.id), 0) as video_count").
		Joins("LEFT JOIN video_actors ON video_actors.actor_id = actors.id").
		Joins("LEFT JOIN videos ON videos.id = video_actors.video_id AND videos.deleted_at IS NULL").
		Where("actors.deleted_at IS NULL").
		Group("actors.id").
		Order(orderClause).
		Limit(limit).
		Offset(offset).
		Find(&actors).Error
	if err != nil {
		return nil, 0, err
	}

	return actors, total, nil
}

func (r *ActorRepositoryImpl) Search(query string, page, limit int, sort string) ([]ActorWithCount, int64, error) {
	var actors []ActorWithCount
	var total int64

	offset := (page - 1) * limit
	searchPattern := "%" + query + "%"

	countQuery := r.DB.Model(&Actor{}).Where("name ILIKE ?", searchPattern)
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	orderClause := getActorOrderClause(sort)

	err := r.DB.
		Table("actors").
		Select("actors.*, COALESCE(COUNT(video_actors.id), 0) as video_count").
		Joins("LEFT JOIN video_actors ON video_actors.actor_id = actors.id").
		Joins("LEFT JOIN videos ON videos.id = video_actors.video_id AND videos.deleted_at IS NULL").
		Where("actors.deleted_at IS NULL").
		Where("actors.name ILIKE ?", searchPattern).
		Group("actors.id").
		Order(orderClause).
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

// GetVideoActorsMultiple returns actors for multiple videos in a single query
func (r *ActorRepositoryImpl) GetVideoActorsMultiple(videoIDs []uint) (map[uint][]Actor, error) {
	if len(videoIDs) == 0 {
		return make(map[uint][]Actor), nil
	}

	// Query all video_actors for the given videos with their actors
	type videoActorResult struct {
		VideoID uint
		Actor
	}

	var results []videoActorResult
	err := r.DB.
		Table("video_actors").
		Select("video_actors.video_id, actors.*").
		Joins("JOIN actors ON actors.id = video_actors.actor_id").
		Where("video_actors.video_id IN ?", videoIDs).
		Where("actors.deleted_at IS NULL").
		Order("actors.name asc").
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	// Group by video ID
	actorsByVideo := make(map[uint][]Actor)
	for _, videoID := range videoIDs {
		actorsByVideo[videoID] = []Actor{}
	}
	for _, r := range results {
		actorsByVideo[r.VideoID] = append(actorsByVideo[r.VideoID], r.Actor)
	}

	return actorsByVideo, nil
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

// BulkAddActorsToVideos adds actors to multiple videos (skips existing associations)
func (r *ActorRepositoryImpl) BulkAddActorsToVideos(videoIDs []uint, actorIDs []uint) error {
	if len(videoIDs) == 0 || len(actorIDs) == 0 {
		return nil
	}

	return r.DB.Transaction(func(tx *gorm.DB) error {
		// Build all associations
		videoActors := make([]VideoActor, 0, len(videoIDs)*len(actorIDs))
		for _, videoID := range videoIDs {
			for _, actorID := range actorIDs {
				videoActors = append(videoActors, VideoActor{
					VideoID: videoID,
					ActorID: actorID,
				})
			}
		}

		// Insert with ON CONFLICT DO NOTHING to skip existing
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&videoActors).Error
	})
}

// BulkRemoveActorsFromVideos removes specific actors from multiple videos
func (r *ActorRepositoryImpl) BulkRemoveActorsFromVideos(videoIDs []uint, actorIDs []uint) error {
	if len(videoIDs) == 0 || len(actorIDs) == 0 {
		return nil
	}

	return r.DB.
		Where("video_id IN ?", videoIDs).
		Where("actor_id IN ?", actorIDs).
		Delete(&VideoActor{}).Error
}

// BulkReplaceActorsForVideos replaces all actors for multiple videos
func (r *ActorRepositoryImpl) BulkReplaceActorsForVideos(videoIDs []uint, actorIDs []uint) error {
	if len(videoIDs) == 0 {
		return nil
	}

	return r.DB.Transaction(func(tx *gorm.DB) error {
		// Delete all existing associations for these videos
		if err := tx.Where("video_id IN ?", videoIDs).Delete(&VideoActor{}).Error; err != nil {
			return err
		}

		if len(actorIDs) == 0 {
			return nil
		}

		// Build all new associations
		videoActors := make([]VideoActor, 0, len(videoIDs)*len(actorIDs))
		for _, videoID := range videoIDs {
			for _, actorID := range actorIDs {
				videoActors = append(videoActors, VideoActor{
					VideoID: videoID,
					ActorID: actorID,
				})
			}
		}

		return tx.Create(&videoActors).Error
	})
}
