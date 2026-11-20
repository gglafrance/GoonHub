package data

import (
	"time"

	"gorm.io/gorm"
)

// Aggregate queries of the stats homepage widgets (stats, activity_calendar, watch_times,
// records). Views are rows of user_scene_watches; library figures exclude trashed and
// soft-deleted scenes. Time bucketing uses an IANA timezone name, validated by the caller.

// LibraryStats are the figures of the "stats" widget.
type LibraryStats struct {
	SceneCount      int64    `gorm:"column:scene_count"`
	TotalDuration   int64    `gorm:"column:total_duration"`
	TotalSize       int64    `gorm:"column:total_size"`
	WatchedScenes   int64    `gorm:"column:watched_scenes"`
	ViewsPeriod     int64    `gorm:"column:views_period"`
	ViewsPrevPeriod int64    `gorm:"column:views_prev_period"`
	JizzTotal       int64    `gorm:"column:jizz_total"`
	LikedCount      int64    `gorm:"column:liked_count"`
	MarkerCount     int64    `gorm:"column:marker_count"`
	RatedCount      int64    `gorm:"column:rated_count"`
	AvgRating       *float64 `gorm:"column:avg_rating"`
}

// QueryLibraryStats returns the library and user figures. Views are counted in
// [periodStart, now) and [prevStart, periodStart).
func QueryLibraryStats(db *gorm.DB, userID uint, prevStart, periodStart time.Time) (*LibraryStats, error) {
	var stats LibraryStats
	err := db.Raw(`
		WITH lib AS (
			SELECT COUNT(*) AS scene_count,
			       COALESCE(SUM(duration), 0) AS total_duration,
			       COALESCE(SUM(size), 0) AS total_size
			FROM scenes
			WHERE deleted_at IS NULL AND trashed_at IS NULL
		)
		SELECT lib.scene_count, lib.total_duration, lib.total_size,
		       (SELECT COUNT(DISTINCT w.scene_id)
		          FROM user_scene_watches w
		          JOIN scenes s ON s.id = w.scene_id AND s.deleted_at IS NULL AND s.trashed_at IS NULL
		         WHERE w.user_id = ?) AS watched_scenes,
		       (SELECT COUNT(*) FROM user_scene_watches
		         WHERE user_id = ? AND watched_at >= ?) AS views_period,
		       (SELECT COUNT(*) FROM user_scene_watches
		         WHERE user_id = ? AND watched_at >= ? AND watched_at < ?) AS views_prev_period,
		       (SELECT COALESCE(SUM(count), 0) FROM user_scene_jizzed WHERE user_id = ?) AS jizz_total,
		       (SELECT COUNT(*) FROM user_scene_likes WHERE user_id = ?) AS liked_count,
		       (SELECT COUNT(*) FROM user_scene_markers WHERE user_id = ?) AS marker_count,
		       (SELECT COUNT(*) FROM user_scene_ratings WHERE user_id = ?) AS rated_count,
		       (SELECT AVG(rating)::float8 FROM user_scene_ratings WHERE user_id = ?) AS avg_rating
		FROM lib
	`, userID, userID, periodStart, userID, prevStart, periodStart, userID, userID, userID, userID, userID).
		Scan(&stats).Error
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

// DailyViewCount is the number of views on a local day (YYYY-MM-DD).
type DailyViewCount struct {
	Day   string `gorm:"column:day"`
	Count int64  `gorm:"column:count"`
}

// QueryDailyViews returns the user's views per local day since `since` (zero time: all
// history), ordered by day.
func QueryDailyViews(db *gorm.DB, userID uint, tz string, since time.Time) ([]DailyViewCount, error) {
	var rows []DailyViewCount
	query := `
		SELECT to_char((watched_at AT TIME ZONE ?)::date, 'YYYY-MM-DD') AS day, COUNT(*) AS count
		FROM user_scene_watches
		WHERE user_id = ?`
	args := []any{tz, userID}
	if !since.IsZero() {
		query += ` AND watched_at >= ?`
		args = append(args, since)
	}
	query += ` GROUP BY 1 ORDER BY 1`
	if err := db.Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// BucketCount is a count per numeric bucket (hour 0-23 or ISO weekday 1-7).
type BucketCount struct {
	Bucket int   `gorm:"column:bucket"`
	Count  int64 `gorm:"column:count"`
}

// QueryViewsByHourAndWeekday returns the user's views by local hour of day (0-23) and by
// ISO day of week (1 = Monday ... 7 = Sunday), since `since` (zero time: all history).
func QueryViewsByHourAndWeekday(db *gorm.DB, userID uint, tz string, since time.Time) (hours, weekdays []BucketCount, err error) {
	where := `WHERE user_id = ?`
	args := []any{tz, userID}
	if !since.IsZero() {
		where += ` AND watched_at >= ?`
		args = append(args, since)
	}
	if err = db.Raw(`
		SELECT EXTRACT(HOUR FROM watched_at AT TIME ZONE ?)::int AS bucket, COUNT(*) AS count
		FROM user_scene_watches `+where+` GROUP BY 1 ORDER BY 1`, args...).Scan(&hours).Error; err != nil {
		return nil, nil, err
	}
	if err = db.Raw(`
		SELECT EXTRACT(ISODOW FROM watched_at AT TIME ZONE ?)::int AS bucket, COUNT(*) AS count
		FROM user_scene_watches `+where+` GROUP BY 1 ORDER BY 1`, args...).Scan(&weekdays).Error; err != nil {
		return nil, nil, err
	}
	return hours, weekdays, nil
}

// RecordScene is a scene referenced by a record.
type RecordScene struct {
	ID            uint   `gorm:"column:id"`
	Title         string `gorm:"column:title"`
	ThumbnailPath string `gorm:"column:thumbnail_path"`
	Value         int64  `gorm:"column:value"`
}

// WatchRecords are the user's viewing records.
type WatchRecords struct {
	MostRewatched  *RecordScene
	MostJizzed     *RecordScene
	LongestScene   *RecordScene
	BiggestFile    *RecordScene
	FirstView      *time.Time
	TotalWatchTime int64
	TotalViews     int64
}

func firstRecordScene(db *gorm.DB, query string, args ...any) (*RecordScene, error) {
	var rows []RecordScene
	if err := db.Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 || rows[0].Value <= 0 {
		return nil, nil
	}
	return &rows[0], nil
}

// QueryWatchRecords returns the scene and history records of the user (scene records only
// consider scenes still in the library).
func QueryWatchRecords(db *gorm.DB, userID uint) (*WatchRecords, error) {
	records := &WatchRecords{}
	var err error

	if records.MostRewatched, err = firstRecordScene(db, `
		SELECT s.id, COALESCE(s.title, '') AS title, COALESCE(s.thumbnail_path, '') AS thumbnail_path, w.views AS value
		FROM (SELECT scene_id, COUNT(*) AS views FROM user_scene_watches WHERE user_id = ?
		      GROUP BY scene_id ORDER BY views DESC, scene_id ASC LIMIT 5) w
		JOIN scenes s ON s.id = w.scene_id AND s.deleted_at IS NULL AND s.trashed_at IS NULL
		ORDER BY w.views DESC, s.id ASC LIMIT 1`, userID); err != nil {
		return nil, err
	}
	if records.MostJizzed, err = firstRecordScene(db, `
		SELECT s.id, COALESCE(s.title, '') AS title, COALESCE(s.thumbnail_path, '') AS thumbnail_path, j.count AS value
		FROM user_scene_jizzed j
		JOIN scenes s ON s.id = j.scene_id AND s.deleted_at IS NULL AND s.trashed_at IS NULL
		WHERE j.user_id = ?
		ORDER BY j.count DESC, s.id ASC LIMIT 1`, userID); err != nil {
		return nil, err
	}
	if records.LongestScene, err = firstRecordScene(db, `
		SELECT id, COALESCE(title, '') AS title, COALESCE(thumbnail_path, '') AS thumbnail_path, COALESCE(duration, 0) AS value
		FROM scenes WHERE deleted_at IS NULL AND trashed_at IS NULL
		ORDER BY duration DESC NULLS LAST, id ASC LIMIT 1`); err != nil {
		return nil, err
	}
	if records.BiggestFile, err = firstRecordScene(db, `
		SELECT id, COALESCE(title, '') AS title, COALESCE(thumbnail_path, '') AS thumbnail_path, COALESCE(size, 0) AS value
		FROM scenes WHERE deleted_at IS NULL AND trashed_at IS NULL
		ORDER BY size DESC NULLS LAST, id ASC LIMIT 1`); err != nil {
		return nil, err
	}

	var totals struct {
		FirstView      *time.Time `gorm:"column:first_view"`
		TotalWatchTime int64      `gorm:"column:total_watch_time"`
		TotalViews     int64      `gorm:"column:total_views"`
	}
	if err := db.Raw(`
		SELECT MIN(watched_at) AS first_view, COALESCE(SUM(watch_duration), 0) AS total_watch_time,
		       COUNT(*) AS total_views
		FROM user_scene_watches WHERE user_id = ?`, userID).Scan(&totals).Error; err != nil {
		return nil, err
	}
	records.FirstView = totals.FirstView
	records.TotalWatchTime = totals.TotalWatchTime
	records.TotalViews = totals.TotalViews
	return records, nil
}
