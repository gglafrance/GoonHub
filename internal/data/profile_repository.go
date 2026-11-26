package data

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ProfileRepository stores the profile pages (user_profiles and the showcases) and runs the
// queries of the profile blocks (docs/PROFILES.md). Scene queries skip trashed and deleted
// scenes; marker queries skip the markers of such scenes.
type ProfileRepository interface {
	// GetProfile returns the profile of a user, DefaultUserProfile when they never edited it.
	GetProfile(ctx context.Context, userID uint) (*UserProfile, error)
	// SaveProfile writes the display name and bio of a user (creating the row if needed).
	SaveProfile(ctx context.Context, userID uint, displayName, bio string) error
	// SetAvatarVersion sets the avatar version (0 = no avatar).
	SetAvatarVersion(ctx context.Context, userID uint, version int64) error
	// SetBanner sets the banner (type, scene for the scene type, version for the upload type).
	SetBanner(ctx context.Context, userID uint, bannerType string, sceneID *uint, version int64) error
	// SaveLayout stores the block layout (nil = back to the default layout).
	SaveLayout(ctx context.Context, userID uint, layout ProfileLayout) error

	// FavoriteSceneIDs and FavoriteMarkerIDs return the showcases in order (everything
	// stored, live or not).
	FavoriteSceneIDs(ctx context.Context, userID uint) ([]uint, error)
	FavoriteMarkerIDs(ctx context.Context, userID uint) ([]uint, error)
	// SetFavoriteScenes and SetFavoriteMarkers replace a showcase (ids in order).
	SetFavoriteScenes(ctx context.Context, userID uint, sceneIDs []uint) error
	SetFavoriteMarkers(ctx context.Context, userID uint, markerIDs []uint) error

	// LiveSceneIDs returns the ids among sceneIDs of scenes neither trashed nor deleted.
	LiveSceneIDs(ctx context.Context, sceneIDs []uint) (map[uint]bool, error)
	// ScenesByIDs returns the live scenes among the ids, in the order of the ids.
	ScenesByIDs(ctx context.Context, sceneIDs []uint) ([]Scene, error)
	// VisibleMarkers returns, in the order of the ids, the markers among markerIDs that the
	// user sees (their own and the shared ones, VisibleMarkersSQL) on live scenes.
	VisibleMarkers(ctx context.Context, userID uint, markerIDs []uint) ([]WidgetMarker, error)

	// RecentWatches returns the scenes the user watched last (one row per scene, At = last
	// watch), most recent first.
	RecentWatches(ctx context.Context, userID uint, limit int) ([]ProfileSceneRef, error)
	// MostWatched returns the scenes the user watched the most since `since` (zero: all
	// time), Count = views.
	MostWatched(ctx context.Context, userID uint, since time.Time, limit int) ([]ProfileSceneRef, error)
	// LikedScenes returns the scenes the user liked, most recent first (At = like date), and
	// the number of live liked scenes.
	LikedScenes(ctx context.Context, userID uint, limit int) ([]ProfileSceneRef, int64, error)
	// RecentMarkers returns the markers the user created (their own only), newest first.
	RecentMarkers(ctx context.Context, userID uint, limit int) ([]WidgetMarker, error)
	// Stats returns the figures of the stats block since `since` (zero: all time).
	Stats(ctx context.Context, userID uint, since time.Time) (*ProfileStats, error)
	// TopTags, TopActors and TopStudios rank by the user's views since `since`.
	TopTags(ctx context.Context, userID uint, since time.Time, limit int) ([]ProfileTopEntry, error)
	TopActors(ctx context.Context, userID uint, since time.Time, limit int) ([]ProfileTopEntry, error)
	TopStudios(ctx context.Context, userID uint, since time.Time, limit int) ([]ProfileTopEntry, error)
	// PublicPlaylists returns the user's public playlists, last updated first, and their count.
	PublicPlaylists(ctx context.Context, userID uint, limit int) ([]ProfilePlaylist, int64, error)
	// RatingSummary returns the user's ratings per half-star bucket (non-empty ones,
	// ascending) and their average, over live scenes.
	RatingSummary(ctx context.Context, userID uint) (*ProfileRatingSummary, error)
	// RatedScenes returns a page of the scenes the user rated (Rating set, At = rating date),
	// optionally one bucket, and their number.
	RatedScenes(ctx context.Context, userID uint, q ProfileListQuery) ([]ProfileSceneRef, int64, error)
	// LikedScenesPage returns a page of the scenes the user liked (At = like date) and their
	// number.
	LikedScenesPage(ctx context.Context, userID uint, q ProfileListQuery) ([]ProfileSceneRef, int64, error)
	// JizzScenes returns a page of the scenes the user jizzed to (Count = O count, At = last
	// O) and their number.
	JizzScenes(ctx context.Context, userID uint, q ProfileListQuery) ([]ProfileSceneRef, int64, error)
	// JizzSummary returns the user's O total and number of scenes, over live scenes.
	JizzSummary(ctx context.Context, userID uint) (*ProfileJizzSummary, error)
	// MarkerCandidates lists the markers the user sees (own and shared) on live scenes whose
	// label or scene title contains the query, newest first (the favorite markers picker).
	MarkerCandidates(ctx context.Context, userID uint, query string, page, limit int) ([]WidgetMarker, int64, error)
}

type ProfileRepositoryImpl struct {
	DB *gorm.DB
}

func NewProfileRepository(db *gorm.DB) *ProfileRepositoryImpl {
	return &ProfileRepositoryImpl{DB: db}
}

// liveSceneSQL keeps the scenes (aliased as alias) neither trashed nor deleted.
func liveSceneSQL(alias string) string {
	return alias + ".deleted_at IS NULL AND " + alias + ".trashed_at IS NULL"
}

func (r *ProfileRepositoryImpl) GetProfile(ctx context.Context, userID uint) (*UserProfile, error) {
	var p UserProfile
	err := r.DB.WithContext(ctx).Where("user_id = ?", userID).Take(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return DefaultUserProfile(userID), nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// upsert creates the profile row of a user if needed, then applies the updates.
func (r *ProfileRepositoryImpl) upsert(ctx context.Context, userID uint, updates map[string]any) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(DefaultUserProfile(userID)).Error; err != nil {
			return err
		}
		updates["updated_at"] = time.Now()
		return tx.Model(&UserProfile{}).Where("user_id = ?", userID).Updates(updates).Error
	})
}

func (r *ProfileRepositoryImpl) SaveProfile(ctx context.Context, userID uint, displayName, bio string) error {
	return r.upsert(ctx, userID, map[string]any{"display_name": displayName, "bio": bio})
}

func (r *ProfileRepositoryImpl) SetAvatarVersion(ctx context.Context, userID uint, version int64) error {
	return r.upsert(ctx, userID, map[string]any{"avatar_version": version})
}

func (r *ProfileRepositoryImpl) SetBanner(ctx context.Context, userID uint, bannerType string, sceneID *uint, version int64) error {
	return r.upsert(ctx, userID, map[string]any{"banner_type": bannerType, "banner_scene_id": sceneID, "banner_version": version})
}

func (r *ProfileRepositoryImpl) SaveLayout(ctx context.Context, userID uint, layout ProfileLayout) error {
	return r.upsert(ctx, userID, map[string]any{"layout": layout})
}

func (r *ProfileRepositoryImpl) ids(ctx context.Context, query string, args ...any) ([]uint, error) {
	var ids []uint
	if err := r.DB.WithContext(ctx).Raw(query, args...).Scan(&ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *ProfileRepositoryImpl) FavoriteSceneIDs(ctx context.Context, userID uint) ([]uint, error) {
	return r.ids(ctx, `SELECT scene_id FROM user_profile_favorite_scenes WHERE user_id = ? ORDER BY position`, userID)
}

func (r *ProfileRepositoryImpl) FavoriteMarkerIDs(ctx context.Context, userID uint) ([]uint, error) {
	return r.ids(ctx, `SELECT marker_id FROM user_profile_favorite_markers WHERE user_id = ? ORDER BY position`, userID)
}

// replaceShowcase replaces the rows of a showcase table in one transaction.
func (r *ProfileRepositoryImpl) replaceShowcase(ctx context.Context, table, column string, userID uint, ids []uint) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`DELETE FROM `+table+` WHERE user_id = ?`, userID).Error; err != nil {
			return err
		}
		for i, id := range ids {
			if err := tx.Exec(`INSERT INTO `+table+` (user_id, `+column+`, position) VALUES (?, ?, ?)`, userID, id, i).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *ProfileRepositoryImpl) SetFavoriteScenes(ctx context.Context, userID uint, sceneIDs []uint) error {
	return r.replaceShowcase(ctx, "user_profile_favorite_scenes", "scene_id", userID, sceneIDs)
}

func (r *ProfileRepositoryImpl) SetFavoriteMarkers(ctx context.Context, userID uint, markerIDs []uint) error {
	return r.replaceShowcase(ctx, "user_profile_favorite_markers", "marker_id", userID, markerIDs)
}

func (r *ProfileRepositoryImpl) LiveSceneIDs(ctx context.Context, sceneIDs []uint) (map[uint]bool, error) {
	out := make(map[uint]bool, len(sceneIDs))
	if len(sceneIDs) == 0 {
		return out, nil
	}
	ids, err := r.ids(ctx, `SELECT s.id FROM scenes s WHERE s.id IN ? AND `+liveSceneSQL("s"), sceneIDs)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
}

func (r *ProfileRepositoryImpl) ScenesByIDs(ctx context.Context, sceneIDs []uint) ([]Scene, error) {
	if len(sceneIDs) == 0 {
		return []Scene{}, nil
	}
	var scenes []Scene
	if err := r.DB.WithContext(ctx).Where("id IN ? AND trashed_at IS NULL", sceneIDs).Find(&scenes).Error; err != nil {
		return nil, err
	}
	byID := make(map[uint]Scene, len(scenes))
	for _, s := range scenes {
		byID[s.ID] = s
	}
	out := make([]Scene, 0, len(scenes))
	for _, id := range sceneIDs {
		if s, ok := byID[id]; ok {
			out = append(out, s)
			delete(byID, id)
		}
	}
	return out, nil
}

// widgetMarkers selects markers in the WidgetMarker shape (m: markers, s: their scene).
func (r *ProfileRepositoryImpl) widgetMarkers(ctx context.Context) *gorm.DB {
	return r.DB.WithContext(ctx).Table("user_scene_markers AS m").
		Select("m.id, m.scene_id, COALESCE(s.title, '') AS scene_title, COALESCE(m.label, '') AS label, m.color, m.timestamp, m.end_timestamp, " +
			"COALESCE(m.thumbnail_path, '') AS thumbnail_path, COALESCE(m.animated_thumbnail_path, '') AS animated_thumbnail_path, m.updated_at").
		Joins("JOIN scenes s ON s.id = m.scene_id AND " + liveSceneSQL("s"))
}

func finishWidgetMarkers(markers []WidgetMarker) []WidgetMarker {
	if markers == nil {
		return []WidgetMarker{}
	}
	for i := range markers {
		markers[i].HasThumbnail = markers[i].ThumbnailPath != ""
		markers[i].HasAnimated = markers[i].AnimatedThumbnailPath != ""
	}
	return markers
}

func (r *ProfileRepositoryImpl) VisibleMarkers(ctx context.Context, userID uint, markerIDs []uint) ([]WidgetMarker, error) {
	if len(markerIDs) == 0 {
		return []WidgetMarker{}, nil
	}
	var markers []WidgetMarker
	if err := r.widgetMarkers(ctx).Where("m.id IN ? AND "+VisibleMarkersSQL("m"), markerIDs, userID).Scan(&markers).Error; err != nil {
		return nil, err
	}
	byID := make(map[uint]WidgetMarker, len(markers))
	for _, m := range markers {
		byID[m.ID] = m
	}
	out := make([]WidgetMarker, 0, len(markers))
	for _, id := range markerIDs {
		if m, ok := byID[id]; ok {
			out = append(out, m)
			delete(byID, id)
		}
	}
	return finishWidgetMarkers(out), nil
}

func (r *ProfileRepositoryImpl) RecentWatches(ctx context.Context, userID uint, limit int) ([]ProfileSceneRef, error) {
	var rows []ProfileSceneRef
	err := r.DB.WithContext(ctx).Raw(`
		SELECT w.scene_id, MAX(w.watched_at) AS at, COUNT(*) AS count
		FROM user_scene_watches w
		JOIN scenes s ON s.id = w.scene_id AND `+liveSceneSQL("s")+`
		WHERE w.user_id = ?
		GROUP BY w.scene_id
		ORDER BY at DESC, w.scene_id DESC
		LIMIT ?`, userID, limit).Scan(&rows).Error
	return rows, err
}

func (r *ProfileRepositoryImpl) MostWatched(ctx context.Context, userID uint, since time.Time, limit int) ([]ProfileSceneRef, error) {
	var rows []ProfileSceneRef
	err := r.DB.WithContext(ctx).Raw(`
		SELECT w.scene_id, MAX(w.watched_at) AS at, COUNT(*) AS count
		FROM user_scene_watches w
		JOIN scenes s ON s.id = w.scene_id AND `+liveSceneSQL("s")+`
		WHERE w.user_id = ? AND w.watched_at >= ?
		GROUP BY w.scene_id
		ORDER BY count DESC, at DESC, w.scene_id DESC
		LIMIT ?`, userID, since, limit).Scan(&rows).Error
	return rows, err
}

func (r *ProfileRepositoryImpl) LikedScenes(ctx context.Context, userID uint, limit int) ([]ProfileSceneRef, int64, error) {
	from := `FROM user_scene_likes l JOIN scenes s ON s.id = l.scene_id AND ` + liveSceneSQL("s") + ` WHERE l.user_id = ?`
	var total int64
	if err := r.DB.WithContext(ctx).Raw(`SELECT COUNT(*) `+from, userID).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []ProfileSceneRef
	if err := r.DB.WithContext(ctx).Raw(`SELECT l.scene_id, l.created_at AS at, 0 AS count `+from+
		` ORDER BY l.created_at DESC, l.scene_id DESC LIMIT ?`, userID, limit).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *ProfileRepositoryImpl) RecentMarkers(ctx context.Context, userID uint, limit int) ([]WidgetMarker, error) {
	var markers []WidgetMarker
	if err := r.widgetMarkers(ctx).Where("m.user_id = ?", userID).
		Order("m.created_at DESC, m.id DESC").Limit(limit).Scan(&markers).Error; err != nil {
		return nil, err
	}
	return finishWidgetMarkers(markers), nil
}

func (r *ProfileRepositoryImpl) Stats(ctx context.Context, userID uint, since time.Time) (*ProfileStats, error) {
	var st ProfileStats
	err := r.DB.WithContext(ctx).Raw(`
		SELECT
			(SELECT COALESCE(SUM(w.watch_duration), 0) FROM user_scene_watches w
			  WHERE w.user_id = ? AND w.watched_at >= ?) AS watch_time,
			(SELECT COUNT(*) FROM user_scene_watches w
			  WHERE w.user_id = ? AND w.watched_at >= ?) AS views,
			(SELECT COUNT(DISTINCT w.scene_id) FROM user_scene_watches w
			  JOIN scenes s ON s.id = w.scene_id AND `+liveSceneSQL("s")+`
			  WHERE w.user_id = ? AND w.watched_at >= ?) AS scenes_watched,
			(SELECT COUNT(*) FROM user_scene_jizz_events j
			  WHERE j.user_id = ? AND j.jizzed_at >= ?) AS jizz,
			(SELECT COUNT(*) FROM user_scene_likes l
			  JOIN scenes s ON s.id = l.scene_id AND `+liveSceneSQL("s")+` WHERE l.user_id = ?) AS liked,
			(SELECT COUNT(*) FROM user_scene_markers m
			  JOIN scenes s ON s.id = m.scene_id AND `+liveSceneSQL("s")+` WHERE m.user_id = ?) AS markers,
			(SELECT COUNT(*) FROM user_scene_ratings r WHERE r.user_id = ?) AS rated,
			(SELECT AVG(r.rating)::float8 FROM user_scene_ratings r WHERE r.user_id = ?) AS avg_rating`,
		userID, since, userID, since, userID, since, userID, since, userID, userID, userID, userID).Scan(&st).Error
	if err != nil {
		return nil, err
	}
	if since.IsZero() {
		// All time: the o totals also count the undated ones (user_scene_jizzed)
		var total int64
		if err := r.DB.WithContext(ctx).Raw(`SELECT COALESCE(SUM(count), 0) FROM user_scene_jizzed WHERE user_id = ?`, userID).Scan(&total).Error; err != nil {
			return nil, err
		}
		if total > st.Jizz {
			st.Jizz = total
		}
	}
	return &st, nil
}

// viewsSinceSQL is the user's views per live scene since a date (bound: user, since).
const viewsSinceSQL = `(SELECT w.scene_id, COUNT(*) AS views FROM user_scene_watches w
	JOIN scenes s ON s.id = w.scene_id AND s.deleted_at IS NULL AND s.trashed_at IS NULL
	WHERE w.user_id = ? AND w.watched_at >= ? GROUP BY w.scene_id) v`

func (r *ProfileRepositoryImpl) top(ctx context.Context, query string, userID uint, since time.Time, limit int) ([]ProfileTopEntry, error) {
	var rows []ProfileTopEntry
	if err := r.DB.WithContext(ctx).Raw(query, userID, since, limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []ProfileTopEntry{}
	}
	return rows, nil
}

func (r *ProfileRepositoryImpl) TopTags(ctx context.Context, userID uint, since time.Time, limit int) ([]ProfileTopEntry, error) {
	return r.top(ctx, `SELECT t.id, '' AS uuid, t.name, SUM(v.views) AS views FROM `+viewsSinceSQL+`
		JOIN scene_tags st ON st.scene_id = v.scene_id
		JOIN tags t ON t.id = st.tag_id
		GROUP BY t.id, t.name ORDER BY views DESC, t.name LIMIT ?`, userID, since, limit)
}

func (r *ProfileRepositoryImpl) TopActors(ctx context.Context, userID uint, since time.Time, limit int) ([]ProfileTopEntry, error) {
	return r.top(ctx, `SELECT a.id, a.uuid::text AS uuid, a.name, SUM(v.views) AS views FROM `+viewsSinceSQL+`
		JOIN scene_actors sa ON sa.scene_id = v.scene_id
		JOIN actors a ON a.id = sa.actor_id AND a.deleted_at IS NULL
		GROUP BY a.id, a.uuid, a.name ORDER BY views DESC, a.name LIMIT ?`, userID, since, limit)
}

func (r *ProfileRepositoryImpl) TopStudios(ctx context.Context, userID uint, since time.Time, limit int) ([]ProfileTopEntry, error) {
	return r.top(ctx, `SELECT st.id, st.uuid::text AS uuid, st.name, SUM(v.views) AS views FROM `+viewsSinceSQL+`
		JOIN scenes s ON s.id = v.scene_id
		JOIN studios st ON st.id = s.studio_id AND st.deleted_at IS NULL
		GROUP BY st.id, st.uuid, st.name ORDER BY views DESC, st.name LIMIT ?`, userID, since, limit)
}

func (r *ProfileRepositoryImpl) PublicPlaylists(ctx context.Context, userID uint, limit int) ([]ProfilePlaylist, int64, error) {
	var total int64
	if err := r.DB.WithContext(ctx).Raw(`SELECT COUNT(*) FROM playlists WHERE user_id = ? AND visibility = 'public'`, userID).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []ProfilePlaylist
	err := r.DB.WithContext(ctx).Raw(`
		SELECT p.uuid::text AS uuid, p.name, p.updated_at,
			(SELECT COUNT(*) FROM playlist_scenes ps JOIN scenes s ON s.id = ps.scene_id AND `+liveSceneSQL("s")+`
			  WHERE ps.playlist_id = p.id) AS scene_count,
			(SELECT ps.scene_id FROM playlist_scenes ps JOIN scenes s ON s.id = ps.scene_id AND `+liveSceneSQL("s")+`
			  WHERE ps.playlist_id = p.id ORDER BY ps.position, ps.id LIMIT 1) AS cover_scene_id
		FROM playlists p
		WHERE p.user_id = ? AND p.visibility = 'public'
		ORDER BY p.updated_at DESC, p.id DESC
		LIMIT ?`, userID, limit).Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *ProfileRepositoryImpl) MarkerCandidates(ctx context.Context, userID uint, query string, page, limit int) ([]WidgetMarker, int64, error) {
	if page < 1 {
		page = 1
	}
	where := VisibleMarkersSQL("m")
	args := []any{userID}
	if q := strings.TrimSpace(query); q != "" {
		like := "%" + escapeLike(q) + "%"
		where += ` AND (m.label ILIKE ? OR s.title ILIKE ?)`
		args = append(args, like, like)
	}
	var total int64
	if err := r.DB.WithContext(ctx).Raw(`SELECT COUNT(*) FROM user_scene_markers m JOIN scenes s ON s.id = m.scene_id AND `+
		liveSceneSQL("s")+` WHERE `+where, args...).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	var markers []WidgetMarker
	if err := r.widgetMarkers(ctx).Where(where, args...).Order("m.created_at DESC, m.id DESC").
		Limit(limit).Offset((page - 1) * limit).Scan(&markers).Error; err != nil {
		return nil, 0, err
	}
	return finishWidgetMarkers(markers), total, nil
}
