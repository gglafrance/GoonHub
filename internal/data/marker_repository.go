package data

import (
	"fmt"
	"strconv"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MarkerRepository interface {
	Create(marker *UserSceneMarker) error
	GetByID(id uint) (*UserSceneMarker, error)
	// GetByUserAndScene returns the markers of a scene that the user sees (VisibleMarkersSQL):
	// their own and the shared ones.
	GetByUserAndScene(userID, sceneID uint) ([]UserSceneMarker, error)
	// CountByUserAndScene counts the user's own markers on a scene.
	CountByUserAndScene(userID, sceneID uint) (int64, error)
	Update(marker *UserSceneMarker) error
	// UpdateThumbnailPath / UpdateAnimatedThumbnailPath set only that column (and updated_at,
	// which versions the thumbnail URLs)
	UpdateThumbnailPath(id uint, path string) error
	UpdateAnimatedThumbnailPath(id uint, path string) error
	Delete(id uint) error
	// The listings below (labels, markers, thumbnails, scenes by label) cover the markers the
	// user sees: their own and the shared ones (VisibleMarkersSQL).
	GetLabelSuggestionsForUser(userID uint, limit int) ([]MarkerLabelSuggestion, error)
	GetLabelGroupsForUser(userID uint, offset, limit int, opts MarkerListOptions) ([]MarkerLabelGroup, int64, error)
	GetMarkersByLabelForUser(userID uint, label string, offset, limit int, opts MarkerListOptions) ([]MarkerWithScene, int64, error)

	// Individual marker tag methods
	GetMarkerTags(markerID uint) ([]MarkerTagInfo, error)
	GetMarkerTagsMultiple(markerIDs []uint) (map[uint][]MarkerTagInfo, error)
	SetMarkerTags(markerID uint, tagIDs []uint) error
	AddMarkerTags(markerID uint, tagIDs []uint) error
	// ApplyLabelTagsToMarker makes the marker's tags "from label" the default tags of its
	// current label (none without label); tags set by hand stay.
	ApplyLabelTagsToMarker(markerID uint) error

	// Thumbnail methods
	GetRandomThumbnailsForLabels(userID uint, labels []string, perLabel int, opts MarkerListOptions) (map[string][]MarkerThumbnailRef, error)
	// GetByUserAndScenes returns the markers the user sees on several scenes, ordered by timestamp.
	GetByUserAndScenes(userID uint, sceneIDs []uint) ([]UserSceneMarker, error)

	// Scene-level methods (not user-scoped)
	GetBySceneWithoutThumbnail(sceneID uint) ([]UserSceneMarker, error)
	GetBySceneWithoutAnimatedThumbnail(sceneID uint) ([]UserSceneMarker, error)
	GetAllByScene(sceneID uint) ([]UserSceneMarker, error)

	// All markers (unwrapped view)
	GetAllMarkersForUser(userID uint, offset, limit int, opts MarkerListOptions) ([]MarkerWithScene, int64, error)

	// Search filter methods
	GetSceneIDsByLabels(userID uint, labels []string) ([]uint, error)

	// Duplicate resolution: markers moved to the kept scene, and duplicate markers folded
	// MoveMarkers sets the scene and times of markers, in one transaction (their tags follow).
	MoveMarkers(moves []MarkerMove) error
	// ApplyMarkerDedup folds duplicate markers into the kept one of each merge, in one
	// transaction (see MarkerDedupMerge).
	ApplyMarkerDedup(merges []MarkerDedupMerge) error
	// ListSceneIDsWithNearbyMarkers returns, in ascending order, the scenes (not deleted)
	// where a user has two markers whose starts are at most toleranceSeconds apart.
	ListSceneIDsWithNearbyMarkers(toleranceSeconds int) ([]uint, error)
}

type MarkerRepositoryImpl struct {
	DB *gorm.DB
}

func NewMarkerRepository(db *gorm.DB) *MarkerRepositoryImpl {
	return &MarkerRepositoryImpl{DB: db}
}

// VisibleMarkersSQL is the condition keeping the markers (of the table aliased as alias, or
// unaliased when alias is empty) that a user sees: their own and those of the users who share
// theirs (users.share_markers). The user ID is its single bound argument. Only listings use it;
// writes stay limited to the owner.
func VisibleMarkersSQL(alias string) string {
	col := "user_id"
	if alias != "" {
		col = alias + ".user_id"
	}
	return "(" + col + " = ? OR " + col + " IN (SELECT id FROM users WHERE share_markers))"
}

func (r *MarkerRepositoryImpl) Create(marker *UserSceneMarker) error {
	return r.DB.Create(marker).Error
}

func (r *MarkerRepositoryImpl) GetByID(id uint) (*UserSceneMarker, error) {
	var marker UserSceneMarker
	if err := r.DB.First(&marker, id).Error; err != nil {
		return nil, err
	}
	return &marker, nil
}

func (r *MarkerRepositoryImpl) GetByUserAndScene(userID, sceneID uint) ([]UserSceneMarker, error) {
	var markers []UserSceneMarker
	err := r.DB.Where(VisibleMarkersSQL("")+" AND scene_id = ?", userID, sceneID).
		Order("timestamp ASC, id ASC").
		Find(&markers).Error
	if err != nil {
		return nil, err
	}
	return markers, nil
}

func (r *MarkerRepositoryImpl) CountByUserAndScene(userID, sceneID uint) (int64, error) {
	var count int64
	err := r.DB.Model(&UserSceneMarker{}).
		Where("user_id = ? AND scene_id = ?", userID, sceneID).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *MarkerRepositoryImpl) Update(marker *UserSceneMarker) error {
	return r.DB.Save(marker).Error
}

func (r *MarkerRepositoryImpl) UpdateThumbnailPath(id uint, path string) error {
	return r.updateThumbnailColumn(id, "thumbnail_path", path)
}

func (r *MarkerRepositoryImpl) UpdateAnimatedThumbnailPath(id uint, path string) error {
	return r.updateThumbnailColumn(id, "animated_thumbnail_path", path)
}

// updateThumbnailColumn sets a thumbnail path and bumps updated_at explicitly: the thumbnail
// URLs are versioned by updated_at, and the file is rewritten under the same name.
func (r *MarkerRepositoryImpl) updateThumbnailColumn(id uint, column, path string) error {
	return r.DB.Model(&UserSceneMarker{}).Where("id = ?", id).
		Updates(map[string]any{column: path, "updated_at": time.Now()}).Error
}

func (r *MarkerRepositoryImpl) Delete(id uint) error {
	return r.DB.Delete(&UserSceneMarker{}, id).Error
}

// GetLabelSuggestionsForUser returns the labels of the referential (unused ones included),
// most used first by the markers the user sees, then by name.
func (r *MarkerRepositoryImpl) GetLabelSuggestionsForUser(userID uint, limit int) ([]MarkerLabelSuggestion, error) {
	var suggestions []MarkerLabelSuggestion
	err := r.DB.Raw(`SELECT ml.id, ml.name AS label, ml.color, COUNT(m.id) AS count
		FROM marker_labels ml
		LEFT JOIN user_scene_markers m ON m.label_id = ml.id AND `+VisibleMarkersSQL("m")+`
		GROUP BY ml.id
		ORDER BY count DESC, lower(ml.name) ASC, ml.id ASC
		LIMIT ?`, userID, limit).Scan(&suggestions).Error
	if err != nil {
		return nil, err
	}
	return suggestions, nil
}

// sortOrderMap maps sort parameter values to safe SQL ORDER BY clauses
var sortOrderMap = map[string]string{
	"label_asc":  "label ASC",
	"label_desc": "label DESC",
	"count_asc":  "count ASC",
	"count_desc": "count DESC",
	"recent":     "MAX(created_at) DESC",
}

// MarkerListOptions controls marker listings (sort and filters).
type MarkerListOptions struct {
	SortBy string
	// Seed makes the "random" sort stable across pages.
	Seed int64
	// RangesOnly restricts listings to markers that have an end (range markers).
	RangesOnly bool
	// Labels keeps markers having one of these labels.
	Labels []string
	// Tags keeps markers whose scene has all these tags (by name).
	Tags []string
	// Actors keeps markers whose scene has all these actors (by name).
	Actors []string
	// Studio keeps markers whose scene belongs to this studio (by name).
	Studio string
	// MarkerTags keeps markers having all these marker tags (by name).
	MarkerTags []string
}

// HasSceneFilters reports whether a filter on the marker's scene is set.
func (o MarkerListOptions) HasSceneFilters() bool {
	return len(o.Tags) > 0 || len(o.Actors) > 0 || o.Studio != ""
}

// randomOrderSQL is a seeded pseudo-random ORDER BY expression; the seed is bound as a text
// parameter (see seedArg).
func randomOrderSQL(keyExpr string) string {
	return "md5(" + keyExpr + "::text || ?)"
}

// seedArg formats the random sort seed as the text bound by randomOrderSQL.
func seedArg(seed int64) string {
	return strconv.FormatInt(seed, 10)
}

// markerFilterSQL returns the " AND ..." conditions (and their bound arguments) applying the
// filters of opts to the marker table aliased as alias. The SQL is built from constants only.
func markerFilterSQL(opts MarkerListOptions, alias string, withLabels bool) (string, []any) {
	var sql string
	var args []any
	if opts.RangesOnly {
		sql += " AND " + alias + ".end_timestamp IS NOT NULL"
	}
	if withLabels && len(opts.Labels) > 0 {
		sql += " AND " + markerLabelInSQL(alias)
		args = append(args, markerLabelKeys(opts.Labels))
	}
	if len(opts.Tags) > 0 {
		sql += " AND " + alias + `.scene_id IN (
			SELECT st.scene_id FROM scene_tags st JOIN tags t ON t.id = st.tag_id
			WHERE t.name IN ? GROUP BY st.scene_id HAVING COUNT(DISTINCT t.name) = ?)`
		args = append(args, opts.Tags, len(uniqueStrings(opts.Tags)))
	}
	if len(opts.Actors) > 0 {
		sql += " AND " + alias + `.scene_id IN (
			SELECT sa.scene_id FROM scene_actors sa JOIN actors a ON a.id = sa.actor_id
			WHERE a.name IN ? GROUP BY sa.scene_id HAVING COUNT(DISTINCT a.name) = ?)`
		args = append(args, opts.Actors, len(uniqueStrings(opts.Actors)))
	}
	if len(opts.MarkerTags) > 0 {
		sql += " AND " + alias + `.id IN (
			SELECT mt.marker_id FROM marker_tags mt JOIN tags mtt ON mtt.id = mt.tag_id
			WHERE mtt.name IN ? GROUP BY mt.marker_id HAVING COUNT(DISTINCT mtt.name) = ?)`
		args = append(args, opts.MarkerTags, len(uniqueStrings(opts.MarkerTags)))
	}
	if opts.Studio != "" {
		sql += " AND " + alias + `.scene_id IN (
			SELECT sc.id FROM scenes sc LEFT JOIN studios so ON so.id = sc.studio_id
			WHERE so.name = ? OR (sc.studio_id IS NULL AND sc.studio = ?))`
		args = append(args, opts.Studio, opts.Studio)
	}
	return sql, args
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func (r *MarkerRepositoryImpl) GetLabelGroupsForUser(userID uint, offset, limit int, opts MarkerListOptions) ([]MarkerLabelGroup, int64, error) {
	outerFilter, outerArgs := markerFilterSQL(opts, "um", true)
	innerFilter, innerArgs := markerFilterSQL(opts, "m2", true)

	// First get total count of unique labels
	var totalCount int64
	err := r.DB.Raw(`
		SELECT COUNT(DISTINCT um.label) FROM user_scene_markers um
		WHERE `+VisibleMarkersSQL("um")+` AND um.label != ''`+outerFilter,
		append([]any{userID}, outerArgs...)...).Scan(&totalCount).Error
	if err != nil {
		return nil, 0, err
	}

	// Get validated ORDER BY clause from whitelist map
	args := append([]any{userID}, innerArgs...)
	args = append(args, userID)
	args = append(args, outerArgs...)
	orderClause, ok := sortOrderMap[opts.SortBy]
	if opts.SortBy == "random" {
		orderClause = randomOrderSQL("um.label")
		args = append(args, seedArg(opts.Seed))
	} else if !ok {
		orderClause = sortOrderMap["count_desc"] // default
	}
	args = append(args, limit, offset)

	// Get label groups with count and most recent marker ID for thumbnail
	var groups []MarkerLabelGroup
	err = r.DB.Raw(`
		SELECT
			um.label,
			COUNT(*) as count,
			(SELECT m2.id FROM user_scene_markers m2
			 WHERE `+VisibleMarkersSQL("m2")+` AND m2.label = um.label`+innerFilter+`
			 ORDER BY m2.created_at DESC LIMIT 1) as thumbnail_marker_id
		FROM user_scene_markers um
		WHERE `+VisibleMarkersSQL("um")+` AND um.label != ''`+outerFilter+`
		GROUP BY um.label
		ORDER BY `+orderClause+`
		LIMIT ? OFFSET ?
	`, args...).Scan(&groups).Error
	if err != nil {
		return nil, 0, err
	}

	return groups, totalCount, nil
}

func (r *MarkerRepositoryImpl) GetMarkersByLabelForUser(userID uint, label string, offset, limit int, opts MarkerListOptions) ([]MarkerWithScene, int64, error) {
	filter, filterArgs := markerFilterSQL(opts, "m", false)
	baseArgs := append([]any{userID, markerLabelKeys([]string{label})}, filterArgs...)

	// Get total count
	var totalCount int64
	err := r.DB.Raw(`
		SELECT COUNT(*) FROM user_scene_markers m
		WHERE `+VisibleMarkersSQL("m")+` AND `+markerLabelInSQL("m")+filter, baseArgs...).Scan(&totalCount).Error
	if err != nil {
		return nil, 0, err
	}

	// Get markers with scene title
	var markers []MarkerWithScene
	err = r.DB.Raw(`
		SELECT m.*, s.title as scene_title
		FROM user_scene_markers m
		JOIN scenes s ON m.scene_id = s.id
		WHERE `+VisibleMarkersSQL("m")+` AND `+markerLabelInSQL("m")+filter+`
		ORDER BY m.created_at DESC
		LIMIT ? OFFSET ?
	`, append(baseArgs, limit, offset)...).Scan(&markers).Error
	if err != nil {
		return nil, 0, err
	}

	return markers, totalCount, nil
}

// allMarkersSortMap maps sort parameter values to safe SQL ORDER BY clauses for individual markers
var allMarkersSortMap = map[string]string{
	"label_asc":  "m.label ASC, m.created_at DESC",
	"label_desc": "m.label DESC, m.created_at DESC",
	"recent":     "m.created_at DESC",
	"oldest":     "m.created_at ASC",
}

// GetAllMarkersForUser returns all individual markers for a user with scene info
func (r *MarkerRepositoryImpl) GetAllMarkersForUser(userID uint, offset, limit int, opts MarkerListOptions) ([]MarkerWithScene, int64, error) {
	filter, filterArgs := markerFilterSQL(opts, "m", true)
	baseArgs := append([]any{userID}, filterArgs...)

	// Get total count
	var totalCount int64
	err := r.DB.Raw(`
		SELECT COUNT(*) FROM user_scene_markers m
		WHERE `+VisibleMarkersSQL("m")+filter, baseArgs...).Scan(&totalCount).Error
	if err != nil {
		return nil, 0, err
	}

	// Get validated ORDER BY clause
	args := append([]any{}, baseArgs...)
	orderClause, ok := allMarkersSortMap[opts.SortBy]
	if opts.SortBy == "random" {
		orderClause = randomOrderSQL("m.id")
		args = append(args, seedArg(opts.Seed))
	} else if !ok {
		orderClause = allMarkersSortMap["label_asc"]
	}
	args = append(args, limit, offset)

	var markers []MarkerWithScene
	err = r.DB.Raw(`
		SELECT m.*, s.title as scene_title
		FROM user_scene_markers m
		JOIN scenes s ON m.scene_id = s.id
		WHERE `+VisibleMarkersSQL("m")+filter+`
		ORDER BY `+orderClause+`
		LIMIT ? OFFSET ?
	`, args...).Scan(&markers).Error
	if err != nil {
		return nil, 0, err
	}

	return markers, totalCount, nil
}

// GetByUserAndScenes returns the markers the user sees (own and shared) on several scenes,
// ordered by timestamp.
func (r *MarkerRepositoryImpl) GetByUserAndScenes(userID uint, sceneIDs []uint) ([]UserSceneMarker, error) {
	if len(sceneIDs) == 0 {
		return []UserSceneMarker{}, nil
	}
	var markers []UserSceneMarker
	err := r.DB.Where(VisibleMarkersSQL("")+" AND scene_id IN ?", userID, sceneIDs).
		Order("scene_id ASC, timestamp ASC").
		Find(&markers).Error
	if err != nil {
		return nil, err
	}
	return markers, nil
}

// GetMarkerTags returns tags for a specific marker
func (r *MarkerRepositoryImpl) GetMarkerTags(markerID uint) ([]MarkerTagInfo, error) {
	var tags []MarkerTagInfo
	err := r.DB.
		Table("tags").
		Select("tags.id, tags.name, tags.color, marker_tags.is_from_label").
		Joins("JOIN marker_tags ON marker_tags.tag_id = tags.id").
		Where("marker_tags.marker_id = ?", markerID).
		Order("tags.name ASC").
		Scan(&tags).Error
	if err != nil {
		return nil, err
	}
	return tags, nil
}

// GetMarkerTagsMultiple returns tags for multiple markers
func (r *MarkerRepositoryImpl) GetMarkerTagsMultiple(markerIDs []uint) (map[uint][]MarkerTagInfo, error) {
	if len(markerIDs) == 0 {
		return make(map[uint][]MarkerTagInfo), nil
	}

	type markerTagResult struct {
		MarkerID uint
		MarkerTagInfo
	}

	var results []markerTagResult
	err := r.DB.
		Table("marker_tags").
		Select("marker_tags.marker_id, tags.id, tags.name, tags.color, marker_tags.is_from_label").
		Joins("JOIN tags ON tags.id = marker_tags.tag_id").
		Where("marker_tags.marker_id IN ?", markerIDs).
		Order("tags.name ASC").
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	tagsByMarker := make(map[uint][]MarkerTagInfo)
	for _, markerID := range markerIDs {
		tagsByMarker[markerID] = []MarkerTagInfo{}
	}
	for _, r := range results {
		tagsByMarker[r.MarkerID] = append(tagsByMarker[r.MarkerID], r.MarkerTagInfo)
	}

	return tagsByMarker, nil
}

// SetMarkerTags replaces individual (non-label-derived) tags on a marker
func (r *MarkerRepositoryImpl) SetMarkerTags(markerID uint, tagIDs []uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		// Delete existing individual tags (preserve label-derived)
		if err := tx.Where("marker_id = ? AND is_from_label = ?", markerID, false).Delete(&MarkerTag{}).Error; err != nil {
			return err
		}

		if len(tagIDs) == 0 {
			return nil
		}

		// Insert new individual tags
		markerTags := make([]MarkerTag, len(tagIDs))
		for i, tagID := range tagIDs {
			markerTags[i] = MarkerTag{
				MarkerID:    markerID,
				TagID:       tagID,
				IsFromLabel: false,
			}
		}

		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&markerTags).Error
	})
}

// AddMarkerTags adds individual tags to a marker
func (r *MarkerRepositoryImpl) AddMarkerTags(markerID uint, tagIDs []uint) error {
	if len(tagIDs) == 0 {
		return nil
	}

	markerTags := make([]MarkerTag, len(tagIDs))
	for i, tagID := range tagIDs {
		markerTags[i] = MarkerTag{
			MarkerID:    markerID,
			TagID:       tagID,
			IsFromLabel: false,
		}
	}

	return r.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&markerTags).Error
}

// ApplyLabelTagsToMarker makes the marker's tags "from label" the default tags of its label:
// the other tags from label are dropped, the missing ones added (a tag set by hand stays).
func (r *MarkerRepositoryImpl) ApplyLabelTagsToMarker(markerID uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`DELETE FROM marker_tags mt
				USING user_scene_markers m
				WHERE mt.marker_id = m.id AND m.id = ? AND mt.is_from_label
					AND (m.label_id IS NULL
						OR mt.tag_id NOT IN (SELECT tag_id FROM marker_label_tags WHERE label_id = m.label_id))`,
			markerID).Error; err != nil {
			return fmt.Errorf("drop old label tags of marker %d: %w", markerID, err)
		}
		if err := tx.Exec(`INSERT INTO marker_tags (marker_id, tag_id, is_from_label)
				SELECT m.id, lt.tag_id, TRUE FROM user_scene_markers m
				JOIN marker_label_tags lt ON lt.label_id = m.label_id
				WHERE m.id = ?
				ON CONFLICT (marker_id, tag_id) DO NOTHING`, markerID).Error; err != nil {
			return fmt.Errorf("add label tags to marker %d: %w", markerID, err)
		}
		return nil
	})
}

// GetRandomThumbnailsForLabels returns random marker IDs with thumbnails for each label
func (r *MarkerRepositoryImpl) GetRandomThumbnailsForLabels(userID uint, labels []string, perLabel int, opts MarkerListOptions) (map[string][]MarkerThumbnailRef, error) {
	if len(labels) == 0 {
		return make(map[string][]MarkerThumbnailRef), nil
	}

	type result struct {
		Label     string
		ID        uint
		UpdatedAt time.Time
	}

	filter, filterArgs := markerFilterSQL(opts, "um", false)
	args := append([]any{userID, labels}, filterArgs...)
	args = append(args, perLabel)

	var results []result
	err := r.DB.Raw(`
		SELECT label, id, updated_at FROM (
			SELECT um.label, um.id, um.updated_at, ROW_NUMBER() OVER (PARTITION BY um.label ORDER BY RANDOM()) as rn
			FROM user_scene_markers um
			WHERE `+VisibleMarkersSQL("um")+` AND um.label IN ? AND um.thumbnail_path != '' AND um.thumbnail_path IS NOT NULL`+filter+`
		) sub WHERE rn <= ?
	`, args...).Scan(&results).Error
	if err != nil {
		return nil, err
	}

	thumbnails := make(map[string][]MarkerThumbnailRef)
	for _, r := range results {
		thumbnails[r.Label] = append(thumbnails[r.Label], MarkerThumbnailRef{ID: r.ID, UpdatedAt: r.UpdatedAt})
	}

	return thumbnails, nil
}

// GetBySceneWithoutThumbnail returns all markers for a scene (regardless of user) where thumbnail_path is empty
func (r *MarkerRepositoryImpl) GetBySceneWithoutThumbnail(sceneID uint) ([]UserSceneMarker, error) {
	var markers []UserSceneMarker
	err := r.DB.Where("scene_id = ? AND (thumbnail_path = '' OR thumbnail_path IS NULL)", sceneID).
		Find(&markers).Error
	if err != nil {
		return nil, err
	}
	return markers, nil
}

// GetBySceneWithoutAnimatedThumbnail returns all markers for a scene where animated_thumbnail_path is empty
func (r *MarkerRepositoryImpl) GetBySceneWithoutAnimatedThumbnail(sceneID uint) ([]UserSceneMarker, error) {
	var markers []UserSceneMarker
	err := r.DB.Where("scene_id = ? AND (animated_thumbnail_path = '' OR animated_thumbnail_path IS NULL)", sceneID).
		Find(&markers).Error
	if err != nil {
		return nil, err
	}
	return markers, nil
}

// GetAllByScene returns all markers for a scene regardless of thumbnail status
func (r *MarkerRepositoryImpl) GetAllByScene(sceneID uint) ([]UserSceneMarker, error) {
	var markers []UserSceneMarker
	err := r.DB.Where("scene_id = ?", sceneID).
		Find(&markers).Error
	if err != nil {
		return nil, err
	}
	return markers, nil
}

// GetSceneIDsByLabels returns distinct scene IDs that have markers with any of the given labels for a user
func (r *MarkerRepositoryImpl) GetSceneIDsByLabels(userID uint, labels []string) ([]uint, error) {
	if len(labels) == 0 {
		return []uint{}, nil
	}

	var sceneIDs []uint
	err := r.DB.Model(&UserSceneMarker{}).
		Select("DISTINCT scene_id").
		Where(VisibleMarkersSQL("")+" AND "+markerLabelInSQL("user_scene_markers"), userID, markerLabelKeys(labels)).
		Pluck("scene_id", &sceneIDs).Error
	if err != nil {
		return nil, err
	}
	return sceneIDs, nil
}

// MoveMarkers sets the scene and times of markers, in one transaction. marker_tags reference
// the marker, so its tags follow; updated_at moves, which versions the thumbnail URLs.
func (r *MarkerRepositoryImpl) MoveMarkers(moves []MarkerMove) error {
	if len(moves) == 0 {
		return nil
	}
	now := time.Now()
	return r.DB.Transaction(func(tx *gorm.DB) error {
		for _, m := range moves {
			res := tx.Model(&UserSceneMarker{}).Where("id = ?", m.ID).Updates(map[string]any{
				"scene_id":      m.SceneID,
				"timestamp":     m.Timestamp,
				"end_timestamp": m.EndTimestamp,
				"updated_at":    now,
			})
			if res.Error != nil {
				return fmt.Errorf("failed to move marker %d: %w", m.ID, res.Error)
			}
		}
		return nil
	})
}

// ApplyMarkerDedup folds duplicate markers, in one transaction: each kept marker gets the
// merge's label (when set) and tags (existing ones kept as they are), then the other markers
// are deleted (their marker_tags go with them, ON DELETE CASCADE).
func (r *MarkerRepositoryImpl) ApplyMarkerDedup(merges []MarkerDedupMerge) error {
	if len(merges) == 0 {
		return nil
	}
	return r.DB.Transaction(func(tx *gorm.DB) error {
		for _, m := range merges {
			if m.Label != nil {
				// the label comes from another marker of the set: it exists (empty: none)
				if err := tx.Exec(`UPDATE user_scene_markers SET label_id =
						(SELECT id FROM marker_labels WHERE lower(name) = lower(btrim(?)))
					WHERE id = ?`, *m.Label, m.KeepID).Error; err != nil {
					return fmt.Errorf("failed to set label of marker %d: %w", m.KeepID, err)
				}
			}
			if len(m.AddTags) > 0 {
				tags := make([]MarkerTag, len(m.AddTags))
				for i, t := range m.AddTags {
					tags[i] = MarkerTag{MarkerID: m.KeepID, TagID: t.TagID, IsFromLabel: t.IsFromLabel}
				}
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&tags).Error; err != nil {
					return fmt.Errorf("failed to add tags to marker %d: %w", m.KeepID, err)
				}
			}
			if len(m.RemoveIDs) > 0 {
				if err := tx.Where("id IN ?", m.RemoveIDs).Delete(&UserSceneMarker{}).Error; err != nil {
					return fmt.Errorf("failed to delete duplicates of marker %d: %w", m.KeepID, err)
				}
			}
		}
		return nil
	})
}

// ListSceneIDsWithNearbyMarkers returns the scenes (not deleted) where a user has two markers
// whose starts are at most toleranceSeconds apart: the candidates for marker deduplication.
func (r *MarkerRepositoryImpl) ListSceneIDsWithNearbyMarkers(toleranceSeconds int) ([]uint, error) {
	var ids []uint
	err := r.DB.Raw(`SELECT DISTINCT a.scene_id
		FROM user_scene_markers a
		JOIN user_scene_markers b ON b.scene_id = a.scene_id AND b.user_id = a.user_id
			AND b.id > a.id AND ABS(b.timestamp - a.timestamp) <= ?
		JOIN scenes s ON s.id = a.scene_id AND s.deleted_at IS NULL
		ORDER BY a.scene_id`, toleranceSeconds).Scan(&ids).Error
	return ids, err
}

// Ensure MarkerRepositoryImpl implements MarkerRepository
var _ MarkerRepository = (*MarkerRepositoryImpl)(nil)
