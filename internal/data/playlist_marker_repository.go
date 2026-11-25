package data

import (
	"errors"
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Marker playlists (docs/MARKER_PLAYLISTS.md): the playlist_markers rows of a playlist of kind
// "markers". A marker is in a playlist at most once (unique index), and leaves it when deleted
// (ON DELETE CASCADE). Only the markers of scenes neither trashed nor deleted are playable:
// the others are kept but not listed nor counted.

// playableMarkerSQL joins a marker (alias m) to its scene (alias s) and keeps it only when the
// scene is neither trashed nor deleted.
const playableMarkerJoinSQL = ` JOIN scenes s ON s.id = m.scene_id AND s.deleted_at IS NULL AND s.trashed_at IS NULL`

// MarkerPlayRange is the part of its scene that a marker plays in a marker playlist: from its
// timestamp to its end_timestamp, or, without an end, for defaultDuration seconds, cut at the
// end of the scene when its duration is known. The SQL twin is markerPlayLengthSQL.
func MarkerPlayRange(timestamp int, endTimestamp *int, sceneDuration int, defaultDuration int) (int, int) {
	if endTimestamp != nil {
		return timestamp, *endTimestamp
	}
	end := timestamp + defaultDuration
	if sceneDuration > timestamp && end > sceneDuration {
		end = sceneDuration
	}
	return timestamp, end
}

// markerPlayLengthSQL is the play length of a marker (alias m, scene alias s) in seconds, as
// MarkerPlayRange computes it. Its two arguments are the default duration.
const markerPlayLengthSQL = `(CASE
	WHEN m.end_timestamp IS NOT NULL THEN m.end_timestamp
	WHEN s.duration > m.timestamp THEN LEAST(m.timestamp + ?, s.duration)
	ELSE m.timestamp + ? END) - m.timestamp`

// playlistFullError is returned by AddMarkers when the playlist would exceed its size limit.
type playlistFullError struct{}

func (e *playlistFullError) Error() string { return "playlist is full" }

// ErrPlaylistFullSentinel returns the error AddMarkers fails with on a full playlist (tests).
func ErrPlaylistFullSentinel() error { return &playlistFullError{} }

// IsPlaylistFull reports whether err is the full playlist error of AddMarkers.
func IsPlaylistFull(err error) bool {
	var full *playlistFullError
	return errors.As(err, &full)
}

func uniqueUints(values []uint) []uint {
	seen := make(map[uint]bool, len(values))
	out := make([]uint, 0, len(values))
	for _, v := range values {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func toInt64s(values []uint) pq.Int64Array {
	out := make(pq.Int64Array, len(values))
	for i, v := range values {
		out[i] = int64(v)
	}
	return out
}

func (r *PlaylistRepositoryImpl) AddMarkers(playlistID uint, markerIDs []uint, maxItems int) (int, error) {
	ids := uniqueUints(markerIDs)
	if len(ids) == 0 {
		return 0, nil
	}
	added := 0
	err := r.DB.Transaction(func(tx *gorm.DB) error {
		// Locks the playlist: concurrent adds see each other's rows, so the size limit and
		// the positions hold
		var locked uint
		if err := tx.Raw("SELECT id FROM playlists WHERE id = ? FOR UPDATE", playlistID).Scan(&locked).Error; err != nil {
			return err
		}

		var existing []uint
		if err := tx.Model(&PlaylistMarker{}).
			Where("playlist_id = ? AND marker_id IN ?", playlistID, ids).
			Pluck("marker_id", &existing).Error; err != nil {
			return err
		}
		present := make(map[uint]bool, len(existing))
		for _, id := range existing {
			present[id] = true
		}
		fresh := make([]uint, 0, len(ids))
		for _, id := range ids {
			if !present[id] {
				fresh = append(fresh, id)
			}
		}
		if len(fresh) == 0 {
			return nil
		}

		if maxItems > 0 {
			var count int64
			if err := tx.Model(&PlaylistMarker{}).Where("playlist_id = ?", playlistID).Count(&count).Error; err != nil {
				return err
			}
			if count+int64(len(fresh)) > int64(maxItems) {
				return &playlistFullError{}
			}
		}

		var maxPos int
		if err := tx.Model(&PlaylistMarker{}).
			Where("playlist_id = ?", playlistID).
			Select("COALESCE(MAX(position), -1)").
			Scan(&maxPos).Error; err != nil {
			return err
		}

		now := time.Now()
		rows := make([]PlaylistMarker, len(fresh))
		for i, id := range fresh {
			rows[i] = PlaylistMarker{PlaylistID: playlistID, MarkerID: id, Position: maxPos + 1 + i, AddedAt: now}
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows)
		if result.Error != nil {
			return result.Error
		}
		added = int(result.RowsAffected)
		return nil
	})
	if err != nil {
		return 0, err
	}
	return added, nil
}

func (r *PlaylistRepositoryImpl) RemoveMarkers(playlistID uint, markerIDs []uint) (int64, error) {
	if len(markerIDs) == 0 {
		return 0, nil
	}
	result := r.DB.Where("playlist_id = ? AND marker_id IN ?", playlistID, markerIDs).Delete(&PlaylistMarker{})
	return result.RowsAffected, result.Error
}

func (r *PlaylistRepositoryImpl) ReorderMarkers(playlistID uint, markerIDs []uint) error {
	ids := uniqueUints(markerIDs)
	if len(ids) == 0 {
		return nil
	}
	// One statement: each listed marker gets its index in the list as position
	return r.DB.Exec(`
		UPDATE playlist_markers pm SET position = v.ord - 1
		FROM unnest(?::bigint[]) WITH ORDINALITY AS v(marker_id, ord)
		WHERE pm.playlist_id = ? AND pm.marker_id = v.marker_id`,
		toInt64s(ids), playlistID).Error
}

func (r *PlaylistRepositoryImpl) GetPlaylistMarkers(playlistID uint) ([]PlaylistMarkerRow, error) {
	var rows []PlaylistMarkerRow
	err := r.DB.Raw(`
		SELECT pm.position, pm.added_at, m.id AS marker_id, m.user_id AS marker_user_id,
			m.scene_id, m.timestamp, m.end_timestamp, COALESCE(m.label, '') AS label, m.color,
			COALESCE(m.thumbnail_path, '') AS thumbnail_path, m.updated_at,
			s.title AS scene_title, s.duration AS scene_duration
		FROM playlist_markers pm
		JOIN user_scene_markers m ON m.id = pm.marker_id`+playableMarkerJoinSQL+`
		WHERE pm.playlist_id = ?
		ORDER BY pm.position ASC, pm.id ASC`, playlistID).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *PlaylistRepositoryImpl) HasMarker(playlistID uint, markerID uint) (bool, uint, error) {
	var sceneIDs []uint
	err := r.DB.Raw(`
		SELECT m.scene_id FROM playlist_markers pm
		JOIN user_scene_markers m ON m.id = pm.marker_id
		WHERE pm.playlist_id = ? AND pm.marker_id = ?`, playlistID, markerID).Scan(&sceneIDs).Error
	if err != nil {
		return false, 0, err
	}
	if len(sceneIDs) == 0 {
		return false, 0, nil
	}
	return true, sceneIDs[0], nil
}

func (r *PlaylistRepositoryImpl) GetMarkerStats(playlistID uint, defaultDuration int) (PlaylistMarkerStats, error) {
	var stats PlaylistMarkerStats
	err := r.DB.Raw(`
		SELECT COUNT(*) AS marker_count, COUNT(DISTINCT m.scene_id) AS scene_count,
			COALESCE(SUM(`+markerPlayLengthSQL+`), 0) AS total_duration
		FROM playlist_markers pm
		JOIN user_scene_markers m ON m.id = pm.marker_id`+playableMarkerJoinSQL+`
		WHERE pm.playlist_id = ?`, defaultDuration, defaultDuration, playlistID).Scan(&stats).Error
	return stats, err
}

func (r *PlaylistRepositoryImpl) GetThumbnailMarkers(playlistID uint, limit int) ([]MarkerThumbnailRef, error) {
	var refs []MarkerThumbnailRef
	err := r.DB.Raw(`
		SELECT m.id, m.updated_at
		FROM playlist_markers pm
		JOIN user_scene_markers m ON m.id = pm.marker_id`+playableMarkerJoinSQL+`
		WHERE pm.playlist_id = ? AND m.thumbnail_path IS NOT NULL AND m.thumbnail_path <> ''
		ORDER BY pm.position ASC, pm.id ASC
		LIMIT ?`, playlistID, limit).Scan(&refs).Error
	if err != nil {
		return nil, err
	}
	return refs, nil
}

func (r *PlaylistRepositoryImpl) FilterVisibleMarkerIDs(userID uint, markerIDs []uint) ([]uint, error) {
	ids := uniqueUints(markerIDs)
	if len(ids) == 0 {
		return []uint{}, nil
	}
	var found []uint
	err := r.DB.Raw(`
		SELECT m.id FROM user_scene_markers m`+playableMarkerJoinSQL+`
		WHERE m.id IN ? AND `+VisibleMarkersSQL("m"), ids, userID).Scan(&found).Error
	if err != nil {
		return nil, err
	}
	visible := make(map[uint]bool, len(found))
	for _, id := range found {
		visible[id] = true
	}
	out := make([]uint, 0, len(found))
	for _, id := range ids {
		if visible[id] {
			out = append(out, id)
		}
	}
	return out, nil
}

func (r *PlaylistRepositoryImpl) ResolveVisibleMarkerIDs(userID uint, label string, opts MarkerListOptions, limit int) ([]uint, error) {
	// A label page lists one label (the labels filter is not offered there)
	filter, filterArgs := markerFilterSQL(opts, "m", label == "")
	where := VisibleMarkersSQL("m")
	args := []any{userID}
	if label != "" {
		// Same match as the label page (GetMarkersByLabelForUser): the label, ignoring case
		where += " AND " + markerLabelInSQL("m")
		args = append(args, markerLabelKeys([]string{label}))
	}
	where += filter
	args = append(args, filterArgs...)

	// Same order as the listing: newest first on a label page, the chosen sort on /markers/all
	var orderClause string
	switch {
	case label != "":
		orderClause = "m.created_at DESC"
	case opts.SortBy == "random":
		orderClause = randomOrderSQL("m.id")
		args = append(args, seedArg(opts.Seed))
	default:
		var ok bool
		if orderClause, ok = allMarkersSortMap[opts.SortBy]; !ok {
			orderClause = allMarkersSortMap["label_asc"]
		}
	}
	args = append(args, limit)

	var ids []uint
	err := r.DB.Raw(`
		SELECT m.id FROM user_scene_markers m`+playableMarkerJoinSQL+`
		WHERE `+where+`
		ORDER BY `+orderClause+`, m.id ASC
		LIMIT ?`, args...).Scan(&ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}
