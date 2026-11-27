package data

import (
	"context"
)

// Full lists of a profile (docs/PROFILES.md, "Lists"): the rated, liked and jizz videos of a
// user, and the histogram of their ratings. Like the blocks, they skip trashed and deleted
// scenes, so the counts match what the lists show.

// Sorts of the profile lists (ProfileListQuery.Sort). The repository maps them to fixed
// ORDER BY clauses; core validates them per list.
const (
	ProfileSortRatingDesc = "rating_desc" // highest rated first, then most recently rated
	ProfileSortRatingAsc  = "rating_asc"
	ProfileSortRatedDesc  = "rated_desc" // most recently rated first
	ProfileSortRatedAsc   = "rated_asc"
	ProfileSortLikedDesc  = "liked_desc" // most recently liked first
	ProfileSortLikedAsc   = "liked_asc"
	ProfileSortJizzRecent = "recent"     // most recent O first
	ProfileSortJizzCount  = "count_desc" // most O first, then most recent
)

// ProfileListQuery is a page of a profile list. Rating (rated list only) keeps the scenes
// whose rating falls in that half-star bucket.
type ProfileListQuery struct {
	Sort   string
	Rating *float64
	Page   int
	Limit  int
}

func (q ProfileListQuery) offset() int {
	if q.Page < 1 {
		return 0
	}
	return (q.Page - 1) * q.Limit
}

// ProfileRatingBucket is the number of a user's ratings in one half-star bucket.
type ProfileRatingBucket struct {
	Rating float64 `gorm:"column:rating" json:"rating"`
	Count  int64   `gorm:"column:count" json:"count"`
}

// ProfileRatingSummary is the histogram of a user's ratings (non-empty buckets, ascending)
// with the average of the ratings (nil without rating).
type ProfileRatingSummary struct {
	Buckets []ProfileRatingBucket
	Average *float64
}

// ProfileJizzSummary is the O total of a user over the live scenes and the number of those
// scenes.
type ProfileJizzSummary struct {
	Total  int64 `gorm:"column:total"`
	Scenes int64 `gorm:"column:scenes"`
}

// ratingBucketSQL rounds a rating to its half-star bucket (the ratings are set in half steps,
// imported ones may have one decimal).
const ratingBucketSQL = "(ROUND(r.rating * 2) / 2)"

func (r *ProfileRepositoryImpl) RatingSummary(ctx context.Context, userID uint) (*ProfileRatingSummary, error) {
	from := `FROM user_scene_ratings r JOIN scenes s ON s.id = r.scene_id AND ` + liveSceneSQL("s") + ` WHERE r.user_id = ?`
	var buckets []ProfileRatingBucket
	if err := r.DB.WithContext(ctx).Raw(`SELECT `+ratingBucketSQL+`::float8 AS rating, COUNT(*) AS count `+from+
		` GROUP BY 1 ORDER BY 1`, userID).Scan(&buckets).Error; err != nil {
		return nil, err
	}
	var avg *float64
	if err := r.DB.WithContext(ctx).Raw(`SELECT AVG(r.rating)::float8 `+from, userID).Scan(&avg).Error; err != nil {
		return nil, err
	}
	if buckets == nil {
		buckets = []ProfileRatingBucket{}
	}
	return &ProfileRatingSummary{Buckets: buckets, Average: avg}, nil
}

func (r *ProfileRepositoryImpl) RatedScenes(ctx context.Context, userID uint, q ProfileListQuery) ([]ProfileSceneRef, int64, error) {
	from := `FROM user_scene_ratings r JOIN scenes s ON s.id = r.scene_id AND ` + liveSceneSQL("s") + ` WHERE r.user_id = ?`
	args := []any{userID}
	if q.Rating != nil {
		from += ` AND ` + ratingBucketSQL + ` = CAST(? AS numeric)`
		args = append(args, *q.Rating)
	}
	var order string
	switch q.Sort {
	case ProfileSortRatingAsc:
		order = "r.rating ASC, r.updated_at DESC"
	case ProfileSortRatedDesc:
		order = "r.updated_at DESC"
	case ProfileSortRatedAsc:
		order = "r.updated_at ASC"
	default:
		order = "r.rating DESC, r.updated_at DESC"
	}
	var total int64
	if err := r.DB.WithContext(ctx).Raw(`SELECT COUNT(*) `+from, args...).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []ProfileSceneRef
	if err := r.DB.WithContext(ctx).Raw(`SELECT r.scene_id, r.updated_at AS at, 0 AS count, r.rating::float8 AS rating `+from+
		` ORDER BY `+order+`, r.scene_id DESC LIMIT ? OFFSET ?`, append(args, q.Limit, q.offset())...).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *ProfileRepositoryImpl) LikedScenesPage(ctx context.Context, userID uint, q ProfileListQuery) ([]ProfileSceneRef, int64, error) {
	from := `FROM user_scene_likes l JOIN scenes s ON s.id = l.scene_id AND ` + liveSceneSQL("s") + ` WHERE l.user_id = ?`
	order := "l.created_at DESC, l.scene_id DESC"
	if q.Sort == ProfileSortLikedAsc {
		order = "l.created_at ASC, l.scene_id ASC"
	}
	var total int64
	if err := r.DB.WithContext(ctx).Raw(`SELECT COUNT(*) `+from, userID).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []ProfileSceneRef
	if err := r.DB.WithContext(ctx).Raw(`SELECT l.scene_id, l.created_at AS at, 0 AS count `+from+
		` ORDER BY `+order+` LIMIT ? OFFSET ?`, userID, q.Limit, q.offset()).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// jizzFromSQL: the user's scenes with an O count, the date of their last O (the last dated
// event, else the last change of the count; bound: user twice).
const jizzFromSQL = `FROM user_scene_jizzed j
	JOIN scenes s ON s.id = j.scene_id AND s.deleted_at IS NULL AND s.trashed_at IS NULL
	LEFT JOIN (SELECT e.scene_id, MAX(e.jizzed_at) AS last FROM user_scene_jizz_events e
		WHERE e.user_id = ? GROUP BY e.scene_id) le ON le.scene_id = j.scene_id
	WHERE j.user_id = ? AND j.count > 0`

func (r *ProfileRepositoryImpl) JizzScenes(ctx context.Context, userID uint, q ProfileListQuery) ([]ProfileSceneRef, int64, error) {
	order := "at DESC, j.count DESC"
	if q.Sort == ProfileSortJizzCount {
		order = "j.count DESC, at DESC"
	}
	var total int64
	if err := r.DB.WithContext(ctx).Raw(`SELECT COUNT(*) FROM user_scene_jizzed j
		JOIN scenes s ON s.id = j.scene_id AND `+liveSceneSQL("s")+`
		WHERE j.user_id = ? AND j.count > 0`, userID).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []ProfileSceneRef
	if err := r.DB.WithContext(ctx).Raw(`SELECT j.scene_id, COALESCE(le.last, j.updated_at AT TIME ZONE 'UTC') AS at, j.count AS count `+
		jizzFromSQL+` ORDER BY `+order+`, j.scene_id DESC LIMIT ? OFFSET ?`, userID, userID, q.Limit, q.offset()).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *ProfileRepositoryImpl) JizzSummary(ctx context.Context, userID uint) (*ProfileJizzSummary, error) {
	var out ProfileJizzSummary
	if err := r.DB.WithContext(ctx).Raw(`SELECT COALESCE(SUM(j.count), 0) AS total, COUNT(*) AS scenes FROM user_scene_jizzed j
		JOIN scenes s ON s.id = j.scene_id AND `+liveSceneSQL("s")+`
		WHERE j.user_id = ? AND j.count > 0`, userID).Scan(&out).Error; err != nil {
		return nil, err
	}
	return &out, nil
}
