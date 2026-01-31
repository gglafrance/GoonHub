package data

import (
	"gorm.io/gorm"
)

type MarkerRepository interface {
	Create(marker *UserVideoMarker) error
	GetByID(id uint) (*UserVideoMarker, error)
	GetByUserAndVideo(userID, videoID uint) ([]UserVideoMarker, error)
	CountByUserAndVideo(userID, videoID uint) (int64, error)
	Update(marker *UserVideoMarker) error
	Delete(id uint) error
	GetLabelSuggestionsForUser(userID uint, limit int) ([]MarkerLabelSuggestion, error)
	GetLabelGroupsForUser(userID uint, offset, limit int, sortBy string) ([]MarkerLabelGroup, int64, error)
	GetMarkersByLabelForUser(userID uint, label string, offset, limit int) ([]MarkerWithVideo, int64, error)
}

type MarkerRepositoryImpl struct {
	DB *gorm.DB
}

func NewMarkerRepository(db *gorm.DB) *MarkerRepositoryImpl {
	return &MarkerRepositoryImpl{DB: db}
}

func (r *MarkerRepositoryImpl) Create(marker *UserVideoMarker) error {
	return r.DB.Create(marker).Error
}

func (r *MarkerRepositoryImpl) GetByID(id uint) (*UserVideoMarker, error) {
	var marker UserVideoMarker
	if err := r.DB.First(&marker, id).Error; err != nil {
		return nil, err
	}
	return &marker, nil
}

func (r *MarkerRepositoryImpl) GetByUserAndVideo(userID, videoID uint) ([]UserVideoMarker, error) {
	var markers []UserVideoMarker
	err := r.DB.Where("user_id = ? AND video_id = ?", userID, videoID).
		Order("timestamp ASC").
		Find(&markers).Error
	if err != nil {
		return nil, err
	}
	return markers, nil
}

func (r *MarkerRepositoryImpl) CountByUserAndVideo(userID, videoID uint) (int64, error) {
	var count int64
	err := r.DB.Model(&UserVideoMarker{}).
		Where("user_id = ? AND video_id = ?", userID, videoID).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *MarkerRepositoryImpl) Update(marker *UserVideoMarker) error {
	return r.DB.Save(marker).Error
}

func (r *MarkerRepositoryImpl) Delete(id uint) error {
	return r.DB.Delete(&UserVideoMarker{}, id).Error
}

func (r *MarkerRepositoryImpl) GetLabelSuggestionsForUser(userID uint, limit int) ([]MarkerLabelSuggestion, error) {
	var suggestions []MarkerLabelSuggestion
	err := r.DB.Model(&UserVideoMarker{}).
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
	err := r.DB.Model(&UserVideoMarker{}).
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
			(SELECT id FROM user_video_markers m2
			 WHERE m2.user_id = ? AND m2.label = user_video_markers.label
			 ORDER BY created_at DESC LIMIT 1) as thumbnail_marker_id
		FROM user_video_markers
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

func (r *MarkerRepositoryImpl) GetMarkersByLabelForUser(userID uint, label string, offset, limit int) ([]MarkerWithVideo, int64, error) {
	// Get total count
	var totalCount int64
	err := r.DB.Model(&UserVideoMarker{}).
		Where("user_id = ? AND label = ?", userID, label).
		Count(&totalCount).Error
	if err != nil {
		return nil, 0, err
	}

	// Get markers with video title
	var markers []MarkerWithVideo
	err = r.DB.Raw(`
		SELECT m.*, v.title as video_title
		FROM user_video_markers m
		JOIN videos v ON m.video_id = v.id
		WHERE m.user_id = ? AND m.label = ?
		ORDER BY m.created_at DESC
		LIMIT ? OFFSET ?
	`, userID, label, limit, offset).Scan(&markers).Error
	if err != nil {
		return nil, 0, err
	}

	return markers, totalCount, nil
}

// Ensure MarkerRepositoryImpl implements MarkerRepository
var _ MarkerRepository = (*MarkerRepositoryImpl)(nil)
