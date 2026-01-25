package data

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WatchHistoryRepository interface {
	RecordWatch(userID, videoID uint, duration, position int, completed bool) error
	GetLastWatch(userID, videoID uint) (*UserVideoWatch, error)
	ListUserHistory(userID uint, page, limit int) ([]UserVideoWatch, int64, error)
	ListVideoWatches(userID, videoID uint, limit int) ([]UserVideoWatch, error)
	HasViewedWithin24Hours(userID, videoID uint) (bool, error)
	IncrementVideoViewCount(videoID uint) error
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
func (r *WatchHistoryRepositoryImpl) RecordWatch(userID, videoID uint, duration, position int, completed bool) error {
	now := time.Now().UTC()
	// Use 5-minute session window - short enough to separate distinct viewing sessions
	// but long enough to merge rapid updates during continuous watching
	sessionWindow := now.Add(-5 * time.Minute)

	return r.DB.Transaction(func(tx *gorm.DB) error {
		// Try to find an active watch session (watched within last 5 minutes)
		// Using watched_at instead of created_at so the session extends while actively watching
		var existing UserVideoWatch
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND video_id = ? AND watched_at > ?", userID, videoID, sessionWindow).
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
		watch := UserVideoWatch{
			UserID:        userID,
			VideoID:       videoID,
			WatchedAt:     now,
			WatchDuration: duration,
			LastPosition:  position,
			Completed:     completed,
		}
		return tx.Create(&watch).Error
	})
}

func (r *WatchHistoryRepositoryImpl) GetLastWatch(userID, videoID uint) (*UserVideoWatch, error) {
	var watch UserVideoWatch
	err := r.DB.Where("user_id = ? AND video_id = ?", userID, videoID).
		Order("watched_at DESC").
		First(&watch).Error
	if err != nil {
		return nil, err
	}
	return &watch, nil
}

func (r *WatchHistoryRepositoryImpl) ListUserHistory(userID uint, page, limit int) ([]UserVideoWatch, int64, error) {
	var watches []UserVideoWatch
	var total int64

	offset := (page - 1) * limit

	// Count distinct videos
	subQuery := r.DB.Model(&UserVideoWatch{}).
		Select("DISTINCT video_id").
		Where("user_id = ?", userID)

	if err := r.DB.Table("(?) as distinct_videos", subQuery).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get most recent watch for each video using window function with proper ordering and pagination
	err := r.DB.Raw(`
		WITH ranked AS (
			SELECT *, ROW_NUMBER() OVER (PARTITION BY video_id ORDER BY watched_at DESC) as rn
			FROM user_video_watches
			WHERE user_id = ?
		)
		SELECT id, user_id, video_id, watched_at, watch_duration, last_position, completed, created_at, updated_at
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

func (r *WatchHistoryRepositoryImpl) ListVideoWatches(userID, videoID uint, limit int) ([]UserVideoWatch, error) {
	var watches []UserVideoWatch
	query := r.DB.Where("user_id = ? AND video_id = ?", userID, videoID).
		Order("watched_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&watches).Error; err != nil {
		return nil, err
	}
	return watches, nil
}

func (r *WatchHistoryRepositoryImpl) HasViewedWithin24Hours(userID, videoID uint) (bool, error) {
	var count int64
	cutoff := time.Now().UTC().Add(-24 * time.Hour)
	err := r.DB.Model(&UserVideoWatch{}).
		Where("user_id = ? AND video_id = ? AND watched_at > ?", userID, videoID, cutoff).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *WatchHistoryRepositoryImpl) IncrementVideoViewCount(videoID uint) error {
	return r.DB.Model(&Video{}).Where("id = ?", videoID).
		UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error
}

var _ WatchHistoryRepository = (*WatchHistoryRepositoryImpl)(nil)
