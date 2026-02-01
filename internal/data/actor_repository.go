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

	// Scene associations
	GetSceneActors(sceneID uint) ([]Actor, error)
	GetSceneActorsMultiple(sceneIDs []uint) (map[uint][]Actor, error)
	SetSceneActors(sceneID uint, actorIDs []uint) error
	GetActorScenes(actorID uint, page, limit int) ([]Scene, int64, error)
	GetSceneCount(actorID uint) (int64, error)

	// Bulk operations
	BulkAddActorsToScenes(sceneIDs []uint, actorIDs []uint) error
	BulkRemoveActorsFromScenes(sceneIDs []uint, actorIDs []uint) error
	BulkReplaceActorsForScenes(sceneIDs []uint, actorIDs []uint) error
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
	"scene_count_asc":  "scene_count ASC",
	"scene_count_desc": "scene_count DESC",
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
		Select("actors.*, COALESCE(COUNT(scenes.id), 0) as scene_count").
		Joins("LEFT JOIN scene_actors ON scene_actors.actor_id = actors.id").
		Joins("LEFT JOIN scenes ON scenes.id = scene_actors.scene_id AND scenes.deleted_at IS NULL").
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
		Select("actors.*, COALESCE(COUNT(scene_actors.id), 0) as scene_count").
		Joins("LEFT JOIN scene_actors ON scene_actors.actor_id = actors.id").
		Joins("LEFT JOIN scenes ON scenes.id = scene_actors.scene_id AND scenes.deleted_at IS NULL").
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

func (r *ActorRepositoryImpl) GetSceneActors(sceneID uint) ([]Actor, error) {
	var actors []Actor
	err := r.DB.
		Joins("JOIN scene_actors ON scene_actors.actor_id = actors.id").
		Where("scene_actors.scene_id = ?", sceneID).
		Where("actors.deleted_at IS NULL").
		Order("actors.name ASC").
		Find(&actors).Error
	if err != nil {
		return nil, err
	}
	return actors, nil
}

// GetSceneActorsMultiple returns actors for multiple scenes in a single query
func (r *ActorRepositoryImpl) GetSceneActorsMultiple(sceneIDs []uint) (map[uint][]Actor, error) {
	if len(sceneIDs) == 0 {
		return make(map[uint][]Actor), nil
	}

	// Query all scene_actors for the given scenes with their actors
	type sceneActorResult struct {
		SceneID uint
		Actor
	}

	var results []sceneActorResult
	err := r.DB.
		Table("scene_actors").
		Select("scene_actors.scene_id, actors.*").
		Joins("JOIN actors ON actors.id = scene_actors.actor_id").
		Where("scene_actors.scene_id IN ?", sceneIDs).
		Where("actors.deleted_at IS NULL").
		Order("actors.name asc").
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	// Group by scene ID
	actorsByScene := make(map[uint][]Actor)
	for _, sceneID := range sceneIDs {
		actorsByScene[sceneID] = []Actor{}
	}
	for _, r := range results {
		actorsByScene[r.SceneID] = append(actorsByScene[r.SceneID], r.Actor)
	}

	return actorsByScene, nil
}

func (r *ActorRepositoryImpl) SetSceneActors(sceneID uint, actorIDs []uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("scene_id = ?", sceneID).Delete(&SceneActor{}).Error; err != nil {
			return err
		}

		if len(actorIDs) == 0 {
			return nil
		}

		sceneActors := make([]SceneActor, len(actorIDs))
		for i, actorID := range actorIDs {
			sceneActors[i] = SceneActor{
				SceneID: sceneID,
				ActorID: actorID,
			}
		}

		return tx.Create(&sceneActors).Error
	})
}

func (r *ActorRepositoryImpl) GetActorScenes(actorID uint, page, limit int) ([]Scene, int64, error) {
	var scenes []Scene
	var total int64

	offset := (page - 1) * limit

	countQuery := r.DB.
		Model(&Scene{}).
		Joins("JOIN scene_actors ON scene_actors.scene_id = scenes.id").
		Where("scene_actors.actor_id = ?", actorID).
		Where("scenes.deleted_at IS NULL")
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := r.DB.
		Joins("JOIN scene_actors ON scene_actors.scene_id = scenes.id").
		Where("scene_actors.actor_id = ?", actorID).
		Where("scenes.deleted_at IS NULL").
		Order("scenes.created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&scenes).Error
	if err != nil {
		return nil, 0, err
	}

	return scenes, total, nil
}

func (r *ActorRepositoryImpl) GetSceneCount(actorID uint) (int64, error) {
	var count int64
	err := r.DB.
		Model(&SceneActor{}).
		Joins("JOIN scenes ON scenes.id = scene_actors.scene_id").
		Where("scene_actors.actor_id = ?", actorID).
		Where("scenes.deleted_at IS NULL").
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

// BulkAddActorsToScenes adds actors to multiple scenes (skips existing associations)
func (r *ActorRepositoryImpl) BulkAddActorsToScenes(sceneIDs []uint, actorIDs []uint) error {
	if len(sceneIDs) == 0 || len(actorIDs) == 0 {
		return nil
	}

	return r.DB.Transaction(func(tx *gorm.DB) error {
		// Build all associations
		sceneActors := make([]SceneActor, 0, len(sceneIDs)*len(actorIDs))
		for _, sceneID := range sceneIDs {
			for _, actorID := range actorIDs {
				sceneActors = append(sceneActors, SceneActor{
					SceneID: sceneID,
					ActorID: actorID,
				})
			}
		}

		// Insert with ON CONFLICT DO NOTHING to skip existing
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&sceneActors).Error
	})
}

// BulkRemoveActorsFromScenes removes specific actors from multiple scenes
func (r *ActorRepositoryImpl) BulkRemoveActorsFromScenes(sceneIDs []uint, actorIDs []uint) error {
	if len(sceneIDs) == 0 || len(actorIDs) == 0 {
		return nil
	}

	return r.DB.
		Where("scene_id IN ?", sceneIDs).
		Where("actor_id IN ?", actorIDs).
		Delete(&SceneActor{}).Error
}

// BulkReplaceActorsForScenes replaces all actors for multiple scenes
func (r *ActorRepositoryImpl) BulkReplaceActorsForScenes(sceneIDs []uint, actorIDs []uint) error {
	if len(sceneIDs) == 0 {
		return nil
	}

	return r.DB.Transaction(func(tx *gorm.DB) error {
		// Delete all existing associations for these scenes
		if err := tx.Where("scene_id IN ?", sceneIDs).Delete(&SceneActor{}).Error; err != nil {
			return err
		}

		if len(actorIDs) == 0 {
			return nil
		}

		// Build all new associations
		sceneActors := make([]SceneActor, 0, len(sceneIDs)*len(actorIDs))
		for _, sceneID := range sceneIDs {
			for _, actorID := range actorIDs {
				sceneActors = append(sceneActors, SceneActor{
					SceneID: sceneID,
					ActorID: actorID,
				})
			}
		}

		return tx.Create(&sceneActors).Error
	})
}
