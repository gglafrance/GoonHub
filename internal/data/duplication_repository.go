package data

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DuplicateGroupRepository interface {
	Create(group *DuplicateGroup) error
	GetByID(id uint) (*DuplicateGroup, error)
	GetByIDWithMembers(id uint) (*DuplicateGroup, error)
	List(page, limit int, status string) ([]DuplicateGroup, int64, error)
	ListWithMembers(page, limit int, status string) ([]DuplicateGroup, int64, error)
	UpdateStatus(id uint, status string, resolvedAt *time.Time) error
	SetBestScene(id uint, sceneID uint) error
	AddMember(member *DuplicateGroupMember) error
	RemoveMember(groupID, sceneID uint) error
	GetGroupBySceneID(sceneID uint) (*DuplicateGroup, error)
	GetGroupsBySceneIDs(sceneIDs []uint) (map[uint]uint, error)
	MergeGroups(targetID uint, sourceIDs []uint) error
	UpdateSceneCount(id uint) error
	CountByStatus() (map[string]int64, error)
	Delete(id uint) error
	UpdateMemberBest(groupID uint, sceneID uint, isBest bool) error
	RunInTransaction(fn func(DuplicateGroupRepository) error) error
}

type DuplicationConfigRepository interface {
	Get() (*DuplicationConfigRecord, error)
	Upsert(record *DuplicationConfigRecord) error
}

// --- DuplicateGroupRepository Implementation ---

type DuplicateGroupRepositoryImpl struct {
	DB *gorm.DB
}

func NewDuplicateGroupRepository(db *gorm.DB) *DuplicateGroupRepositoryImpl {
	return &DuplicateGroupRepositoryImpl{DB: db}
}

func (r *DuplicateGroupRepositoryImpl) Create(group *DuplicateGroup) error {
	return r.DB.Create(group).Error
}

func (r *DuplicateGroupRepositoryImpl) GetByID(id uint) (*DuplicateGroup, error) {
	var group DuplicateGroup
	if err := r.DB.First(&group, id).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

func (r *DuplicateGroupRepositoryImpl) GetByIDWithMembers(id uint) (*DuplicateGroup, error) {
	var group DuplicateGroup
	if err := r.DB.Preload("Members").First(&group, id).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

func (r *DuplicateGroupRepositoryImpl) List(page, limit int, status string) ([]DuplicateGroup, int64, error) {
	var groups []DuplicateGroup
	var total int64
	offset := (page - 1) * limit

	query := r.DB.Model(&DuplicateGroup{})
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Limit(limit).Offset(offset).Order("created_at DESC").Find(&groups).Error; err != nil {
		return nil, 0, err
	}

	return groups, total, nil
}

func (r *DuplicateGroupRepositoryImpl) ListWithMembers(page, limit int, status string) ([]DuplicateGroup, int64, error) {
	var groups []DuplicateGroup
	var total int64
	offset := (page - 1) * limit

	countQuery := r.DB.Model(&DuplicateGroup{})
	if status != "" {
		countQuery = countQuery.Where("status = ?", status)
	}

	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	findQuery := r.DB.Preload("Members")
	if status != "" {
		findQuery = findQuery.Where("status = ?", status)
	}

	if err := findQuery.Limit(limit).Offset(offset).Order("created_at DESC").Find(&groups).Error; err != nil {
		return nil, 0, err
	}

	return groups, total, nil
}

func (r *DuplicateGroupRepositoryImpl) UpdateStatus(id uint, status string, resolvedAt *time.Time) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}
	if resolvedAt != nil {
		updates["resolved_at"] = resolvedAt
	}
	return r.DB.Model(&DuplicateGroup{}).Where("id = ?", id).Updates(updates).Error
}

func (r *DuplicateGroupRepositoryImpl) SetBestScene(id uint, sceneID uint) error {
	return r.DB.Model(&DuplicateGroup{}).Where("id = ?", id).Update("best_scene_id", sceneID).Error
}

func (r *DuplicateGroupRepositoryImpl) AddMember(member *DuplicateGroupMember) error {
	return r.DB.Create(member).Error
}

func (r *DuplicateGroupRepositoryImpl) RemoveMember(groupID, sceneID uint) error {
	return r.DB.Where("group_id = ? AND scene_id = ?", groupID, sceneID).Delete(&DuplicateGroupMember{}).Error
}

func (r *DuplicateGroupRepositoryImpl) GetGroupBySceneID(sceneID uint) (*DuplicateGroup, error) {
	var member DuplicateGroupMember
	if err := r.DB.Where("scene_id = ?", sceneID).First(&member).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	var group DuplicateGroup
	if err := r.DB.First(&group, member.GroupID).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

func (r *DuplicateGroupRepositoryImpl) GetGroupsBySceneIDs(sceneIDs []uint) (map[uint]uint, error) {
	if len(sceneIDs) == 0 {
		return make(map[uint]uint), nil
	}
	var members []DuplicateGroupMember
	if err := r.DB.Where("scene_id IN ?", sceneIDs).Find(&members).Error; err != nil {
		return nil, err
	}
	result := make(map[uint]uint, len(members))
	for _, m := range members {
		result[m.SceneID] = m.GroupID
	}
	return result, nil
}

func (r *DuplicateGroupRepositoryImpl) MergeGroups(targetID uint, sourceIDs []uint) error {
	if len(sourceIDs) == 0 {
		return nil
	}
	return r.DB.Transaction(func(tx *gorm.DB) error {
		// Find scene_ids already in the target group to avoid unique constraint violations
		var targetSceneIDs []uint
		if err := tx.Model(&DuplicateGroupMember{}).
			Where("group_id = ?", targetID).
			Pluck("scene_id", &targetSceneIDs).Error; err != nil {
			return err
		}

		// Delete overlapping members from source groups before moving
		if len(targetSceneIDs) > 0 {
			if err := tx.Where("group_id IN ? AND scene_id IN ?", sourceIDs, targetSceneIDs).
				Delete(&DuplicateGroupMember{}).Error; err != nil {
				return err
			}
		}

		// Move remaining members from source groups to target
		if err := tx.Model(&DuplicateGroupMember{}).
			Where("group_id IN ?", sourceIDs).
			Update("group_id", targetID).Error; err != nil {
			return err
		}
		// Delete source groups
		if err := tx.Where("id IN ?", sourceIDs).Delete(&DuplicateGroup{}).Error; err != nil {
			return err
		}
		// Update scene count on target
		var count int64
		if err := tx.Model(&DuplicateGroupMember{}).Where("group_id = ?", targetID).Count(&count).Error; err != nil {
			return err
		}
		return tx.Model(&DuplicateGroup{}).Where("id = ?", targetID).Updates(map[string]interface{}{
			"scene_count": count,
			"updated_at":  time.Now(),
		}).Error
	})
}

func (r *DuplicateGroupRepositoryImpl) UpdateSceneCount(id uint) error {
	var count int64
	if err := r.DB.Model(&DuplicateGroupMember{}).Where("group_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	return r.DB.Model(&DuplicateGroup{}).Where("id = ?", id).Updates(map[string]interface{}{
		"scene_count": count,
		"updated_at":  time.Now(),
	}).Error
}

func (r *DuplicateGroupRepositoryImpl) CountByStatus() (map[string]int64, error) {
	type statusCount struct {
		Status string
		Count  int64
	}
	var results []statusCount
	if err := r.DB.Model(&DuplicateGroup{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Scan(&results).Error; err != nil {
		return nil, err
	}
	counts := make(map[string]int64)
	for _, sc := range results {
		counts[sc.Status] = sc.Count
	}
	return counts, nil
}

func (r *DuplicateGroupRepositoryImpl) Delete(id uint) error {
	return r.DB.Delete(&DuplicateGroup{}, id).Error
}

func (r *DuplicateGroupRepositoryImpl) UpdateMemberBest(groupID uint, sceneID uint, isBest bool) error {
	return r.DB.Model(&DuplicateGroupMember{}).
		Where("group_id = ? AND scene_id = ?", groupID, sceneID).
		Update("is_best", isBest).Error
}

func (r *DuplicateGroupRepositoryImpl) RunInTransaction(fn func(DuplicateGroupRepository) error) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		txRepo := &DuplicateGroupRepositoryImpl{DB: tx}
		return fn(txRepo)
	})
}

// --- DuplicationConfigRepository Implementation ---

type DuplicationConfigRepositoryImpl struct {
	DB *gorm.DB
}

func NewDuplicationConfigRepository(db *gorm.DB) *DuplicationConfigRepositoryImpl {
	return &DuplicationConfigRepositoryImpl{DB: db}
}

func (r *DuplicationConfigRepositoryImpl) Get() (*DuplicationConfigRecord, error) {
	var record DuplicationConfigRecord
	err := r.DB.First(&record).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

func (r *DuplicationConfigRepositoryImpl) Upsert(record *DuplicationConfigRecord) error {
	record.ID = 1
	record.UpdatedAt = time.Now()
	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"audio_density_threshold", "audio_min_hashes", "audio_max_hash_occurrences", "audio_min_span", "visual_hamming_max", "visual_min_frames", "visual_min_span", "delta_tolerance", "fingerprint_mode", "updated_at"}),
	}).Create(record).Error
}
