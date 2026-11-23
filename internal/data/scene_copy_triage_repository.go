package data

import (
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SceneCopyFilter selects copies for the triage list (Duplicates page > Copies). Empty fields
// do not filter.
type SceneCopyFilter struct {
	// StoragePathID keeps the copies found under this storage path
	StoragePathID *uint
	// Search is a case-insensitive substring of the copy path, the main file path or the title
	Search string
	// MinSize and MaxSize bound the copy size in bytes (0: no bound)
	MinSize int64
	MaxSize int64
}

// Sorts of the triage list.
const (
	SceneCopySortNewest = "newest" // most recently recorded copy first
	SceneCopySortSize   = "size"   // largest file first
	SceneCopySortCopies = "copies" // most copies first
)

// SceneCopyQuery is a page of the triage list.
type SceneCopyQuery struct {
	SceneCopyFilter
	Sort  string
	Page  int
	Limit int
}

// SceneWithCopiesRow is a scene with copies matching the filter, with its main file.
type SceneWithCopiesRow struct {
	SceneID       uint      `json:"scene_id"`
	Title         string    `json:"title"`
	StoredPath    string    `json:"stored_path"`
	Size          int64     `json:"size"`
	StoragePathID *uint     `json:"storage_path_id"`
	ThumbnailPath string    `json:"thumbnail_path"`
	IsTrashed     bool      `json:"is_trashed"`
	IsMissing     bool      `json:"is_missing"`
	CopyCount     int       `json:"copy_count"`
	CopiesSize    int64     `json:"copies_size"`
	LatestCopyAt  time.Time `json:"latest_copy_at"`
}

// SceneCopyTotals sums the copies matching a filter.
type SceneCopyTotals struct {
	Scenes int64 `json:"scenes"`
	Copies int64 `json:"copies"`
	// Size is the disk space the copies take
	Size int64 `json:"size"`
}

// SceneFileRow is the main file of a scene, whatever its state (trashed or marked missing).
type SceneFileRow struct {
	ID            uint
	StoredPath    string
	StoragePathID *uint
	Size          int64
	IsTrashed     bool
	IsDeleted     bool
}

const sceneFileSelect = `SELECT id, COALESCE(stored_path, '') AS stored_path, storage_path_id, COALESCE(size, 0) AS size,
	trashed_at IS NOT NULL AS is_trashed, deleted_at IS NOT NULL AS is_deleted FROM scenes WHERE id = ?`

func (r *SceneCopyRepositoryImpl) copiesJoin(f SceneCopyFilter) *gorm.DB {
	q := r.DB.Table("scene_copies AS c").Joins("JOIN scenes s ON s.id = c.scene_id")
	if f.StoragePathID != nil {
		q = q.Where("c.storage_path_id = ?", *f.StoragePathID)
	}
	if term := strings.TrimSpace(f.Search); term != "" {
		like := "%" + escapeLike(term) + "%"
		q = q.Where("(c.stored_path ILIKE ? OR s.stored_path ILIKE ? OR s.title ILIKE ?)", like, like, like)
	}
	if f.MinSize > 0 {
		q = q.Where("c.size >= ?", f.MinSize)
	}
	if f.MaxSize > 0 {
		q = q.Where("c.size <= ?", f.MaxSize)
	}
	return q
}

func sceneCopyOrder(sort string) string {
	switch sort {
	case SceneCopySortSize:
		return "s.size DESC, s.id DESC"
	case SceneCopySortCopies:
		return "copy_count DESC, s.id DESC"
	default:
		return "latest_copy_at DESC, s.id DESC"
	}
}

func (r *SceneCopyRepositoryImpl) ListScenesWithCopies(q SceneCopyQuery) ([]SceneWithCopiesRow, error) {
	rows := []SceneWithCopiesRow{}
	page, limit := q.Page, q.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	err := r.copiesJoin(q.SceneCopyFilter).
		Select(`s.id AS scene_id, COALESCE(s.title, '') AS title, COALESCE(s.stored_path, '') AS stored_path,
			COALESCE(s.size, 0) AS size, s.storage_path_id, COALESCE(s.thumbnail_path, '') AS thumbnail_path,
			s.trashed_at IS NOT NULL AS is_trashed, s.deleted_at IS NOT NULL AS is_missing,
			COUNT(c.id) AS copy_count, COALESCE(SUM(c.size), 0) AS copies_size, MAX(c.created_at) AS latest_copy_at`).
		Group("s.id").
		Order(sceneCopyOrder(q.Sort)).
		Limit(limit).Offset((page - 1) * limit).
		Scan(&rows).Error
	return rows, err
}

func (r *SceneCopyRepositoryImpl) CopyTotals(f SceneCopyFilter) (SceneCopyTotals, error) {
	var t SceneCopyTotals
	err := r.copiesJoin(f).
		Select("COUNT(DISTINCT c.scene_id) AS scenes, COUNT(c.id) AS copies, COALESCE(SUM(c.size), 0) AS size").
		Scan(&t).Error
	return t, err
}

func (r *SceneCopyRepositoryImpl) ListCopiesOfScenes(sceneIDs []uint, f SceneCopyFilter) ([]SceneCopy, error) {
	copies := []SceneCopy{}
	if len(sceneIDs) == 0 {
		return copies, nil
	}
	err := r.copiesJoin(f).Select("c.*").Where("c.scene_id IN ?", sceneIDs).
		Order("c.stored_path").Scan(&copies).Error
	return copies, err
}

func (r *SceneCopyRepositoryImpl) ListCopyIDs(f SceneCopyFilter, limit int) ([]uint, int64, error) {
	var total int64
	if err := r.copiesJoin(f).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	ids := []uint{}
	if err := r.copiesJoin(f).Order("c.id").Limit(limit).Pluck("c.id", &ids).Error; err != nil {
		return nil, 0, err
	}
	return ids, total, nil
}

func (r *SceneCopyRepositoryImpl) LastCompletedScanStart() (*time.Time, error) {
	var started *time.Time
	if err := r.DB.Raw(`SELECT MAX(started_at) FROM scan_history WHERE status = 'completed'`).Scan(&started).Error; err != nil {
		return nil, err
	}
	return started, nil
}

func (r *SceneCopyRepositoryImpl) GetByID(id uint) (*SceneCopy, error) {
	var c SceneCopy
	if err := r.DB.First(&c, id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *SceneCopyRepositoryImpl) GetByIDs(ids []uint) ([]SceneCopy, error) {
	copies := []SceneCopy{}
	if len(ids) == 0 {
		return copies, nil
	}
	err := r.DB.Where("id IN ?", ids).Order("id").Find(&copies).Error
	return copies, err
}

func scanSceneFile(db *gorm.DB, query string, sceneID uint) (*SceneFileRow, error) {
	var row SceneFileRow
	res := db.Raw(query, sceneID).Scan(&row)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &row, nil
}

func (r *SceneCopyRepositoryImpl) GetSceneFile(sceneID uint) (*SceneFileRow, error) {
	return scanSceneFile(r.DB, sceneFileSelect, sceneID)
}

// SwapWithMain makes the copy the scene's main file: the scene takes the copy's path (and the
// given storage path; its size is kept since the content is the same), a scene marked missing
// is restored, and the old main file becomes the copy row (same oshash). The scene ID does not
// change, so everything attached to the scene is kept. No file is touched.
func (r *SceneCopyRepositoryImpl) SwapWithMain(copyID uint, storagePathID *uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		var c SceneCopy
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&c, copyID).Error; err != nil {
			return err
		}
		s, err := scanSceneFile(tx, sceneFileSelect+" FOR UPDATE", c.SceneID)
		if err != nil {
			return err
		}
		now := time.Now()
		if err := tx.Exec(`UPDATE scenes SET stored_path = ?, storage_path_id = ?, deleted_at = NULL, updated_at = ? WHERE id = ?`,
			c.StoredPath, storagePathID, now, s.ID).Error; err != nil {
			return err
		}
		if s.StoredPath == "" {
			return tx.Delete(&SceneCopy{}, c.ID).Error
		}
		return tx.Exec(`UPDATE scene_copies SET stored_path = ?, storage_path_id = ?, last_seen_at = ? WHERE id = ?`,
			s.StoredPath, s.StoragePathID, now, c.ID).Error
	})
}

// PromoteToScene creates the scene for a copy's file and removes the copy row, together.
func (r *SceneCopyRepositoryImpl) PromoteToScene(copyID uint, scene *Scene) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Delete(&SceneCopy{}, copyID)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return tx.Create(scene).Error
	})
}

// Delete removes a copy row (never the file).
func (r *SceneCopyRepositoryImpl) Delete(id uint) error {
	res := r.DB.Delete(&SceneCopy{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteByIDs removes copy rows (never the files) and returns how many were removed.
func (r *SceneCopyRepositoryImpl) DeleteByIDs(ids []uint) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	res := r.DB.Where("id IN ?", ids).Delete(&SceneCopy{})
	return res.RowsAffected, res.Error
}
