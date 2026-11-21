package data

import (
	"errors"
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

// UserSceneHeatmap is the "most replayed" histogram of a scene for one user: the scene is
// split into equal buckets and each element is the watched time (milliseconds) in that bucket.
type UserSceneHeatmap struct {
	UserID    uint          `gorm:"primaryKey;autoIncrement:false" json:"user_id"`
	SceneID   uint          `gorm:"primaryKey;autoIncrement:false" json:"scene_id"`
	Buckets   pq.Int32Array `gorm:"type:integer[];not null" json:"buckets"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

func (UserSceneHeatmap) TableName() string {
	return "user_scene_heatmaps"
}

type ReplayHeatmapRepository interface {
	// AddBuckets adds the increments element-wise to the user's histogram of the scene
	// (creating it when missing) in a single statement and returns the updated histogram.
	AddBuckets(userID, sceneID uint, increments []int32) ([]int32, error)
	// GetBuckets returns the user's histogram of the scene, or nil when there is none.
	GetBuckets(userID, sceneID uint) ([]int32, error)
}

type ReplayHeatmapRepositoryImpl struct {
	DB *gorm.DB
}

func NewReplayHeatmapRepository(db *gorm.DB) *ReplayHeatmapRepositoryImpl {
	return &ReplayHeatmapRepositoryImpl{DB: db}
}

// addBucketsSQL upserts the histogram. On conflict both arrays are walked side by side
// (unnest pads the shorter one with NULLs, so a histogram with another bucket count keeps
// its extra elements) and each sum saturates at the INTEGER maximum.
const addBucketsSQL = `
INSERT INTO user_scene_heatmaps AS h (user_id, scene_id, buckets, created_at, updated_at)
VALUES (?, ?, ?::integer[], ?, ?)
ON CONFLICT (user_id, scene_id) DO UPDATE SET
	buckets = ARRAY(
		SELECT LEAST(COALESCE(t.old, 0)::bigint + COALESCE(t.inc, 0), 2147483647)::integer
		FROM unnest(h.buckets, EXCLUDED.buckets) WITH ORDINALITY AS t(old, inc, i)
		ORDER BY t.i
	),
	updated_at = EXCLUDED.updated_at
RETURNING buckets`

func (r *ReplayHeatmapRepositoryImpl) AddBuckets(userID, sceneID uint, increments []int32) ([]int32, error) {
	now := time.Now().UTC()
	var buckets pq.Int32Array
	row := r.DB.Raw(addBucketsSQL, userID, sceneID, pq.Int32Array(increments), now, now).Row()
	if err := row.Scan(&buckets); err != nil {
		return nil, err
	}
	return []int32(buckets), nil
}

func (r *ReplayHeatmapRepositoryImpl) GetBuckets(userID, sceneID uint) ([]int32, error) {
	var heatmap UserSceneHeatmap
	err := r.DB.Where("user_id = ? AND scene_id = ?", userID, sceneID).First(&heatmap).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return []int32(heatmap.Buckets), nil
}

var _ ReplayHeatmapRepository = (*ReplayHeatmapRepositoryImpl)(nil)
