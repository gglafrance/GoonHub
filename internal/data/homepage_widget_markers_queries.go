package data

import (
	"gorm.io/gorm"
)

// Queries of the marker homepage widgets (moments, marker_compilation, recent_markers,
// browse_tags). Markers of trashed or deleted scenes are excluded.

// WidgetMarkerQuery selects markers of a user for a homepage widget.
type WidgetMarkerQuery struct {
	Labels     []string
	RangesOnly bool
	// Order: "random" or "recent" (newest markers first)
	Order string
	Limit int
}

// WidgetMarker is a marker with its scene title, as shown by the marker widgets.
type WidgetMarker struct {
	ID                    uint   `json:"id"`
	SceneID               uint   `json:"scene_id"`
	SceneTitle            string `json:"scene_title"`
	Label                 string `json:"label"`
	Color                 string `json:"color"`
	Timestamp             int    `json:"timestamp"`
	EndTimestamp          *int   `json:"end_timestamp,omitempty"`
	ThumbnailPath         string `json:"-"`
	AnimatedThumbnailPath string `json:"-"`
	HasThumbnail          bool   `json:"has_thumbnail" gorm:"-"`
	HasAnimated           bool   `json:"has_animated" gorm:"-"`
	// Tags of the marker (label tag included), shown as "+X" on the card
	Tags []MarkerTagInfo `json:"tags,omitempty" gorm:"-"`
}

// ListWidgetMarkers returns markers of a user for a widget.
func ListWidgetMarkers(db *gorm.DB, userID uint, q WidgetMarkerQuery) ([]WidgetMarker, error) {
	order := "m.created_at DESC, m.id DESC"
	if q.Order == "random" {
		order = "RANDOM()"
	}
	query := db.Table("user_scene_markers AS m").
		Select("m.id, m.scene_id, s.title AS scene_title, m.label, m.color, m.timestamp, m.end_timestamp, "+
			"m.thumbnail_path, m.animated_thumbnail_path").
		Joins("JOIN scenes s ON s.id = m.scene_id").
		Where("m.user_id = ? AND s.trashed_at IS NULL AND s.deleted_at IS NULL", userID)
	if len(q.Labels) > 0 {
		query = query.Where("m.label IN ?", q.Labels)
	}
	if q.RangesOnly {
		query = query.Where("m.end_timestamp IS NOT NULL")
	}
	var markers []WidgetMarker
	if err := query.Order(order).Limit(q.Limit).Scan(&markers).Error; err != nil {
		return nil, err
	}
	for i := range markers {
		markers[i].HasThumbnail = markers[i].ThumbnailPath != ""
		markers[i].HasAnimated = markers[i].AnimatedThumbnailPath != ""
	}
	return markers, nil
}

// WidgetTile is a tag or marker label tile with a cover.
type WidgetTile struct {
	Key           string `json:"key"`
	Label         string `json:"label"`
	Color         string `json:"color,omitempty"`
	Count         int64  `json:"count"`
	CoverMarkerID *uint  `json:"cover_marker_id,omitempty"`
	CoverSceneID  *uint  `json:"cover_scene_id,omitempty"`
}

// ListMarkerLabelTiles returns marker label tiles of a user (most used first), each with a random
// marker thumbnail as cover. With labels, only those labels are returned (in that order).
func ListMarkerLabelTiles(db *gorm.DB, userID uint, labels []string, limit int) ([]WidgetTile, error) {
	type row struct {
		Label         string
		Count         int64
		CoverMarkerID *uint
	}
	query := db.Table("user_scene_markers AS m").
		Select(`m.label, COUNT(*) AS count,
			(SELECT m2.id FROM user_scene_markers m2 JOIN scenes s2 ON s2.id = m2.scene_id
			 WHERE m2.user_id = m.user_id AND m2.label = m.label AND m2.thumbnail_path <> ''
			   AND s2.trashed_at IS NULL AND s2.deleted_at IS NULL
			 ORDER BY RANDOM() LIMIT 1) AS cover_marker_id`).
		Joins("JOIN scenes s ON s.id = m.scene_id").
		Where("m.user_id = ? AND m.label <> '' AND s.trashed_at IS NULL AND s.deleted_at IS NULL", userID).
		Group("m.user_id, m.label")
	if len(labels) > 0 {
		query = query.Where("m.label IN ?", labels)
	}
	var rows []row
	if err := query.Order("count DESC, m.label ASC").Limit(limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	tiles := make([]WidgetTile, len(rows))
	for i, r := range rows {
		tiles[i] = WidgetTile{Key: r.Label, Label: r.Label, Count: r.Count, CoverMarkerID: r.CoverMarkerID}
	}
	return orderTiles(tiles, labels), nil
}

// ListSceneTagTiles returns scene tag tiles (most used first), each with a random scene
// thumbnail as cover. With names, only those tags are returned (in that order).
func ListSceneTagTiles(db *gorm.DB, names []string, limit int) ([]WidgetTile, error) {
	type row struct {
		Name         string
		Color        string
		Count        int64
		CoverSceneID *uint
	}
	query := db.Table("tags AS t").
		Select(`t.name, t.color, COUNT(s.id) AS count,
			(SELECT s2.id FROM scene_tags st2 JOIN scenes s2 ON s2.id = st2.scene_id
			 WHERE st2.tag_id = t.id AND s2.thumbnail_path <> ''
			   AND s2.trashed_at IS NULL AND s2.deleted_at IS NULL
			 ORDER BY RANDOM() LIMIT 1) AS cover_scene_id`).
		Joins("JOIN scene_tags st ON st.tag_id = t.id").
		Joins("JOIN scenes s ON s.id = st.scene_id AND s.trashed_at IS NULL AND s.deleted_at IS NULL").
		Group("t.id, t.name, t.color")
	if len(names) > 0 {
		query = query.Where("t.name IN ?", names)
	}
	var rows []row
	if err := query.Order("count DESC, t.name ASC").Limit(limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	tiles := make([]WidgetTile, len(rows))
	for i, r := range rows {
		tiles[i] = WidgetTile{Key: r.Name, Label: r.Name, Color: r.Color, Count: r.Count, CoverSceneID: r.CoverSceneID}
	}
	return orderTiles(tiles, names), nil
}

// orderTiles keeps the explicit order of the configured items, when given.
func orderTiles(tiles []WidgetTile, order []string) []WidgetTile {
	if len(order) == 0 {
		return tiles
	}
	byKey := make(map[string]WidgetTile, len(tiles))
	for _, t := range tiles {
		byKey[t.Key] = t
	}
	ordered := make([]WidgetTile, 0, len(tiles))
	for _, key := range order {
		if t, ok := byKey[key]; ok {
			ordered = append(ordered, t)
			delete(byKey, key)
		}
	}
	return ordered
}
