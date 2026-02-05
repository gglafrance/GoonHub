package data

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WatchHistoryRepository interface {
	RecordWatch(userID, sceneID uint, duration, position int, completed bool) error
	GetLastWatch(userID, sceneID uint) (*UserSceneWatch, error)
	ListUserHistory(userID uint, page, limit int) ([]UserSceneWatch, int64, error)
	ListUserHistoryByDateRange(userID uint, since time.Time, limit int) ([]UserSceneWatch, error)
	ListUserHistoryByTimeRange(userID uint, since, until time.Time, limit int) ([]UserSceneWatch, error)
	GetDailyActivityCounts(userID uint, since time.Time) ([]DailyActivityCount, error)
	ListSceneWatches(userID, sceneID uint, limit int) ([]UserSceneWatch, error)
	// TryIncrementViewCount atomically checks if a view should be counted (not counted in last 24h)
	// and increments the scene view count if so. Returns true if the count was incremented.
	// This prevents race conditions from concurrent requests.
	TryIncrementViewCount(userID, sceneID uint) (bool, error)
}

type WatchHistoryRepositoryImpl struct {
	DB *gorm.DB
}

func NewWatchHistoryRepository(db *gorm.DB) *WatchHistoryRepositoryImpl {
	return &WatchHistoryRepositoryImpl{DB: db}
}

// RecordWatch creates or updates a watch session.
// If an active watch session exists (watched within last 5 minutes), it updates that record.
// Otherwise, it creates a new watch record.
// Uses a transaction with row locking to prevent race conditions.
func (r *WatchHistoryRepositoryImpl) RecordWatch(userID, sceneID uint, duration, position int, completed bool) error {
	now := time.Now().UTC()
	// Use 5-minute session window - short enough to separate distinct viewing sessions
	// but long enough to merge rapid updates during continuous watching
	sessionWindow := now.Add(-5 * time.Minute)

	return r.DB.Transaction(func(tx *gorm.DB) error {
		// Try to find an active watch session (watched within last 5 minutes)
		// Using watched_at instead of created_at so the session extends while actively watching
		var existing UserSceneWatch
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND scene_id = ? AND watched_at > ?", userID, sceneID, sessionWindow).
			Order("watched_at DESC").
			First(&existing).Error

		if err == nil {
			// Update existing session - keep accumulating watch time
			updates := map[string]any{
				"watch_duration": duration,
				"last_position":  position,
				"completed":      completed,
				"watched_at":     now,
			}
			return tx.Model(&existing).Updates(updates).Error
		}

		// Only create new record if error is "not found"
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// Create new session
		watch := UserSceneWatch{
			UserID:        userID,
			SceneID:       sceneID,
			WatchedAt:     now,
			WatchDuration: duration,
			LastPosition:  position,
			Completed:     completed,
		}
		return tx.Create(&watch).Error
	})
}

func (r *WatchHistoryRepositoryImpl) GetLastWatch(userID, sceneID uint) (*UserSceneWatch, error) {
	var watch UserSceneWatch
	err := r.DB.Where("user_id = ? AND scene_id = ?", userID, sceneID).
		Order("watched_at DESC").
		First(&watch).Error
	if err != nil {
		return nil, err
	}
	return &watch, nil
}

func (r *WatchHistoryRepositoryImpl) ListUserHistory(userID uint, page, limit int) ([]UserSceneWatch, int64, error) {
	var watches []UserSceneWatch
	var total int64

	offset := (page - 1) * limit

	// Count distinct scenes
	subQuery := r.DB.Model(&UserSceneWatch{}).
		Select("DISTINCT scene_id").
		Where("user_id = ?", userID)

	if err := r.DB.Table("(?) as distinct_scenes", subQuery).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get most recent watch for each scene using window function with proper ordering and pagination
	err := r.DB.Raw(`
		WITH ranked AS (
			SELECT *, ROW_NUMBER() OVER (PARTITION BY scene_id ORDER BY watched_at DESC) as rn
			FROM user_scene_watches
			WHERE user_id = ?
		)
		SELECT id, user_id, scene_id, watched_at, watch_duration, last_position, completed, created_at, updated_at
		FROM ranked
		WHERE rn = 1
		ORDER BY watched_at DESC
		LIMIT ? OFFSET ?
	`, userID, limit, offset).Scan(&watches).Error
	if err != nil {
		return nil, 0, err
	}

	return watches, total, nil
}

func (r *WatchHistoryRepositoryImpl) ListSceneWatches(userID, sceneID uint, limit int) ([]UserSceneWatch, error) {
	var watches []UserSceneWatch
	query := r.DB.Where("user_id = ? AND scene_id = ?", userID, sceneID).
		Order("watched_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&watches).Error; err != nil {
		return nil, err
	}
	return watches, nil
}

// TryIncrementViewCount atomically checks if the user has had a view counted in the last 24 hours.
// If not, it records the view and increments the scene's view count.
// Returns true if the view count was incremented, false if already counted recently.
// Uses a single transaction with INSERT ON CONFLICT to prevent race conditions.
func (r *WatchHistoryRepositoryImpl) TryIncrementViewCount(userID, sceneID uint) (bool, error) {
	var incremented bool

	err := r.DB.Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		cutoff := now.Add(-24 * time.Hour)

		// Atomic upsert: insert new record or update if last_counted_at > 24h ago
		// Using raw SQL for the atomic ON CONFLICT with WHERE clause
		result := tx.Exec(`
			INSERT INTO user_scene_view_counts (user_id, scene_id, last_counted_at, created_at)
			VALUES (?, ?, ?, ?)
			ON CONFLICT (user_id, scene_id) DO UPDATE
			SET last_counted_at = EXCLUDED.last_counted_at
			WHERE user_scene_view_counts.last_counted_at < ?
		`, userID, sceneID, now, now, cutoff)

		if result.Error != nil {
			return result.Error
		}

		// If a row was affected (inserted or updated), increment the view count
		if result.RowsAffected > 0 {
			if err := tx.Model(&Scene{}).Where("id = ?", sceneID).
				UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error; err != nil {
				return err
			}
			incremented = true
		}

		return nil
	})

	return incremented, err
}

// ListUserHistoryByDateRange returns watch records since the given time, deduped per scene per day.
// The same scene may appear on multiple days but only once per day (most recent watch kept).
func (r *WatchHistoryRepositoryImpl) ListUserHistoryByDateRange(userID uint, since time.Time, limit int) ([]UserSceneWatch, error) {
	var watches []UserSceneWatch
	err := r.DB.Raw(`
		WITH ranked AS (
			SELECT *, ROW_NUMBER() OVER (
				PARTITION BY scene_id, DATE(watched_at)
				ORDER BY watched_at DESC
			) as rn
			FROM user_scene_watches
			WHERE user_id = ? AND watched_at >= ?
		)
		SELECT id, user_id, scene_id, watched_at, watch_duration, last_position, completed, created_at, updated_at
		FROM ranked WHERE rn = 1
		ORDER BY watched_at DESC
		LIMIT ?
	`, userID, since, limit).Scan(&watches).Error
	if err != nil {
		return nil, err
	}
	return watches, nil
}

// ListUserHistoryByTimeRange returns watch records between since and until, deduped per scene per day.
func (r *WatchHistoryRepositoryImpl) ListUserHistoryByTimeRange(userID uint, since, until time.Time, limit int) ([]UserSceneWatch, error) {
	var watches []UserSceneWatch
	err := r.DB.Raw(`
		WITH ranked AS (
			SELECT *, ROW_NUMBER() OVER (
				PARTITION BY scene_id, DATE(watched_at)
				ORDER BY watched_at DESC
			) as rn
			FROM user_scene_watches
			WHERE user_id = ? AND watched_at >= ? AND watched_at <= ?
		)
		SELECT id, user_id, scene_id, watched_at, watch_duration, last_position, completed, created_at, updated_at
		FROM ranked WHERE rn = 1
		ORDER BY watched_at DESC
		LIMIT ?
	`, userID, since, until, limit).Scan(&watches).Error
	if err != nil {
		return nil, err
	}
	return watches, nil
}

// GetDailyActivityCounts returns the count of distinct scenes watched per day since the given time.
func (r *WatchHistoryRepositoryImpl) GetDailyActivityCounts(userID uint, since time.Time) ([]DailyActivityCount, error) {
	var counts []DailyActivityCount
	err := r.DB.Raw(`
		SELECT DATE(watched_at) as date, COUNT(DISTINCT scene_id) as count
		FROM user_scene_watches
		WHERE user_id = ? AND watched_at >= ?
		GROUP BY DATE(watched_at)
		ORDER BY date ASC
	`, userID, since).Scan(&counts).Error
	if err != nil {
		return nil, err
	}
	return counts, nil
}

var _ WatchHistoryRepository = (*WatchHistoryRepositoryImpl)(nil)
