package data

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// InsightsWidgetQueries holds the SQL of the homepage insight widgets (trends, top performers,
// curation to-do, processing status). Scenes in the trash or soft deleted are always excluded.
type InsightsWidgetQueries struct {
	DB *gorm.DB
}

// NewInsightsWidgetQueries creates the queries helper.
func NewInsightsWidgetQueries(db *gorm.DB) *InsightsWidgetQueries {
	return &InsightsWidgetQueries{DB: db}
}

// InsightsTrendRow is the view count of a tag or actor in the recent and baseline windows.
type InsightsTrendRow struct {
	ID          uint   `json:"id"`
	UUID        string `json:"uuid,omitempty"`
	Name        string `json:"name"`
	Color       string `json:"color,omitempty"`
	RecentViews int64  `json:"recent_views"`
	BaseViews   int64  `json:"base_views"`
}

// InsightsViewTotals is the number of views of a user in the recent and baseline windows.
type InsightsViewTotals struct {
	Recent int64
	Base   int64
}

// insightsLiveSceneSQL restricts a scenes alias to scenes neither trashed nor deleted.
func insightsLiveSceneSQL(alias string) string {
	return alias + ".trashed_at IS NULL AND " + alias + ".deleted_at IS NULL"
}

// ViewTotals counts the user's views in the recent window [recentFrom, now] and the baseline
// window [baseFrom, recentFrom).
func (q *InsightsWidgetQueries) ViewTotals(userID uint, baseFrom, recentFrom time.Time) (InsightsViewTotals, error) {
	var out InsightsViewTotals
	err := q.DB.Raw(`
		SELECT
			COUNT(*) FILTER (WHERE w.watched_at >= ?) AS recent,
			COUNT(*) FILTER (WHERE w.watched_at < ?) AS base
		FROM user_scene_watches w
		JOIN scenes s ON s.id = w.scene_id AND `+insightsLiveSceneSQL("s")+`
		WHERE w.user_id = ? AND w.watched_at >= ?`,
		recentFrom, recentFrom, userID, baseFrom).Scan(&out).Error
	return out, err
}

// TagTrends returns, per tag, the user's views in the recent and baseline windows.
func (q *InsightsWidgetQueries) TagTrends(userID uint, baseFrom, recentFrom time.Time) ([]InsightsTrendRow, error) {
	var rows []InsightsTrendRow
	err := q.DB.Raw(`
		SELECT t.id, t.name, t.color,
			COUNT(*) FILTER (WHERE w.watched_at >= ?) AS recent_views,
			COUNT(*) FILTER (WHERE w.watched_at < ?) AS base_views
		FROM user_scene_watches w
		JOIN scenes s ON s.id = w.scene_id AND `+insightsLiveSceneSQL("s")+`
		JOIN scene_tags st ON st.scene_id = w.scene_id
		JOIN tags t ON t.id = st.tag_id
		WHERE w.user_id = ? AND w.watched_at >= ?
		GROUP BY t.id, t.name, t.color`,
		recentFrom, recentFrom, userID, baseFrom).Scan(&rows).Error
	return rows, err
}

// ActorTrends returns, per actor, the user's views in the recent and baseline windows.
func (q *InsightsWidgetQueries) ActorTrends(userID uint, baseFrom, recentFrom time.Time) ([]InsightsTrendRow, error) {
	var rows []InsightsTrendRow
	err := q.DB.Raw(`
		SELECT a.id, a.uuid::text AS uuid, a.name,
			COUNT(*) FILTER (WHERE w.watched_at >= ?) AS recent_views,
			COUNT(*) FILTER (WHERE w.watched_at < ?) AS base_views
		FROM user_scene_watches w
		JOIN scenes s ON s.id = w.scene_id AND `+insightsLiveSceneSQL("s")+`
		JOIN scene_actors sa ON sa.scene_id = w.scene_id
		JOIN actors a ON a.id = sa.actor_id AND a.deleted_at IS NULL
		WHERE w.user_id = ? AND w.watched_at >= ?
		GROUP BY a.id, a.uuid, a.name`,
		recentFrom, recentFrom, userID, baseFrom).Scan(&rows).Error
	return rows, err
}

// InsightsPerformerRow is a performer with the user's statistics on their scenes.
type InsightsPerformerRow struct {
	ID          uint       `json:"id"`
	UUID        string     `json:"uuid"`
	Name        string     `json:"name"`
	ImageURL    string     `json:"image_url"`
	SceneCount  int64      `json:"scene_count"`
	Views       int64      `json:"views"`
	Jizz        int64      `json:"jizz"`
	Liked       int64      `json:"liked"`
	LastWatched *time.Time `json:"last_watched,omitempty"`
}

var insightsPerformerOrderSQL = map[string]string{
	"views":  "views DESC, jizz DESC",
	"jizz":   "jizz DESC, views DESC",
	"scenes": "scene_count DESC, views DESC",
}

// TopPerformers returns the user's top performers ordered by views, jizz or scene count.
// For views/jizz orders, performers without any are left out.
func (q *InsightsWidgetQueries) TopPerformers(userID uint, order string, limit int) ([]InsightsPerformerRow, error) {
	orderSQL, ok := insightsPerformerOrderSQL[order]
	if !ok {
		return nil, fmt.Errorf("unknown performer order %q", order)
	}
	having := ""
	switch order {
	case "views":
		having = "HAVING COALESCE(SUM(v.views), 0) > 0"
	case "jizz":
		having = "HAVING COALESCE(SUM(j.count), 0) > 0"
	}
	var rows []InsightsPerformerRow
	err := q.DB.Raw(`
		SELECT a.id, a.uuid::text AS uuid, a.name, COALESCE(a.image_url, '') AS image_url,
			COUNT(DISTINCT s.id) AS scene_count,
			COALESCE(SUM(v.views), 0) AS views,
			COALESCE(SUM(j.count), 0) AS jizz,
			COUNT(l.scene_id) AS liked,
			MAX(v.last_watched) AS last_watched
		FROM actors a
		JOIN scene_actors sa ON sa.actor_id = a.id
		JOIN scenes s ON s.id = sa.scene_id AND `+insightsLiveSceneSQL("s")+`
		LEFT JOIN (
			SELECT scene_id, COUNT(*) AS views, MAX(watched_at) AS last_watched
			FROM user_scene_watches WHERE user_id = ? GROUP BY scene_id
		) v ON v.scene_id = s.id
		LEFT JOIN user_scene_jizzed j ON j.scene_id = s.id AND j.user_id = ?
		LEFT JOIN user_scene_likes l ON l.scene_id = s.id AND l.user_id = ?
		WHERE a.deleted_at IS NULL
		GROUP BY a.id, a.uuid, a.name, a.image_url
		`+having+`
		ORDER BY `+orderSQL+`, a.name ASC
		LIMIT ?`,
		userID, userID, userID, limit).Scan(&rows).Error
	return rows, err
}

// InsightsCurationCounts counts the scenes missing metadata or generated files.
type InsightsCurationCounts struct {
	Total         int64 `json:"total"`
	NoTags        int64 `json:"no_tags"`
	NoActors      int64 `json:"no_actors"`
	NoStudio      int64 `json:"no_studio"`
	NoThumbnail   int64 `json:"no_thumbnail"`
	NoSprites     int64 `json:"no_sprites"`
	WatchedNoRate int64 `json:"watched_not_rated"`
}

// CurationCounts counts library scenes missing tags, actors, studio, thumbnail or sprites, and
// the scenes the user watched without rating them.
func (q *InsightsWidgetQueries) CurationCounts(userID uint) (InsightsCurationCounts, error) {
	var out InsightsCurationCounts
	err := q.DB.Raw(`
		SELECT
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE NOT EXISTS (SELECT 1 FROM scene_tags st WHERE st.scene_id = s.id)) AS no_tags,
			COUNT(*) FILTER (WHERE NOT EXISTS (SELECT 1 FROM scene_actors sa WHERE sa.scene_id = s.id)) AS no_actors,
			COUNT(*) FILTER (WHERE s.studio_id IS NULL AND COALESCE(s.studio, '') = '') AS no_studio,
			COUNT(*) FILTER (WHERE COALESCE(s.thumbnail_path, '') = '') AS no_thumbnail,
			COUNT(*) FILTER (WHERE COALESCE(s.sprite_sheet_path, '') = '') AS no_sprites
		FROM scenes s
		WHERE ` + insightsLiveSceneSQL("s")).Scan(&out).Error
	if err != nil {
		return out, err
	}
	err = q.DB.Raw(`
		SELECT COUNT(DISTINCT w.scene_id)
		FROM user_scene_watches w
		JOIN scenes s ON s.id = w.scene_id AND `+insightsLiveSceneSQL("s")+`
		WHERE w.user_id = ?
			AND NOT EXISTS (SELECT 1 FROM user_scene_ratings r WHERE r.user_id = ? AND r.scene_id = w.scene_id)`,
		userID, userID).Scan(&out.WatchedNoRate).Error
	return out, err
}

// InsightsJobCount is the number of jobs of a phase in a status.
type InsightsJobCount struct {
	Phase  string `json:"phase"`
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

// ActiveJobCounts counts pending and running jobs per phase, and the jobs failed since failedSince.
func (q *InsightsWidgetQueries) ActiveJobCounts(failedSince time.Time) ([]InsightsJobCount, error) {
	var rows []InsightsJobCount
	err := q.DB.Raw(`
		SELECT phase, status, COUNT(*) AS count
		FROM job_history
		WHERE status IN ('pending', 'running')
			OR (status = 'failed' AND COALESCE(completed_at, created_at) >= ?)
		GROUP BY phase, status
		ORDER BY phase, status`, failedSince).Scan(&rows).Error
	return rows, err
}

// InsightsCoverage counts the library scenes having each generated asset.
type InsightsCoverage struct {
	Total       int64 `json:"total"`
	Thumbnail   int64 `json:"thumbnail"`
	Sprites     int64 `json:"sprites"`
	Preview     int64 `json:"preview"`
	Fingerprint int64 `json:"fingerprint"`
}

// Coverage counts the library scenes having a thumbnail, sprites, a preview and a fingerprint.
func (q *InsightsWidgetQueries) Coverage() (InsightsCoverage, error) {
	var out InsightsCoverage
	err := q.DB.Raw(`
		SELECT
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE COALESCE(s.thumbnail_path, '') <> '') AS thumbnail,
			COUNT(*) FILTER (WHERE COALESCE(s.sprite_sheet_path, '') <> '') AS sprites,
			COUNT(*) FILTER (WHERE COALESCE(s.preview_video_path, '') <> '') AS preview,
			COUNT(*) FILTER (WHERE s.fingerprint_type IS NOT NULL) AS fingerprint
		FROM scenes s
		WHERE ` + insightsLiveSceneSQL("s")).Scan(&out).Error
	return out, err
}
