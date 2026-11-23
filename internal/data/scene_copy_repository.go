package data

import (
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SceneCopy is a file with the same content as a scene's file (same size and oshash) at
// another path. The library scan records it instead of creating a scene for it. The row is
// bookkeeping only: GoonHub never deletes, moves or writes the file of a copy.
type SceneCopy struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	SceneID       uint      `gorm:"not null" json:"scene_id"`
	StoredPath    string    `gorm:"not null" json:"stored_path"`
	StoragePathID *uint     `json:"storage_path_id"`
	Size          int64     `gorm:"not null" json:"size"`
	Oshash        string    `gorm:"column:oshash;not null" json:"oshash"`
	CreatedAt     time.Time `json:"created_at"`
	LastSeenAt    time.Time `json:"last_seen_at"`
}

func (SceneCopy) TableName() string {
	return "scene_copies"
}

// sceneCopyTouchBatchSize bounds the number of paths per last_seen_at update.
const sceneCopyTouchBatchSize = 5000

type SceneCopyRepository interface {
	// GetAllStoredPathSet returns the path of every recorded copy.
	GetAllStoredPathSet() (map[string]struct{}, error)
	// CreateBatch records copies; a path that is already recorded is left as it is.
	CreateBatch(copies []*SceneCopy) error
	// TouchLastSeen sets last_seen_at of the copies at the given paths.
	TouchLastSeen(paths []string, seenAt time.Time) error
	// ListBySceneID returns the copies of a scene ordered by path.
	ListBySceneID(sceneID uint) ([]SceneCopy, error)

	// Triage (Duplicates page > Copies, see scene_copy_triage_repository.go)
	ListScenesWithCopies(q SceneCopyQuery) ([]SceneWithCopiesRow, error)
	CopyTotals(f SceneCopyFilter) (SceneCopyTotals, error)
	ListCopiesOfScenes(sceneIDs []uint, f SceneCopyFilter) ([]SceneCopy, error)
	ListCopyIDs(f SceneCopyFilter, limit int) ([]uint, int64, error)
	LastCompletedScanStart() (*time.Time, error)
	GetByID(id uint) (*SceneCopy, error)
	GetByIDs(ids []uint) ([]SceneCopy, error)
	GetSceneFile(sceneID uint) (*SceneFileRow, error)
	SwapWithMain(copyID uint, storagePathID *uint) error
	PromoteToScene(copyID uint, scene *Scene) error
	Delete(id uint) error
	DeleteByIDs(ids []uint) (int64, error)
}

type SceneCopyRepositoryImpl struct {
	DB *gorm.DB
}

func NewSceneCopyRepository(db *gorm.DB) *SceneCopyRepositoryImpl {
	return &SceneCopyRepositoryImpl{DB: db}
}

func (r *SceneCopyRepositoryImpl) GetAllStoredPathSet() (map[string]struct{}, error) {
	var paths []string
	if err := r.DB.Model(&SceneCopy{}).Pluck("stored_path", &paths).Error; err != nil {
		return nil, err
	}
	result := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		result[p] = struct{}{}
	}
	return result, nil
}

func (r *SceneCopyRepositoryImpl) CreateBatch(copies []*SceneCopy) error {
	if len(copies) == 0 {
		return nil
	}
	now := time.Now()
	for _, c := range copies {
		if c.LastSeenAt.IsZero() {
			c.LastSeenAt = now
		}
	}
	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "stored_path"}},
		DoNothing: true,
	}).CreateInBatches(copies, 100).Error
}

func (r *SceneCopyRepositoryImpl) TouchLastSeen(paths []string, seenAt time.Time) error {
	for start := 0; start < len(paths); start += sceneCopyTouchBatchSize {
		end := min(start+sceneCopyTouchBatchSize, len(paths))
		if err := r.DB.Exec(`UPDATE scene_copies SET last_seen_at = ? WHERE stored_path = ANY(?)`,
			seenAt, pq.Array(paths[start:end])).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *SceneCopyRepositoryImpl) ListBySceneID(sceneID uint) ([]SceneCopy, error) {
	copies := []SceneCopy{}
	if err := r.DB.Where("scene_id = ?", sceneID).Order("stored_path").Find(&copies).Error; err != nil {
		return nil, err
	}
	return copies, nil
}
