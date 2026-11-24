package data

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// ScenePhaseFailure counts the consecutive failures of a processing phase on a scene, with the
// size and modification time of the file at the last failure (nil when the file could not be
// read). A success of the phase deletes the row.
type ScenePhaseFailure struct {
	ID           uint       `gorm:"primarykey" json:"id"`
	SceneID      uint       `gorm:"not null" json:"scene_id"`
	Phase        string     `gorm:"size:30;not null" json:"phase"`
	Failures     int        `gorm:"not null;default:1" json:"failures"`
	LastError    string     `gorm:"type:text;not null;default:''" json:"last_error"`
	LastFailedAt time.Time  `gorm:"not null" json:"last_failed_at"`
	FileSize     *int64     `json:"file_size"`
	FileModTime  *time.Time `json:"file_mod_time"`
	CreatedAt    time.Time  `json:"created_at"`
}

func (ScenePhaseFailure) TableName() string {
	return "scene_phase_failures"
}

// ScenePhaseFailureRow is a failure counter with what the list shows of its scene.
type ScenePhaseFailureRow struct {
	ScenePhaseFailure
	SceneTitle string `json:"scene_title"`
	StoredPath string `json:"stored_path"`
}

// ScenePhaseFailureRecord is one failure to add to the counter of its (scene, phase).
type ScenePhaseFailureRecord struct {
	SceneID     uint
	Phase       string
	Error       string
	FailedAt    time.Time
	FileSize    *int64
	FileModTime *time.Time
}

// ScenePhaseFailureFilter selects failure counters. Phase and ErrorQuery are optional.
type ScenePhaseFailureFilter struct {
	MinFailures int
	Phase       string
	ErrorQuery  string // case-insensitive substring of the last error
}

// ScenePhaseFailureRepository stores the consecutive failures per (scene, phase).
type ScenePhaseFailureRepository interface {
	// RecordFailure adds a failure to the counter of the (scene, phase) and returns the row. The
	// counter restarts at 1 when the file changed (size or modification time) since the last one.
	RecordFailure(rec ScenePhaseFailureRecord) (*ScenePhaseFailure, error)
	// Clear forgets the failures of a (scene, phase).
	Clear(sceneID uint, phase string) error
	// List returns the counters matching the filter, most recent failure first, with their scene
	// (scenes deleted, soft-deleted or in the trash are left out).
	List(filter ScenePhaseFailureFilter) ([]ScenePhaseFailureRow, error)
	// GetByScenePhase returns the counter of a (scene, phase) with its scene, or gorm.ErrRecordNotFound.
	GetByScenePhase(sceneID uint, phase string) (*ScenePhaseFailureRow, error)
	// GetByIDs returns the counters with these IDs, with their scene.
	GetByIDs(ids []uint) ([]ScenePhaseFailureRow, error)
	// DeleteByIDs forgets the counters with these IDs and returns how many were deleted.
	DeleteByIDs(ids []uint) (int64, error)
}

type ScenePhaseFailureRepositoryImpl struct {
	DB *gorm.DB
}

func NewScenePhaseFailureRepository(db *gorm.DB) *ScenePhaseFailureRepositoryImpl {
	return &ScenePhaseFailureRepositoryImpl{DB: db}
}

func (r *ScenePhaseFailureRepositoryImpl) RecordFailure(rec ScenePhaseFailureRecord) (*ScenePhaseFailure, error) {
	var row ScenePhaseFailure
	err := r.DB.Raw(`
		INSERT INTO scene_phase_failures (scene_id, phase, failures, last_error, last_failed_at, file_size, file_mod_time)
		VALUES (?, ?, 1, ?, ?, ?, ?)
		ON CONFLICT (scene_id, phase) DO UPDATE SET
			failures = CASE
				WHEN scene_phase_failures.file_size IS NOT DISTINCT FROM EXCLUDED.file_size
					AND scene_phase_failures.file_mod_time IS NOT DISTINCT FROM EXCLUDED.file_mod_time
				THEN scene_phase_failures.failures + 1
				ELSE 1
			END,
			last_error = EXCLUDED.last_error,
			last_failed_at = EXCLUDED.last_failed_at,
			file_size = EXCLUDED.file_size,
			file_mod_time = EXCLUDED.file_mod_time
		RETURNING *
	`, rec.SceneID, rec.Phase, rec.Error, rec.FailedAt, rec.FileSize, rec.FileModTime).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *ScenePhaseFailureRepositoryImpl) Clear(sceneID uint, phase string) error {
	return r.DB.Where("scene_id = ? AND phase = ?", sceneID, phase).Delete(&ScenePhaseFailure{}).Error
}

// failureRowsQuery selects the counters with the title and path of their live scene.
func (r *ScenePhaseFailureRepositoryImpl) failureRowsQuery() *gorm.DB {
	return r.DB.Table("scene_phase_failures f").
		Select("f.*, s.title AS scene_title, s.stored_path AS stored_path").
		Joins("JOIN scenes s ON s.id = f.scene_id AND s.deleted_at IS NULL AND s.trashed_at IS NULL")
}

func (r *ScenePhaseFailureRepositoryImpl) List(filter ScenePhaseFailureFilter) ([]ScenePhaseFailureRow, error) {
	q := r.failureRowsQuery()
	if filter.MinFailures > 0 {
		q = q.Where("f.failures >= ?", filter.MinFailures)
	}
	if filter.Phase != "" {
		q = q.Where("f.phase = ?", filter.Phase)
	}
	if text := strings.TrimSpace(filter.ErrorQuery); text != "" {
		q = q.Where("f.last_error ILIKE ?", "%"+escapeLike(text)+"%")
	}
	var rows []ScenePhaseFailureRow
	if err := q.Order("f.last_failed_at DESC, f.id DESC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *ScenePhaseFailureRepositoryImpl) GetByScenePhase(sceneID uint, phase string) (*ScenePhaseFailureRow, error) {
	var rows []ScenePhaseFailureRow
	if err := r.failureRowsQuery().Where("f.scene_id = ? AND f.phase = ?", sceneID, phase).Limit(1).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &rows[0], nil
}

func (r *ScenePhaseFailureRepositoryImpl) GetByIDs(ids []uint) ([]ScenePhaseFailureRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []ScenePhaseFailureRow
	if err := r.failureRowsQuery().Where("f.id IN ?", ids).Order("f.id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *ScenePhaseFailureRepositoryImpl) DeleteByIDs(ids []uint) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.DB.Where("id IN ?", ids).Delete(&ScenePhaseFailure{})
	return result.RowsAffected, result.Error
}
