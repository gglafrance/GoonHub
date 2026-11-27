package data

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// Queries of the scene based homepage widgets (discover, rediscover, roulette, on_this_day,
// folder_latest, because_you_watched, showcase_4k, long_form). They return scene IDs (in the
// widget order) that the caller loads with SceneRepository.GetByIDs.

// HomepageSceneFilter restricts the candidate scenes of a widget. Zero values disable a filter.
type HomepageSceneFilter struct {
	MinDuration   int
	MaxDuration   int
	MinHeight     int
	StoragePathID uint
	// FolderPrefix matches stored_path: an absolute path is a prefix, otherwise it matches a
	// folder name anywhere in the path ("/<folder>/").
	Folder string
	// Tags keeps scenes having at least one of these tags (by name).
	Tags []string
}

// HomepageSceneOrder is the order of a scene ID listing.
type HomepageSceneOrder string

const (
	HomepageSceneOrderRandom HomepageSceneOrder = "random"
	HomepageSceneOrderNewest HomepageSceneOrder = "newest"
)

// HomepageSceneQueries runs the widget queries.
type HomepageSceneQueries struct {
	DB *gorm.DB
}

// NewHomepageSceneQueries creates the widget queries.
func NewHomepageSceneQueries(db *gorm.DB) *HomepageSceneQueries {
	return &HomepageSceneQueries{DB: db}
}

// homepageSceneEscapeLike escapes the LIKE wildcards of a literal.
func homepageSceneEscapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// filterSQL returns the " AND ..." conditions of a filter on the scenes table aliased s.
func (f HomepageSceneFilter) filterSQL() (string, []any) {
	sql := " AND s.deleted_at IS NULL AND s.trashed_at IS NULL"
	var args []any
	if f.MinDuration > 0 {
		sql += " AND s.duration >= ?"
		args = append(args, f.MinDuration)
	}
	if f.MaxDuration > 0 {
		sql += " AND s.duration <= ?"
		args = append(args, f.MaxDuration)
	}
	if f.MinHeight > 0 {
		sql += " AND s.height >= ?"
		args = append(args, f.MinHeight)
	}
	if f.StoragePathID > 0 {
		sql += " AND s.storage_path_id = ?"
		args = append(args, f.StoragePathID)
	}
	if folder := strings.TrimSpace(f.Folder); folder != "" {
		if strings.HasPrefix(folder, "/") {
			sql += ` AND s.stored_path LIKE ? ESCAPE '\'`
			args = append(args, homepageSceneEscapeLike(strings.TrimRight(folder, "/"))+"/%")
		} else {
			sql += ` AND s.stored_path LIKE ? ESCAPE '\'`
			args = append(args, "%/"+homepageSceneEscapeLike(strings.Trim(folder, "/"))+"/%")
		}
	}
	if len(f.Tags) > 0 {
		sql += ` AND EXISTS (SELECT 1 FROM scene_tags st JOIN tags t ON t.id = st.tag_id
			WHERE st.scene_id = s.id AND t.name IN ?)`
		args = append(args, f.Tags)
	}
	return sql, args
}

// SceneIDs lists scene IDs matching the filter. With unwatchedOnly, scenes the user ever
// watched are excluded. Returns the IDs (at most limit) and the total number of matches.
func (q *HomepageSceneQueries) SceneIDs(userID uint, f HomepageSceneFilter, unwatchedOnly bool, order HomepageSceneOrder, limit int) ([]uint, int64, error) {
	where, args := f.filterSQL()
	if unwatchedOnly {
		where += " AND NOT EXISTS (SELECT 1 FROM user_scene_watches w WHERE w.user_id = ? AND w.scene_id = s.id)"
		args = append(args, userID)
	}

	var total int64
	if err := q.DB.Raw("SELECT COUNT(*) FROM scenes s WHERE TRUE"+where, args...).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 || limit <= 0 {
		return []uint{}, total, nil
	}

	orderBy := "random()"
	if order == HomepageSceneOrderNewest {
		orderBy = "s.created_at DESC, s.id DESC"
	}
	var ids []uint
	err := q.DB.Raw("SELECT s.id FROM scenes s WHERE TRUE"+where+" ORDER BY "+orderBy+" LIMIT ?",
		append(args, limit)...).Scan(&ids).Error
	return ids, total, err
}

// RediscoverRow is a favorite scene not watched recently.
type RediscoverRow struct {
	SceneID     uint
	JizzCount   int
	Rating      float64
	Liked       bool
	Views       int
	LastWatched *time.Time
}

// RediscoverOptions selects the favorites of RediscoverScenes.
type RediscoverOptions struct {
	// NotWatchedSince keeps scenes not watched since this time (or never watched).
	NotWatchedSince time.Time
	// MinRating includes scenes rated at least this (0 disables the criterion).
	MinRating     float64
	IncludeLiked  bool
	IncludeJizzed bool
}

// RediscoverScenes lists the user's favorites (liked, well rated or jizzed) not watched since
// a date, strongest first (jizz count, rating, views) then the longest unseen.
func (q *HomepageSceneQueries) RediscoverScenes(userID uint, opts RediscoverOptions, limit int) ([]RediscoverRow, error) {
	if !opts.IncludeLiked && !opts.IncludeJizzed && opts.MinRating <= 0 {
		return []RediscoverRow{}, nil
	}
	var rows []RediscoverRow
	err := q.DB.Raw(`
		WITH fav AS (
			SELECT s.id AS scene_id,
				COALESCE(j.count, 0) AS jizz_count,
				COALESCE(r.rating, 0) AS rating,
				(l.scene_id IS NOT NULL) AS liked,
				(SELECT COUNT(*) FROM user_scene_watches w WHERE w.user_id = ? AND w.scene_id = s.id) AS views,
				(SELECT MAX(w.watched_at) FROM user_scene_watches w WHERE w.user_id = ? AND w.scene_id = s.id) AS last_watched
			FROM scenes s
			LEFT JOIN user_scene_jizzed j ON j.scene_id = s.id AND j.user_id = ?
			LEFT JOIN user_scene_ratings r ON r.scene_id = s.id AND r.user_id = ?
			LEFT JOIN user_scene_likes l ON l.scene_id = s.id AND l.user_id = ?
			WHERE s.deleted_at IS NULL AND s.trashed_at IS NULL
				AND ((?::boolean AND l.scene_id IS NOT NULL)
					OR (?::boolean AND COALESCE(j.count, 0) > 0)
					OR (?::numeric > 0 AND COALESCE(r.rating, 0) >= ?::numeric))
		)
		SELECT scene_id, jizz_count, rating, liked, views, last_watched FROM fav
		WHERE last_watched IS NULL OR last_watched < ?
		ORDER BY jizz_count DESC, rating DESC, views DESC, last_watched ASC NULLS FIRST, scene_id
		LIMIT ?`,
		userID, userID, userID, userID, userID,
		opts.IncludeLiked, opts.IncludeJizzed, opts.MinRating, opts.MinRating,
		opts.NotWatchedSince, limit,
	).Scan(&rows).Error
	return rows, err
}

// OnThisDayRow is a scene watched or added on the same month-day in a previous year.
type OnThisDayRow struct {
	SceneID uint
	Year    int
	// Kind is "watched" or "added"
	Kind string
}

// OnThisDay lists scenes watched (watched=true) and/or added (added=true) on the month-day of
// `day` in years before it, most recent years first.
func (q *HomepageSceneQueries) OnThisDay(userID uint, day time.Time, watched, added bool, limit int) ([]OnThisDayRow, error) {
	month, dom, year := int(day.Month()), day.Day(), day.Year()
	var parts []string
	var args []any
	if watched {
		parts = append(parts, `
			SELECT w.scene_id, EXTRACT(YEAR FROM w.watched_at)::int AS year, 'watched' AS kind,
				MAX(w.watched_at) AS at
			FROM user_scene_watches w
			JOIN scenes s ON s.id = w.scene_id AND s.deleted_at IS NULL AND s.trashed_at IS NULL
			WHERE w.user_id = ? AND EXTRACT(MONTH FROM w.watched_at) = ?
				AND EXTRACT(DAY FROM w.watched_at) = ? AND EXTRACT(YEAR FROM w.watched_at) < ?
			GROUP BY w.scene_id, EXTRACT(YEAR FROM w.watched_at)`)
		args = append(args, userID, month, dom, year)
	}
	if added {
		parts = append(parts, `
			SELECT s.id AS scene_id, EXTRACT(YEAR FROM s.created_at)::int AS year, 'added' AS kind,
				s.created_at AS at
			FROM scenes s
			WHERE s.deleted_at IS NULL AND s.trashed_at IS NULL
				AND EXTRACT(MONTH FROM s.created_at) = ? AND EXTRACT(DAY FROM s.created_at) = ?
				AND EXTRACT(YEAR FROM s.created_at) < ?`)
		args = append(args, month, dom, year)
	}
	if len(parts) == 0 {
		return []OnThisDayRow{}, nil
	}
	var rows []OnThisDayRow
	err := q.DB.Raw(`SELECT scene_id, year, kind FROM (`+strings.Join(parts, " UNION ALL ")+`) x
		ORDER BY year DESC, kind DESC, at DESC LIMIT ?`, append(args, limit)...).Scan(&rows).Error
	return rows, err
}

// RecentlyWatchedSceneIDs returns the user's most recently watched distinct scenes.
func (q *HomepageSceneQueries) RecentlyWatchedSceneIDs(userID uint, limit int) ([]uint, error) {
	var ids []uint
	err := q.DB.Raw(`
		SELECT w.scene_id FROM user_scene_watches w
		JOIN scenes s ON s.id = w.scene_id AND s.deleted_at IS NULL AND s.trashed_at IS NULL
		WHERE w.user_id = ?
		GROUP BY w.scene_id
		ORDER BY MAX(w.watched_at) DESC
		LIMIT ?`, userID, limit).Scan(&ids).Error
	return ids, err
}

// WatchedSceneIDs returns which of the given scenes the user has watched.
func (q *HomepageSceneQueries) WatchedSceneIDs(userID uint, sceneIDs []uint) (map[uint]bool, error) {
	watched := make(map[uint]bool)
	if len(sceneIDs) == 0 {
		return watched, nil
	}
	var ids []uint
	if err := q.DB.Raw(`SELECT DISTINCT scene_id FROM user_scene_watches WHERE user_id = ? AND scene_id IN ?`,
		userID, sceneIDs).Scan(&ids).Error; err != nil {
		return nil, err
	}
	for _, id := range ids {
		watched[id] = true
	}
	return watched, nil
}

// InProgressRow is a scene whose latest watch stopped before the end.
type InProgressRow struct {
	SceneID      uint
	LastPosition int
}

// InProgressScenes lists scenes matching the filter whose latest watch by the user is not
// completed and stopped before maxProgress (0..1) of the duration, most recent first.
func (q *HomepageSceneQueries) InProgressScenes(userID uint, f HomepageSceneFilter, maxProgress float64, limit int) ([]InProgressRow, error) {
	where, args := f.filterSQL()
	var rows []InProgressRow
	err := q.DB.Raw(`
		SELECT scene_id, last_position FROM (
			SELECT DISTINCT ON (w.scene_id) w.scene_id, w.last_position, w.completed, w.watched_at, s.duration
			FROM user_scene_watches w
			JOIN scenes s ON s.id = w.scene_id
			WHERE w.user_id = ?`+where+`
			ORDER BY w.scene_id, w.watched_at DESC
		) latest
		WHERE NOT completed AND last_position > 0 AND last_position < duration * ?::double precision
		ORDER BY watched_at DESC
		LIMIT ?`, append(append([]any{userID}, args...), maxProgress, limit)...).Scan(&rows).Error
	return rows, err
}
