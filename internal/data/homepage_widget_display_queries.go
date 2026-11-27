package data

import (
	"time"

	"gorm.io/gorm"
)

// DisplayWidgetQueries holds the queries of the shorts, preview_wall, hero, top_period and
// quick_rate homepage widgets.
type DisplayWidgetQueries struct {
	DB *gorm.DB
}

// NewDisplayWidgetQueries creates the display widget queries.
func NewDisplayWidgetQueries(db *gorm.DB) DisplayWidgetQueries {
	return DisplayWidgetQueries{DB: db}
}

// Scene sources of the display widgets
const (
	WidgetSourceRandom    = "random"
	WidgetSourceUnwatched = "unwatched"
	WidgetSourceLiked     = "liked"
	WidgetSourceLatest    = "latest"
	WidgetSourceTopJizz   = "top_jizz"
)

// SceneViewCount is the number of views of a scene over a period.
type SceneViewCount struct {
	SceneID uint  `json:"scene_id"`
	Views   int64 `json:"views"`
}

// visibleScenes selects the scenes that can be shown on the homepage: not deleted (GORM soft
// delete), not trashed, not corrupted and with a thumbnail.
func (q DisplayWidgetQueries) visibleScenes() *gorm.DB {
	return q.DB.Model(&Scene{}).
		Where("scenes.trashed_at IS NULL AND scenes.is_corrupted = false AND scenes.thumbnail_path <> ''")
}

func unwatchedBy(db *gorm.DB, userID uint) *gorm.DB {
	return db.Where("NOT EXISTS (SELECT 1 FROM user_scene_watches w WHERE w.scene_id = scenes.id AND w.user_id = ?)", userID)
}

// ShortScenes returns random scenes lasting at most maxDuration seconds.
func (q DisplayWidgetQueries) ShortScenes(userID uint, maxDuration int, unwatchedOnly bool, limit int) ([]Scene, error) {
	db := q.visibleScenes().Where("scenes.duration > 0 AND scenes.duration <= ?", maxDuration)
	if unwatchedOnly {
		db = unwatchedBy(db, userID)
	}
	var scenes []Scene
	err := db.Order("random()").Limit(limit).Find(&scenes).Error
	return scenes, err
}

// SourceScenes returns scenes from a source (random, unwatched, liked, latest, top_jizz).
// With requirePreview only scenes having a generated preview video are returned.
func (q DisplayWidgetQueries) SourceScenes(userID uint, source string, requirePreview bool, limit int) ([]Scene, error) {
	db := q.visibleScenes()
	if requirePreview {
		db = db.Where("scenes.preview_video_path <> ''")
	}
	switch source {
	case WidgetSourceUnwatched:
		db = unwatchedBy(db, userID).Order("random()")
	case WidgetSourceLiked:
		db = db.Joins("JOIN user_scene_likes l ON l.scene_id = scenes.id AND l.user_id = ?", userID).Order("random()")
	case WidgetSourceLatest:
		db = db.Order("scenes.created_at DESC")
	case WidgetSourceTopJizz:
		db = db.Joins("JOIN user_scene_jizzed j ON j.scene_id = scenes.id AND j.user_id = ? AND j.count > 0", userID).
			Order("j.count DESC").Order("random()")
	default:
		db = db.Order("random()")
	}
	var scenes []Scene
	err := db.Limit(limit).Find(&scenes).Error
	return scenes, err
}

// TopWatchedScenes returns the user's most watched scenes in [from, to).
func (q DisplayWidgetQueries) TopWatchedScenes(userID uint, from, to time.Time, limit int) ([]SceneViewCount, error) {
	var rows []SceneViewCount
	err := q.DB.Raw(`
		SELECT w.scene_id, COUNT(*) AS views
		FROM user_scene_watches w
		JOIN scenes s ON s.id = w.scene_id AND s.deleted_at IS NULL AND s.trashed_at IS NULL
		WHERE w.user_id = ? AND w.watched_at >= ? AND w.watched_at < ?
		GROUP BY w.scene_id
		ORDER BY views DESC, MAX(w.watched_at) DESC
		LIMIT ?
	`, userID, from, to, limit).Scan(&rows).Error
	return rows, err
}

// ViewCountsForScenes returns the user's views of the given scenes in [from, to).
func (q DisplayWidgetQueries) ViewCountsForScenes(userID uint, sceneIDs []uint, from, to time.Time) (map[uint]int64, error) {
	counts := make(map[uint]int64, len(sceneIDs))
	if len(sceneIDs) == 0 {
		return counts, nil
	}
	var rows []SceneViewCount
	err := q.DB.Raw(`
		SELECT w.scene_id, COUNT(*) AS views
		FROM user_scene_watches w
		WHERE w.user_id = ? AND w.scene_id IN ? AND w.watched_at >= ? AND w.watched_at < ?
		GROUP BY w.scene_id
	`, userID, sceneIDs, from, to).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		counts[r.SceneID] = r.Views
	}
	return counts, nil
}

// CountViews returns the user's number of views in [from, to).
func (q DisplayWidgetQueries) CountViews(userID uint, from, to time.Time) (int64, error) {
	var count int64
	err := q.DB.Raw(`
		SELECT COUNT(*) FROM user_scene_watches w
		WHERE w.user_id = ? AND w.watched_at >= ? AND w.watched_at < ?
	`, userID, from, to).Scan(&count).Error
	return count, err
}

const unratedRecentlyWatchedFrom = `
	FROM scenes s
	JOIN (
		SELECT scene_id, MAX(watched_at) AS last_watched
		FROM user_scene_watches
		WHERE user_id = ? AND watched_at >= ?
		GROUP BY scene_id
	) w ON w.scene_id = s.id
	WHERE s.deleted_at IS NULL AND s.trashed_at IS NULL
	  AND NOT EXISTS (SELECT 1 FROM user_scene_ratings r WHERE r.user_id = ? AND r.scene_id = s.id)`

// UnratedRecentlyWatched returns the scenes watched since the given time and not rated by the
// user (most recently watched first), and their total count.
func (q DisplayWidgetQueries) UnratedRecentlyWatched(userID uint, since time.Time, limit int) ([]Scene, int64, error) {
	var total int64
	if err := q.DB.Raw(`SELECT COUNT(*) `+unratedRecentlyWatchedFrom, userID, since, userID).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	var scenes []Scene
	err := q.DB.Raw(`SELECT s.* `+unratedRecentlyWatchedFrom+`
		ORDER BY w.last_watched DESC
		LIMIT ?`, userID, since, userID, limit).Scan(&scenes).Error
	return scenes, total, err
}
