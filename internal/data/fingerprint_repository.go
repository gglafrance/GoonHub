package data

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// FingerprintRepository handles scene fingerprint persistence.
type FingerprintRepository interface {
	BulkInsert(fingerprints []SceneFingerprint) error
	GetBySceneID(sceneID uint) ([]SceneFingerprint, error)
	DeleteBySceneID(sceneID uint) error
	GetFingerprintedSceneIDs() ([]uint, error)
	GetAllHashValues() ([]int64, error)
}

type FingerprintRepositoryImpl struct {
	DB *gorm.DB
}

func NewFingerprintRepository(db *gorm.DB) *FingerprintRepositoryImpl {
	return &FingerprintRepositoryImpl{DB: db}
}

func (r *FingerprintRepositoryImpl) BulkInsert(fingerprints []SceneFingerprint) error {
	if len(fingerprints) == 0 {
		return nil
	}
	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "scene_id"}, {Name: "frame_index"}},
		DoNothing: true,
	}).CreateInBatches(fingerprints, 500).Error
}

func (r *FingerprintRepositoryImpl) GetBySceneID(sceneID uint) ([]SceneFingerprint, error) {
	var fingerprints []SceneFingerprint
	err := r.DB.Where("scene_id = ?", sceneID).Order("frame_index ASC").Find(&fingerprints).Error
	return fingerprints, err
}

func (r *FingerprintRepositoryImpl) DeleteBySceneID(sceneID uint) error {
	return r.DB.Where("scene_id = ?", sceneID).Delete(&SceneFingerprint{}).Error
}

func (r *FingerprintRepositoryImpl) GetFingerprintedSceneIDs() ([]uint, error) {
	var ids []uint
	err := r.DB.Model(&SceneFingerprint{}).
		Distinct("scene_id").
		Pluck("scene_id", &ids).Error
	return ids, err
}

func (r *FingerprintRepositoryImpl) GetAllHashValues() ([]int64, error) {
	var hashes []int64
	err := r.DB.Model(&SceneFingerprint{}).
		Distinct("hash_value").
		Pluck("hash_value", &hashes).Error
	return hashes, err
}
