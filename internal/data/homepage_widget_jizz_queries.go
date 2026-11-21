package data

import (
	"time"

	"gorm.io/gorm"
)

// Aggregate queries over the dated "o" events (user_scene_jizz_events) used by the stats
// homepage widgets. Time bucketing uses an IANA timezone name validated by the caller.
// Events of scenes no longer in the library still count in time series (they happened);
// scene rankings only consider scenes still in the library.

// JizzPeriodStats are the o figures of a period and of the period before it.
type JizzPeriodStats struct {
	Period     int64      `gorm:"column:period"`
	PrevPeriod int64      `gorm:"column:prev_period"`
	Total      int64      `gorm:"column:total"`
	Last       *time.Time `gorm:"column:last"`
	First      *time.Time `gorm:"column:first"`
}

// QueryJizzPeriodStats counts the user's dated o in [periodStart, now) and
// [prevStart, periodStart), with the total and the first/last dates.
func QueryJizzPeriodStats(db *gorm.DB, userID uint, prevStart, periodStart time.Time) (*JizzPeriodStats, error) {
	var stats JizzPeriodStats
	err := db.Raw(`
		SELECT COUNT(*) FILTER (WHERE jizzed_at >= ?) AS period,
		       COUNT(*) FILTER (WHERE jizzed_at >= ? AND jizzed_at < ?) AS prev_period,
		       COUNT(*) AS total,
		       MAX(jizzed_at) AS last,
		       MIN(jizzed_at) AS first
		FROM user_scene_jizz_events
		WHERE user_id = ?`, periodStart, prevStart, periodStart, userID).Scan(&stats).Error
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

// QueryDailyJizz returns the user's o per local day since `since` (zero time: all history).
func QueryDailyJizz(db *gorm.DB, userID uint, tz string, since time.Time) ([]DailyViewCount, error) {
	var rows []DailyViewCount
	query := `
		SELECT to_char((jizzed_at AT TIME ZONE ?)::date, 'YYYY-MM-DD') AS day, COUNT(*) AS count
		FROM user_scene_jizz_events
		WHERE user_id = ?`
	args := []any{tz, userID}
	if !since.IsZero() {
		query += ` AND jizzed_at >= ?`
		args = append(args, since)
	}
	query += ` GROUP BY 1 ORDER BY 1`
	if err := db.Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// QueryMonthlyJizz returns the user's o per local month (YYYY-MM) since `since`.
func QueryMonthlyJizz(db *gorm.DB, userID uint, tz string, since time.Time) ([]DailyViewCount, error) {
	var rows []DailyViewCount
	query := `
		SELECT to_char(date_trunc('month', jizzed_at AT TIME ZONE ?), 'YYYY-MM') AS day, COUNT(*) AS count
		FROM user_scene_jizz_events
		WHERE user_id = ?`
	args := []any{tz, userID}
	if !since.IsZero() {
		query += ` AND jizzed_at >= ?`
		args = append(args, since)
	}
	query += ` GROUP BY 1 ORDER BY 1`
	if err := db.Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// QueryJizzByHourAndWeekday returns the user's o by local hour (0-23) and ISO weekday
// (1 = Monday ... 7 = Sunday), since `since` (zero time: all history).
func QueryJizzByHourAndWeekday(db *gorm.DB, userID uint, tz string, since time.Time) (hours, weekdays []BucketCount, err error) {
	where := `WHERE user_id = ?`
	args := []any{tz, userID}
	if !since.IsZero() {
		where += ` AND jizzed_at >= ?`
		args = append(args, since)
	}
	if err = db.Raw(`
		SELECT EXTRACT(HOUR FROM jizzed_at AT TIME ZONE ?)::int AS bucket, COUNT(*) AS count
		FROM user_scene_jizz_events `+where+` GROUP BY 1 ORDER BY 1`, args...).Scan(&hours).Error; err != nil {
		return nil, nil, err
	}
	if err = db.Raw(`
		SELECT EXTRACT(ISODOW FROM jizzed_at AT TIME ZONE ?)::int AS bucket, COUNT(*) AS count
		FROM user_scene_jizz_events `+where+` GROUP BY 1 ORDER BY 1`, args...).Scan(&weekdays).Error; err != nil {
		return nil, nil, err
	}
	return hours, weekdays, nil
}

// JizzSceneCount is a scene with a number of o (scene still in the library).
type JizzSceneCount struct {
	ID            uint      `gorm:"column:id" json:"id"`
	Title         string    `gorm:"column:title" json:"title"`
	ThumbnailPath string    `gorm:"column:thumbnail_path" json:"thumbnail_path"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updated_at"`
	Count         int64     `gorm:"column:count" json:"count"`
	// Day is set for per-day records (local YYYY-MM-DD)
	Day string `gorm:"column:day" json:"day,omitempty"`
}

// QueryTopJizzScenes returns the scenes with the most o events in [from, to) (zero from: all
// history), most first.
func QueryTopJizzScenes(db *gorm.DB, userID uint, from, to time.Time, limit int) ([]JizzSceneCount, error) {
	var rows []JizzSceneCount
	query := `
		SELECT s.id, COALESCE(s.title, '') AS title, COALESCE(s.thumbnail_path, '') AS thumbnail_path,
		       s.updated_at, e.count
		FROM (SELECT scene_id, COUNT(*) AS count FROM user_scene_jizz_events
		      WHERE user_id = ? AND jizzed_at < ?`
	args := []any{userID, to}
	if !from.IsZero() {
		query += ` AND jizzed_at >= ?`
		args = append(args, from)
	}
	query += ` GROUP BY scene_id) e
		JOIN scenes s ON s.id = e.scene_id AND s.deleted_at IS NULL AND s.trashed_at IS NULL
		ORDER BY e.count DESC, s.id ASC
		LIMIT ?`
	args = append(args, limit)
	if err := db.Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// QueryMostJizzedSceneInADay returns the scene with the most o on a single local day.
func QueryMostJizzedSceneInADay(db *gorm.DB, userID uint, tz string) (*JizzSceneCount, error) {
	var rows []JizzSceneCount
	err := db.Raw(`
		SELECT s.id, COALESCE(s.title, '') AS title, COALESCE(s.thumbnail_path, '') AS thumbnail_path,
		       s.updated_at, e.count, e.day
		FROM (SELECT scene_id, to_char((jizzed_at AT TIME ZONE ?)::date, 'YYYY-MM-DD') AS day, COUNT(*) AS count
		      FROM user_scene_jizz_events WHERE user_id = ?
		      GROUP BY 1, 2) e
		JOIN scenes s ON s.id = e.scene_id AND s.deleted_at IS NULL AND s.trashed_at IS NULL
		ORDER BY e.count DESC, e.day ASC, s.id ASC
		LIMIT 1`, tz, userID).Scan(&rows).Error
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}

// JizzedSceneShare counts the library scenes the user has o'd (any count, dated or not).
type JizzedSceneShare struct {
	JizzedScenes int64 `gorm:"column:jizzed_scenes"`
	LibraryCount int64 `gorm:"column:library_count"`
}

// QueryJizzedSceneShare returns how many library scenes the user has o'd.
func QueryJizzedSceneShare(db *gorm.DB, userID uint) (*JizzedSceneShare, error) {
	var share JizzedSceneShare
	err := db.Raw(`
		SELECT (SELECT COUNT(*) FROM user_scene_jizzed j
		          JOIN scenes s ON s.id = j.scene_id AND s.deleted_at IS NULL AND s.trashed_at IS NULL
		         WHERE j.user_id = ? AND j.count > 0) AS jizzed_scenes,
		       (SELECT COUNT(*) FROM scenes WHERE deleted_at IS NULL AND trashed_at IS NULL) AS library_count`,
		userID).Scan(&share).Error
	if err != nil {
		return nil, err
	}
	return &share, nil
}

// TopJizzedScenes returns the scenes with the most of the user's dated o in [from, to), as
// SceneViewCount rows (Views holds the o count), most first then most recent o first.
func (q DisplayWidgetQueries) TopJizzedScenes(userID uint, from, to time.Time, limit int) ([]SceneViewCount, error) {
	var rows []SceneViewCount
	err := q.DB.Raw(`
		SELECT e.scene_id, COUNT(*) AS views
		FROM user_scene_jizz_events e
		JOIN scenes s ON s.id = e.scene_id AND s.deleted_at IS NULL AND s.trashed_at IS NULL
		WHERE e.user_id = ? AND e.jizzed_at >= ? AND e.jizzed_at < ?
		GROUP BY e.scene_id
		ORDER BY views DESC, MAX(e.jizzed_at) DESC
		LIMIT ?
	`, userID, from, to, limit).Scan(&rows).Error
	return rows, err
}

// JizzCountsForScenes returns the user's dated o of the given scenes in [from, to).
func (q DisplayWidgetQueries) JizzCountsForScenes(userID uint, sceneIDs []uint, from, to time.Time) (map[uint]int64, error) {
	counts := make(map[uint]int64, len(sceneIDs))
	if len(sceneIDs) == 0 {
		return counts, nil
	}
	var rows []SceneViewCount
	err := q.DB.Raw(`
		SELECT e.scene_id, COUNT(*) AS views
		FROM user_scene_jizz_events e
		WHERE e.user_id = ? AND e.scene_id IN ? AND e.jizzed_at >= ? AND e.jizzed_at < ?
		GROUP BY e.scene_id
	`, userID, sceneIDs, from, to).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		counts[r.SceneID] = r.Views
	}
	return counts, nil
}

// CountJizz returns the user's number of dated o in [from, to).
func (q DisplayWidgetQueries) CountJizz(userID uint, from, to time.Time) (int64, error) {
	var count int64
	err := q.DB.Raw(`
		SELECT COUNT(*) FROM user_scene_jizz_events e
		WHERE e.user_id = ? AND e.jizzed_at >= ? AND e.jizzed_at < ?
	`, userID, from, to).Scan(&count).Error
	return count, err
}
