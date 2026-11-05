package data

import (
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
	GetLabelGroupsForUser(userID uint, offset, limit int, sortBy string) ([]MarkerLabelGroup, int64, error)
	GetMarkersByLabelForUser(userID uint, label string, offset, limit int) ([]MarkerWithScene, int64, error)

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
	GetRandomThumbnailsForLabels(userID uint, labels []string, perLabel int) (map[string][]uint, error)

	// Scene-level methods (not user-scoped)
	GetBySceneWithoutThumbnail(sceneID uint) ([]UserSceneMarker, error)
	GetBySceneWithoutAnimatedThumbnail(sceneID uint) ([]UserSceneMarker, error)
	GetAllByScene(sceneID uint) ([]UserSceneMarker, error)

	// All markers (unwrapped view)
	GetAllMarkersForUser(userID uint, offset, limit int, sortBy string) ([]MarkerWithScene, int64, error)

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

func (r *MarkerRepositoryImpl) GetLabelGroupsForUser(userID uint, offset, limit int, sortBy string) ([]MarkerLabelGroup, int64, error) {
	// First get total count of unique labels
	var totalCount int64
	err := r.DB.Model(&UserSceneMarker{}).
		Select("COUNT(DISTINCT label)").
		Where("user_id = ? AND label != ''", userID).
		Scan(&totalCount).Error
	if err != nil {
		return nil, 0, err
	}

	// Get validated ORDER BY clause from whitelist map
	orderClause, ok := sortOrderMap[sortBy]
	if !ok {
		orderClause = sortOrderMap["count_desc"] // default
	}

	// Get label groups with count and most recent marker ID for thumbnail
	var groups []MarkerLabelGroup
	err = r.DB.Raw(`
		SELECT
			label,
			COUNT(*) as count,
			(SELECT id FROM user_scene_markers m2
			 WHERE m2.user_id = ? AND m2.label = user_scene_markers.label
			 ORDER BY created_at DESC LIMIT 1) as thumbnail_marker_id
		FROM user_scene_markers
		WHERE user_id = ? AND label != ''
		GROUP BY label
		ORDER BY `+orderClause+`
		LIMIT ? OFFSET ?
	`, userID, userID, limit, offset).Scan(&groups).Error
	if err != nil {
		return nil, 0, err
	}

	return groups, totalCount, nil
}

func (r *MarkerRepositoryImpl) GetMarkersByLabelForUser(userID uint, label string, offset, limit int) ([]MarkerWithScene, int64, error) {
	// Get total count
	var totalCount int64
	err := r.DB.Model(&UserSceneMarker{}).
		Where("user_id = ? AND label = ?", userID, label).
		Count(&totalCount).Error
	if err != nil {
		return nil, 0, err
	}

	// Get markers with scene title
	var markers []MarkerWithScene
	err = r.DB.Raw(`
		SELECT m.*, s.title as scene_title
		FROM user_scene_markers m
		JOIN scenes s ON m.scene_id = s.id
		WHERE m.user_id = ? AND m.label = ?
		ORDER BY m.created_at DESC
		LIMIT ? OFFSET ?
	`, userID, label, limit, offset).Scan(&markers).Error
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
func (r *MarkerRepositoryImpl) GetAllMarkersForUser(userID uint, offset, limit int, sortBy string) ([]MarkerWithScene, int64, error) {
	// Get total count
	var totalCount int64
	err := r.DB.Model(&UserSceneMarker{}).
		Where("user_id = ?", userID).
		Count(&totalCount).Error
	if err != nil {
		return nil, 0, err
	}

	// Get validated ORDER BY clause
	orderClause, ok := allMarkersSortMap[sortBy]
	if !ok {
		orderClause = allMarkersSortMap["label_asc"]
	}

	var markers []MarkerWithScene
	err = r.DB.Raw(`
		SELECT m.*, s.title as scene_title
		FROM user_scene_markers m
		JOIN scenes s ON m.scene_id = s.id
		WHERE m.user_id = ?
		ORDER BY `+orderClause+`
		LIMIT ? OFFSET ?
	`, userID, limit, offset).Scan(&markers).Error
	if err != nil {
		return nil, 0, err
	}

	return markers, totalCount, nil
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
func (r *MarkerRepositoryImpl) GetRandomThumbnailsForLabels(userID uint, labels []string, perLabel int) (map[string][]uint, error) {
	if len(labels) == 0 {
		return make(map[string][]uint), nil
	}

	type result struct {
		Label string
		ID    uint
	}

	var results []result
	err := r.DB.Raw(`
		SELECT label, id FROM (
			SELECT label, id, ROW_NUMBER() OVER (PARTITION BY label ORDER BY RANDOM()) as rn
			FROM user_scene_markers
			WHERE user_id = ? AND label IN ? AND thumbnail_path != '' AND thumbnail_path IS NOT NULL
		) sub WHERE rn <= ?
	`, userID, labels, perLabel).Scan(&results).Error
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
