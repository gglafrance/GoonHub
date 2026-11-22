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
	// ListWithMembers lists groups with their members, optionally filtered by status and by
	// kind (DuplicateKind*, derived from the members' relations).
	ListWithMembers(page, limit int, status, kind string) ([]DuplicateGroup, int64, error)
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
	// FillMemberSegment stores the matching segment (and its relation) of a member that has none yet.
	FillMemberSegment(member *DuplicateGroupMember) error
	// ListMembersForRelationBackfill returns, by ascending ID after afterID, the members that
	// have a segment but no relation yet, with the durations of their scene and reference.
	ListMembersForRelationBackfill(afterID uint, limit int) ([]DuplicateMemberRelationInput, error)
	// SetMemberRelation stores the relation of a member to its reference.
	SetMemberRelation(memberID uint, relation string) error
	// SetMemberDeleteAfter schedules (or cancels, with nil) the deletion of group members.
	SetMemberDeleteAfter(groupID uint, sceneIDs []uint, deleteAfter *time.Time) error
	// ListMembersDueForDeletion returns members whose scheduled deletion time has passed.
	ListMembersDueForDeletion(now time.Time, limit int) ([]DuplicateGroupMember, error)
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

func (r *DuplicateGroupRepositoryImpl) ListWithMembers(page, limit int, status, kind string) ([]DuplicateGroup, int64, error) {
	var groups []DuplicateGroup
	var total int64
	offset := (page - 1) * limit

	countQuery := r.DB.Model(&DuplicateGroup{})
	if status != "" {
		countQuery = countQuery.Where("status = ?", status)
	}
	countQuery = whereDuplicateKind(countQuery, kind)

	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	findQuery := r.DB.Preload("Members")
	if status != "" {
		findQuery = findQuery.Where("status = ?", status)
	}
	findQuery = whereDuplicateKind(findQuery, kind)

	if err := findQuery.Limit(limit).Offset(offset).Order("created_at DESC").Find(&groups).Error; err != nil {
		return nil, 0, err
	}

	return groups, total, nil
}

// Subqueries on the members of the listed group, for whereDuplicateKind
const (
	duplicateGroupHasMembers    = "EXISTS (SELECT 1 FROM duplicate_group_members dm WHERE dm.group_id = duplicate_groups.id)"
	duplicateGroupHasRelation   = "EXISTS (SELECT 1 FROM duplicate_group_members dm WHERE dm.group_id = duplicate_groups.id AND dm.relation IN ?)"
	duplicateGroupLacksRelation = "NOT " + duplicateGroupHasRelation
)

// whereDuplicateKind keeps the groups of the given kind, with the same precedence as
// core.DuplicateGroupKind (containment, then partial, then unverified, else identical).
// An empty or unknown kind keeps every group.
func whereDuplicateKind(query *gorm.DB, kind string) *gorm.DB {
	containment := []string{DuplicateRelationContained, DuplicateRelationContains}
	switch kind {
	case DuplicateKindIdentical:
		return query.Where(duplicateGroupHasMembers).
			Where(duplicateGroupLacksRelation, append(containment, DuplicateRelationPartial, DuplicateRelationUnknown))
	case DuplicateKindContainment:
		return query.Where(duplicateGroupHasRelation, containment)
	case DuplicateKindPartial:
		return query.Where(duplicateGroupHasRelation, []string{DuplicateRelationPartial}).
			Where(duplicateGroupLacksRelation, containment)
	case DuplicateKindUnverified:
		return query.Where(duplicateGroupHasRelation, []string{DuplicateRelationUnknown}).
			Where(duplicateGroupLacksRelation, append(containment, DuplicateRelationPartial))
	default:
		return query
	}
}

func (r *DuplicateGroupRepositoryImpl) UpdateStatus(id uint, status string, resolvedAt *time.Time) error {
	updates := map[string]interface{}{
		"status":      status,
		"updated_at":  time.Now(),
		"resolved_at": resolvedAt,
	}
	return r.DB.Model(&DuplicateGroup{}).Where("id = ?", id).Updates(updates).Error
}

func (r *DuplicateGroupRepositoryImpl) SetBestScene(id uint, sceneID uint) error {
	return r.DB.Model(&DuplicateGroup{}).Where("id = ?", id).Update("best_scene_id", sceneID).Error
}

func (r *DuplicateGroupRepositoryImpl) AddMember(member *DuplicateGroupMember) error {
	return r.DB.Create(member).Error
}

func (r *DuplicateGroupRepositoryImpl) FillMemberSegment(member *DuplicateGroupMember) error {
	return r.DB.Model(&DuplicateGroupMember{}).
		Where("group_id = ? AND scene_id = ? AND match_start IS NULL", member.GroupID, member.SceneID).
		Updates(map[string]any{
			"match_start":  member.MatchStart,
			"match_end":    member.MatchEnd,
			"ref_scene_id": member.RefSceneID,
			"ref_start":    member.RefStart,
			"ref_end":      member.RefEnd,
			"relation":     DuplicateRelationOrUnknown(member.Relation),
		}).Error
}

// DuplicateRelationOrUnknown maps an unset relation to DuplicateRelationUnknown.
func DuplicateRelationOrUnknown(relation string) string {
	if relation == "" {
		return DuplicateRelationUnknown
	}
	return relation
}

func (r *DuplicateGroupRepositoryImpl) ListMembersForRelationBackfill(afterID uint, limit int) ([]DuplicateMemberRelationInput, error) {
	var rows []DuplicateMemberRelationInput
	err := r.DB.Table("duplicate_group_members AS m").
		Select(`m.id, m.match_start, m.match_end, m.ref_start, m.ref_end,
			COALESCE(s.duration, 0) AS duration, COALESCE(ref.duration, 0) AS ref_duration`).
		Joins("LEFT JOIN scenes s ON s.id = m.scene_id").
		Joins("LEFT JOIN scenes ref ON ref.id = m.ref_scene_id").
		Where("m.relation = ? AND m.match_start IS NOT NULL AND m.id > ?", DuplicateRelationUnknown, afterID).
		Order("m.id ASC").
		Limit(limit).
		Scan(&rows).Error
	return rows, err
}

func (r *DuplicateGroupRepositoryImpl) SetMemberRelation(memberID uint, relation string) error {
	return r.DB.Model(&DuplicateGroupMember{}).
		Where("id = ?", memberID).
		Update("relation", DuplicateRelationOrUnknown(relation)).Error
}

func (r *DuplicateGroupRepositoryImpl) SetMemberDeleteAfter(groupID uint, sceneIDs []uint, deleteAfter *time.Time) error {
	if len(sceneIDs) == 0 {
		return nil
	}
	return r.DB.Model(&DuplicateGroupMember{}).
		Where("group_id = ? AND scene_id IN ?", groupID, sceneIDs).
		Update("delete_after", deleteAfter).Error
}

func (r *DuplicateGroupRepositoryImpl) ListMembersDueForDeletion(now time.Time, limit int) ([]DuplicateGroupMember, error) {
	var members []DuplicateGroupMember
	err := r.DB.Where("delete_after IS NOT NULL AND delete_after <= ?", now).
		Order("delete_after ASC").
		Limit(limit).
		Find(&members).Error
	return members, err
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
		DoUpdates: clause.AssignmentColumns([]string{"audio_density_threshold", "audio_min_hashes", "audio_max_hash_occurrences", "audio_min_span", "visual_hamming_max", "visual_min_frames", "visual_min_span", "delta_tolerance", "fingerprint_mode", "delete_resolved_files", "deletion_delay_hours", "best_selection_criteria", "updated_at"}),
	}).Create(record).Error
}
