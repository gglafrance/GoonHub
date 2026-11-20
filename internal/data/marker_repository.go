package data

import (
	"strconv"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MarkerRepository interface {
	Create(marker *UserSceneMarker) error
	GetByID(id uint) (*UserSceneMarker, error)
	GetByUserAndScene(userID, sceneID uint) ([]UserSceneMarker, error)
	CountByUserAndScene(userID, sceneID uint) (int64, error)
	Update(marker *UserSceneMarker) error
	Delete(id uint) error
	GetLabelSuggestionsForUser(userID uint, limit int) ([]MarkerLabelSuggestion, error)
	GetLabelGroupsForUser(userID uint, offset, limit int, opts MarkerListOptions) ([]MarkerLabelGroup, int64, error)
	GetMarkersByLabelForUser(userID uint, label string, offset, limit int, opts MarkerListOptions) ([]MarkerWithScene, int64, error)

	// Label tag methods
	GetLabelTags(userID uint, label string) ([]Tag, error)
	SetLabelTags(userID uint, label string, tagIDs []uint) error
	GetAllLabelTagsForUser(userID uint) (map[string][]Tag, error)

	// Individual marker tag methods
	GetMarkerTags(markerID uint) ([]MarkerTagInfo, error)
	GetMarkerTagsMultiple(markerIDs []uint) (map[uint][]MarkerTagInfo, error)
	SetMarkerTags(markerID uint, tagIDs []uint) error
	AddMarkerTags(markerID uint, tagIDs []uint) error
	SyncMarkerTagsFromLabel(userID uint, label string) error
	ApplyLabelTagsToMarker(userID uint, markerID uint, label string) error
	GetMarkerIDsByLabel(userID uint, label string) ([]uint, error)

	// Thumbnail methods
	GetRandomThumbnailsForLabels(userID uint, labels []string, perLabel int, opts MarkerListOptions) (map[string][]uint, error)
	// GetByUserAndScenes returns the user's markers of several scenes, ordered by timestamp.
	GetByUserAndScenes(userID uint, sceneIDs []uint) ([]UserSceneMarker, error)

	// Scene-level methods (not user-scoped)
	GetBySceneWithoutThumbnail(sceneID uint) ([]UserSceneMarker, error)
	GetBySceneWithoutAnimatedThumbnail(sceneID uint) ([]UserSceneMarker, error)
	GetAllByScene(sceneID uint) ([]UserSceneMarker, error)

	// All markers (unwrapped view)
	GetAllMarkersForUser(userID uint, offset, limit int, opts MarkerListOptions) ([]MarkerWithScene, int64, error)

	// Search filter methods
	GetSceneIDsByLabels(userID uint, labels []string) ([]uint, error)

	// Reassignment methods (for duplicate resolution)
	ReassignMarkersToScene(fromSceneID, toSceneID uint) error
}

type MarkerRepositoryImpl struct {
	DB *gorm.DB
}

func NewMarkerRepository(db *gorm.DB) *MarkerRepositoryImpl {
	return &MarkerRepositoryImpl{DB: db}
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
	err := r.DB.Where("user_id = ? AND scene_id = ?", userID, sceneID).
		Order("timestamp ASC").
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

func (r *MarkerRepositoryImpl) Delete(id uint) error {
	return r.DB.Delete(&UserSceneMarker{}, id).Error
}

func (r *MarkerRepositoryImpl) GetLabelSuggestionsForUser(userID uint, limit int) ([]MarkerLabelSuggestion, error) {
	var suggestions []MarkerLabelSuggestion
	err := r.DB.Model(&UserSceneMarker{}).
		Select("label, COUNT(*) as count").
		Where("user_id = ? AND label != ''", userID).
		Group("label").
		Order("count DESC").
		Limit(limit).
		Scan(&suggestions).Error
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
		sql += " AND " + alias + ".label IN ?"
		args = append(args, opts.Labels)
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
		WHERE um.user_id = ? AND um.label != ''`+outerFilter,
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
			 WHERE m2.user_id = ? AND m2.label = um.label`+innerFilter+`
			 ORDER BY m2.created_at DESC LIMIT 1) as thumbnail_marker_id
		FROM user_scene_markers um
		WHERE um.user_id = ? AND um.label != ''`+outerFilter+`
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
	baseArgs := append([]any{userID, label}, filterArgs...)

	// Get total count
	var totalCount int64
	err := r.DB.Raw(`
		SELECT COUNT(*) FROM user_scene_markers m
		WHERE m.user_id = ? AND m.label = ?`+filter, baseArgs...).Scan(&totalCount).Error
	if err != nil {
		return nil, 0, err
	}

	// Get markers with scene title
	var markers []MarkerWithScene
	err = r.DB.Raw(`
		SELECT m.*, s.title as scene_title
		FROM user_scene_markers m
		JOIN scenes s ON m.scene_id = s.id
		WHERE m.user_id = ? AND m.label = ?`+filter+`
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
		WHERE m.user_id = ?`+filter, baseArgs...).Scan(&totalCount).Error
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
		WHERE m.user_id = ?`+filter+`
		ORDER BY `+orderClause+`
		LIMIT ? OFFSET ?
	`, args...).Scan(&markers).Error
	if err != nil {
		return nil, 0, err
	}

	return markers, totalCount, nil
}

// GetByUserAndScenes returns the user's markers of several scenes, ordered by timestamp.
func (r *MarkerRepositoryImpl) GetByUserAndScenes(userID uint, sceneIDs []uint) ([]UserSceneMarker, error) {
	if len(sceneIDs) == 0 {
		return []UserSceneMarker{}, nil
	}
	var markers []UserSceneMarker
	err := r.DB.Where("user_id = ? AND scene_id IN ?", userID, sceneIDs).
		Order("scene_id ASC, timestamp ASC").
		Find(&markers).Error
	if err != nil {
		return nil, err
	}
	return markers, nil
}

// GetLabelTags returns the default tags for a label
func (r *MarkerRepositoryImpl) GetLabelTags(userID uint, label string) ([]Tag, error) {
	var tags []Tag
	err := r.DB.
		Table("tags").
		Joins("JOIN marker_label_tags ON marker_label_tags.tag_id = tags.id").
		Where("marker_label_tags.user_id = ? AND marker_label_tags.label = ?", userID, label).
		Order("tags.name ASC").
		Find(&tags).Error
	if err != nil {
		return nil, err
	}
	return tags, nil
}

// SetLabelTags sets the default tags for a label and syncs to all existing markers
func (r *MarkerRepositoryImpl) SetLabelTags(userID uint, label string, tagIDs []uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		// Delete existing label tags
		if err := tx.Where("user_id = ? AND label = ?", userID, label).Delete(&MarkerLabelTag{}).Error; err != nil {
			return err
		}

		// Insert new label tags
		if len(tagIDs) > 0 {
			labelTags := make([]MarkerLabelTag, len(tagIDs))
			for i, tagID := range tagIDs {
				labelTags[i] = MarkerLabelTag{
					UserID: userID,
					Label:  label,
					TagID:  tagID,
				}
			}
			if err := tx.Create(&labelTags).Error; err != nil {
				return err
			}
		}

		// Sync to all existing markers with this label
		// First, get all marker IDs with this label
		var markerIDs []uint
		if err := tx.Model(&UserSceneMarker{}).
			Where("user_id = ? AND label = ?", userID, label).
			Pluck("id", &markerIDs).Error; err != nil {
			return err
		}

		if len(markerIDs) == 0 {
			return nil
		}

		// Delete existing label-derived tags from these markers
		if err := tx.Where("marker_id IN ? AND is_from_label = ?", markerIDs, true).Delete(&MarkerTag{}).Error; err != nil {
			return err
		}

		// Add new label-derived tags to all markers
		if len(tagIDs) > 0 {
			markerTags := make([]MarkerTag, 0, len(markerIDs)*len(tagIDs))
			for _, markerID := range markerIDs {
				for _, tagID := range tagIDs {
					markerTags = append(markerTags, MarkerTag{
						MarkerID:    markerID,
						TagID:       tagID,
						IsFromLabel: true,
					})
				}
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&markerTags).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// GetAllLabelTagsForUser returns all label->tags mappings for a user
func (r *MarkerRepositoryImpl) GetAllLabelTagsForUser(userID uint) (map[string][]Tag, error) {
	type labelTagResult struct {
		Label string
		Tag
	}

	var results []labelTagResult
	err := r.DB.
		Table("marker_label_tags").
		Select("marker_label_tags.label, tags.*").
		Joins("JOIN tags ON tags.id = marker_label_tags.tag_id").
		Where("marker_label_tags.user_id = ?", userID).
		Order("marker_label_tags.label ASC, tags.name ASC").
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	tagsByLabel := make(map[string][]Tag)
	for _, r := range results {
		tagsByLabel[r.Label] = append(tagsByLabel[r.Label], r.Tag)
	}

	return tagsByLabel, nil
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

// SyncMarkerTagsFromLabel syncs label-derived tags to all markers with a given label
func (r *MarkerRepositoryImpl) SyncMarkerTagsFromLabel(userID uint, label string) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		// Get label tag IDs
		var tagIDs []uint
		if err := tx.Model(&MarkerLabelTag{}).
			Where("user_id = ? AND label = ?", userID, label).
			Pluck("tag_id", &tagIDs).Error; err != nil {
			return err
		}

		// Get marker IDs with this label
		var markerIDs []uint
		if err := tx.Model(&UserSceneMarker{}).
			Where("user_id = ? AND label = ?", userID, label).
			Pluck("id", &markerIDs).Error; err != nil {
			return err
		}

		if len(markerIDs) == 0 {
			return nil
		}

		// Delete existing label-derived tags
		if err := tx.Where("marker_id IN ? AND is_from_label = ?", markerIDs, true).Delete(&MarkerTag{}).Error; err != nil {
			return err
		}

		// Add new label-derived tags
		if len(tagIDs) > 0 {
			markerTags := make([]MarkerTag, 0, len(markerIDs)*len(tagIDs))
			for _, markerID := range markerIDs {
				for _, tagID := range tagIDs {
					markerTags = append(markerTags, MarkerTag{
						MarkerID:    markerID,
						TagID:       tagID,
						IsFromLabel: true,
					})
				}
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&markerTags).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// ApplyLabelTagsToMarker applies label-derived tags to a single marker
func (r *MarkerRepositoryImpl) ApplyLabelTagsToMarker(userID uint, markerID uint, label string) error {
	if label == "" {
		return nil
	}

	// Get label tag IDs
	var tagIDs []uint
	if err := r.DB.Model(&MarkerLabelTag{}).
		Where("user_id = ? AND label = ?", userID, label).
		Pluck("tag_id", &tagIDs).Error; err != nil {
		return err
	}

	if len(tagIDs) == 0 {
		return nil
	}

	// Add label-derived tags to the marker
	markerTags := make([]MarkerTag, len(tagIDs))
	for i, tagID := range tagIDs {
		markerTags[i] = MarkerTag{
			MarkerID:    markerID,
			TagID:       tagID,
			IsFromLabel: true,
		}
	}

	return r.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&markerTags).Error
}

// GetMarkerIDsByLabel returns all marker IDs with a given label for a user
func (r *MarkerRepositoryImpl) GetMarkerIDsByLabel(userID uint, label string) ([]uint, error) {
	var markerIDs []uint
	err := r.DB.Model(&UserSceneMarker{}).
		Where("user_id = ? AND label = ?", userID, label).
		Pluck("id", &markerIDs).Error
	if err != nil {
		return nil, err
	}
	return markerIDs, nil
}

// GetRandomThumbnailsForLabels returns random marker IDs with thumbnails for each label
func (r *MarkerRepositoryImpl) GetRandomThumbnailsForLabels(userID uint, labels []string, perLabel int, opts MarkerListOptions) (map[string][]uint, error) {
	if len(labels) == 0 {
		return make(map[string][]uint), nil
	}

	type result struct {
		Label string
		ID    uint
	}

	filter, filterArgs := markerFilterSQL(opts, "um", false)
	args := append([]any{userID, labels}, filterArgs...)
	args = append(args, perLabel)

	var results []result
	err := r.DB.Raw(`
		SELECT label, id FROM (
			SELECT um.label, um.id, ROW_NUMBER() OVER (PARTITION BY um.label ORDER BY RANDOM()) as rn
			FROM user_scene_markers um
			WHERE um.user_id = ? AND um.label IN ? AND um.thumbnail_path != '' AND um.thumbnail_path IS NOT NULL`+filter+`
		) sub WHERE rn <= ?
	`, args...).Scan(&results).Error
	if err != nil {
		return nil, err
	}

	thumbnails := make(map[string][]uint)
	for _, r := range results {
		thumbnails[r.Label] = append(thumbnails[r.Label], r.ID)
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
		Where("user_id = ? AND label IN ?", userID, labels).
		Pluck("scene_id", &sceneIDs).Error
	if err != nil {
		return nil, err
	}
	return sceneIDs, nil
}

// ReassignMarkersToScene moves all markers from one scene to another
func (r *MarkerRepositoryImpl) ReassignMarkersToScene(fromSceneID, toSceneID uint) error {
	return r.DB.Model(&UserSceneMarker{}).
		Where("scene_id = ?", fromSceneID).
		Update("scene_id", toSceneID).Error
}

// Ensure MarkerRepositoryImpl implements MarkerRepository
var _ MarkerRepository = (*MarkerRepositoryImpl)(nil)
