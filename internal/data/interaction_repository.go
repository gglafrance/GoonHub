package data

import (
	"time"

	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SceneInteractions holds all interaction data for a scene
type SceneInteractions struct {
	Rating      float64
	Liked       bool
	JizzedCount int
}

type InteractionRepository interface {
	UpsertRating(userID, sceneID uint, rating float64) error
	DeleteRating(userID, sceneID uint) error
	GetRating(userID, sceneID uint) (*UserSceneRating, error)
	GetRatingsBySceneIDs(userID uint, sceneIDs []uint) (map[uint]float64, error)
	SetLike(userID, sceneID uint) error
	DeleteLike(userID, sceneID uint) error
	IsLiked(userID, sceneID uint) (bool, error)
	// IncrementJizzed adds one to the jizz count and records a dated event (now).
	IncrementJizzed(userID, sceneID uint) (int, error)
	// GetJizzEvents returns the user's dated jizz events on a scene, most recent first.
	GetJizzEvents(userID, sceneID uint) ([]UserSceneJizzEvent, error)
	// GetLastJizzTimes returns, for the given scenes, the user's most recent jizz time.
	GetLastJizzTimes(userID uint, sceneIDs []uint) (map[uint]time.Time, error)
	// SetJizzedCount sets the exact jizzed count (not an increment). A count <= 0 removes the record.
	SetJizzedCount(userID, sceneID uint, count int) error
	GetJizzedCount(userID, sceneID uint) (int, error)
	GetAllInteractions(userID, sceneID uint) (*SceneInteractions, error)
	GetLikedSceneIDs(userID uint) ([]uint, error)
	GetRatedSceneIDs(userID uint, minRating, maxRating float64) ([]uint, error)
	GetJizzedSceneIDs(userID uint, minCount, maxCount int) ([]uint, error)
	// GetWatchedSceneIDs returns the scenes the user has watched at least once.
	GetWatchedSceneIDs(userID uint) ([]uint, error)
	// GetMarkedSceneIDs returns the scenes on which the user sees markers (own or shared).
	GetMarkedSceneIDs(userID uint) ([]uint, error)
	GetLikesBySceneIDs(userID uint, sceneIDs []uint) (map[uint]bool, error)
	GetJizzCountsBySceneIDs(userID uint, sceneIDs []uint) (map[uint]int, error)
	// GetWatchedBySceneIDs returns the scenes of sceneIDs the user watched to the end at least once
	GetWatchedBySceneIDs(userID uint, sceneIDs []uint) (map[uint]bool, error)

	// Reassignment methods (for duplicate resolution)
	ReassignInteractionsToScene(fromSceneID, toSceneID uint) error
}

type InteractionRepositoryImpl struct {
	DB *gorm.DB
}

func NewInteractionRepository(db *gorm.DB) *InteractionRepositoryImpl {
	return &InteractionRepositoryImpl{DB: db}
}

func (r *InteractionRepositoryImpl) UpsertRating(userID, sceneID uint, rating float64) error {
	record := UserSceneRating{
		UserID:  userID,
		SceneID: sceneID,
		Rating:  rating,
	}
	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "scene_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"rating", "updated_at"}),
	}).Create(&record).Error
}

func (r *InteractionRepositoryImpl) DeleteRating(userID, sceneID uint) error {
	return r.DB.Where("user_id = ? AND scene_id = ?", userID, sceneID).Delete(&UserSceneRating{}).Error
}

func (r *InteractionRepositoryImpl) GetRating(userID, sceneID uint) (*UserSceneRating, error) {
	var rating UserSceneRating
	err := r.DB.Where("user_id = ? AND scene_id = ?", userID, sceneID).First(&rating).Error
	if err != nil {
		return nil, err
	}
	return &rating, nil
}

func (r *InteractionRepositoryImpl) GetRatingsBySceneIDs(userID uint, sceneIDs []uint) (map[uint]float64, error) {
	if len(sceneIDs) == 0 {
		return make(map[uint]float64), nil
	}

	var ratings []UserSceneRating
	err := r.DB.Where("user_id = ? AND scene_id IN ?", userID, sceneIDs).Find(&ratings).Error
	if err != nil {
		return nil, err
	}

	result := make(map[uint]float64)
	for _, r := range ratings {
		result[r.SceneID] = r.Rating
	}
	return result, nil
}

func (r *InteractionRepositoryImpl) SetLike(userID, sceneID uint) error {
	like := UserSceneLike{
		UserID:  userID,
		SceneID: sceneID,
	}
	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "scene_id"}},
		DoNothing: true,
	}).Create(&like).Error
}

func (r *InteractionRepositoryImpl) DeleteLike(userID, sceneID uint) error {
	return r.DB.Where("user_id = ? AND scene_id = ?", userID, sceneID).Delete(&UserSceneLike{}).Error
}

func (r *InteractionRepositoryImpl) IsLiked(userID, sceneID uint) (bool, error) {
	var count int64
	err := r.DB.Model(&UserSceneLike{}).Where("user_id = ? AND scene_id = ?", userID, sceneID).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *InteractionRepositoryImpl) IncrementJizzed(userID, sceneID uint) (int, error) {
	var count int
	err := r.DB.Transaction(func(tx *gorm.DB) error {
		record := UserSceneJizzed{
			UserID:  userID,
			SceneID: sceneID,
			Count:   1,
		}
		result := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}, {Name: "scene_id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"count":      gorm.Expr("user_scene_jizzed.count + 1"),
				"updated_at": gorm.Expr("NOW()"),
			}),
		}).Create(&record)
		if result.Error != nil {
			return result.Error
		}
		event := UserSceneJizzEvent{UserID: userID, SceneID: sceneID, JizzedAt: time.Now().UTC()}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&event).Error; err != nil {
			return err
		}

		// Fetch the current count
		var updated UserSceneJizzed
		if err := tx.Where("user_id = ? AND scene_id = ?", userID, sceneID).First(&updated).Error; err != nil {
			return err
		}
		count = updated.Count
		return nil
	})
	return count, err
}

func (r *InteractionRepositoryImpl) GetJizzEvents(userID, sceneID uint) ([]UserSceneJizzEvent, error) {
	var events []UserSceneJizzEvent
	err := r.DB.Where("user_id = ? AND scene_id = ?", userID, sceneID).
		Order("jizzed_at DESC").
		Find(&events).Error
	return events, err
}

func (r *InteractionRepositoryImpl) GetLastJizzTimes(userID uint, sceneIDs []uint) (map[uint]time.Time, error) {
	result := make(map[uint]time.Time)
	if len(sceneIDs) == 0 {
		return result, nil
	}
	type row struct {
		SceneID  uint
		LastJizz time.Time
	}
	var rows []row
	err := r.DB.Raw(`
		SELECT scene_id, MAX(jizzed_at) AS last_jizz
		FROM user_scene_jizz_events
		WHERE user_id = ? AND scene_id IN ?
		GROUP BY scene_id`, userID, sceneIDs).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		result[r.SceneID] = r.LastJizz
	}
	return result, nil
}

func (r *InteractionRepositoryImpl) SetJizzedCount(userID, sceneID uint, count int) error {
	if count <= 0 {
		return r.DB.Where("user_id = ? AND scene_id = ?", userID, sceneID).Delete(&UserSceneJizzed{}).Error
	}
	record := UserSceneJizzed{
		UserID:  userID,
		SceneID: sceneID,
		Count:   count,
	}
	return r.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "scene_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"count":      count,
			"updated_at": gorm.Expr("NOW()"),
		}),
	}).Create(&record).Error
}

func (r *InteractionRepositoryImpl) GetJizzedCount(userID, sceneID uint) (int, error) {
	var record UserSceneJizzed
	err := r.DB.Where("user_id = ? AND scene_id = ?", userID, sceneID).First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return record.Count, nil
}

func (r *InteractionRepositoryImpl) GetAllInteractions(userID, sceneID uint) (*SceneInteractions, error) {
	result := &SceneInteractions{}

	// Get rating
	var rating UserSceneRating
	err := r.DB.Where("user_id = ? AND scene_id = ?", userID, sceneID).First(&rating).Error
	if err == nil {
		result.Rating = rating.Rating
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// Check if liked
	var likeCount int64
	err = r.DB.Model(&UserSceneLike{}).Where("user_id = ? AND scene_id = ?", userID, sceneID).Count(&likeCount).Error
	if err != nil {
		return nil, err
	}
	result.Liked = likeCount > 0

	// Get jizzed count
	var jizzed UserSceneJizzed
	err = r.DB.Where("user_id = ? AND scene_id = ?", userID, sceneID).First(&jizzed).Error
	if err == nil {
		result.JizzedCount = jizzed.Count
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return result, nil
}

// Ensure gorm.ErrRecordNotFound is accessible for callers
func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

func (r *InteractionRepositoryImpl) GetLikedSceneIDs(userID uint) ([]uint, error) {
	var ids []uint
	err := r.DB.Model(&UserSceneLike{}).
		Where("user_id = ?", userID).
		Pluck("scene_id", &ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *InteractionRepositoryImpl) GetRatedSceneIDs(userID uint, minRating, maxRating float64) ([]uint, error) {
	var ids []uint
	query := r.DB.Model(&UserSceneRating{}).Where("user_id = ?", userID)

	if minRating > 0 {
		query = query.Where("rating >= ?", minRating)
	}
	if maxRating > 0 {
		query = query.Where("rating <= ?", maxRating)
	}

	err := query.Pluck("scene_id", &ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *InteractionRepositoryImpl) GetJizzedSceneIDs(userID uint, minCount, maxCount int) ([]uint, error) {
	var ids []uint
	query := r.DB.Model(&UserSceneJizzed{}).Where("user_id = ?", userID)

	if minCount > 0 {
		query = query.Where("count >= ?", minCount)
	}
	if maxCount > 0 {
		query = query.Where("count <= ?", maxCount)
	}

	err := query.Pluck("scene_id", &ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *InteractionRepositoryImpl) GetWatchedSceneIDs(userID uint) ([]uint, error) {
	var ids []uint
	err := r.DB.Raw("SELECT DISTINCT scene_id FROM user_scene_watches WHERE user_id = ?", userID).
		Scan(&ids).Error
	return ids, err
}

func (r *InteractionRepositoryImpl) GetMarkedSceneIDs(userID uint) ([]uint, error) {
	var ids []uint
	err := r.DB.Raw("SELECT DISTINCT scene_id FROM user_scene_markers WHERE "+VisibleMarkersSQL(""), userID).
		Scan(&ids).Error
	return ids, err
}

func (r *InteractionRepositoryImpl) GetLikesBySceneIDs(userID uint, sceneIDs []uint) (map[uint]bool, error) {
	if len(sceneIDs) == 0 {
		return make(map[uint]bool), nil
	}

	var likes []UserSceneLike
	err := r.DB.Where("user_id = ? AND scene_id IN ?", userID, sceneIDs).Find(&likes).Error
	if err != nil {
		return nil, err
	}

	result := make(map[uint]bool)
	for _, l := range likes {
		result[l.SceneID] = true
	}
	return result, nil
}

func (r *InteractionRepositoryImpl) GetWatchedBySceneIDs(userID uint, sceneIDs []uint) (map[uint]bool, error) {
	result := make(map[uint]bool)
	if len(sceneIDs) == 0 {
		return result, nil
	}
	var ids []uint
	err := r.DB.Model(&UserSceneWatch{}).
		Where("user_id = ? AND scene_id IN ? AND completed", userID, sceneIDs).
		Distinct("scene_id").
		Pluck("scene_id", &ids).Error
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		result[id] = true
	}
	return result, nil
}

func (r *InteractionRepositoryImpl) GetJizzCountsBySceneIDs(userID uint, sceneIDs []uint) (map[uint]int, error) {
	if len(sceneIDs) == 0 {
		return make(map[uint]int), nil
	}

	var records []UserSceneJizzed
	err := r.DB.Where("user_id = ? AND scene_id IN ?", userID, sceneIDs).Find(&records).Error
	if err != nil {
		return nil, err
	}

	result := make(map[uint]int)
	for _, j := range records {
		result[j.SceneID] = j.Count
	}
	return result, nil
}

// ReassignInteractionsToScene moves user interactions from one scene to another.
// For each interaction table, it inserts records that don't already exist on the
// target scene, then deletes the originals from the source scene.
func (r *InteractionRepositoryImpl) ReassignInteractionsToScene(fromSceneID, toSceneID uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		// Ratings: copy where user doesn't already have a rating on the target
		if err := tx.Exec(`
			INSERT INTO user_scene_ratings (user_id, scene_id, rating, created_at, updated_at)
			SELECT user_id, ?, rating, created_at, updated_at
			FROM user_scene_ratings
			WHERE scene_id = ?
			AND user_id NOT IN (SELECT user_id FROM user_scene_ratings WHERE scene_id = ?)
		`, toSceneID, fromSceneID, toSceneID).Error; err != nil {
			return err
		}
		if err := tx.Where("scene_id = ?", fromSceneID).Delete(&UserSceneRating{}).Error; err != nil {
			return err
		}

		// Likes: copy where user doesn't already have a like on the target
		if err := tx.Exec(`
			INSERT INTO user_scene_likes (user_id, scene_id, created_at)
			SELECT user_id, ?, created_at
			FROM user_scene_likes
			WHERE scene_id = ?
			AND user_id NOT IN (SELECT user_id FROM user_scene_likes WHERE scene_id = ?)
		`, toSceneID, fromSceneID, toSceneID).Error; err != nil {
			return err
		}
		if err := tx.Where("scene_id = ?", fromSceneID).Delete(&UserSceneLike{}).Error; err != nil {
			return err
		}

		// Jizzed: copy where user doesn't already have a jizz count on the target
		if err := tx.Exec(`
			INSERT INTO user_scene_jizzed (user_id, scene_id, count, created_at, updated_at)
			SELECT user_id, ?, count, created_at, updated_at
			FROM user_scene_jizzed
			WHERE scene_id = ?
			AND user_id NOT IN (SELECT user_id FROM user_scene_jizzed WHERE scene_id = ?)
		`, toSceneID, fromSceneID, toSceneID).Error; err != nil {
			return err
		}
		// Dated jizz events move to the target scene (duplicates of the same instant are dropped)
		if err := tx.Exec(`
			INSERT INTO user_scene_jizz_events (user_id, scene_id, jizzed_at, created_at)
			SELECT user_id, ?, jizzed_at, created_at
			FROM user_scene_jizz_events
			WHERE scene_id = ?
			ON CONFLICT (user_id, scene_id, jizzed_at) DO NOTHING
		`, toSceneID, fromSceneID).Error; err != nil {
			return err
		}
		if err := tx.Where("scene_id = ?", fromSceneID).Delete(&UserSceneJizzEvent{}).Error; err != nil {
			return err
		}
		if err := tx.Where("scene_id = ?", fromSceneID).Delete(&UserSceneJizzed{}).Error; err != nil {
			return err
		}

		return nil
	})
}

// Ensure InteractionRepositoryImpl implements InteractionRepository
var _ InteractionRepository = (*InteractionRepositoryImpl)(nil)
