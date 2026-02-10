package data

import (
	"time"

	"gorm.io/gorm"
)

// DuplicateRepository handles duplicate group persistence.
type DuplicateRepository interface {
	CreateGroup(group *DuplicateGroup) error
	GetGroupByID(id uint) (*DuplicateGroup, error)
	GetGroupByIDWithMembers(id uint) (*DuplicateGroup, error)
	ListGroups(page, limit int, status string) ([]DuplicateGroup, int64, error)
	UpdateGroupStatus(id uint, status string) error
	SetGroupWinner(id uint, winnerSceneID uint) error
	DeleteGroup(id uint) error
	CountPendingGroups() (int64, error)

	AddMember(member *DuplicateGroupMember) error
	GetMembersForGroup(groupID uint) ([]DuplicateGroupMember, error)
	GetGroupForScene(sceneID uint) (*DuplicateGroup, error)
	SetMemberWinner(groupID, sceneID uint) error
	ClearMemberWinners(groupID uint) error
}

type DuplicateRepositoryImpl struct {
	DB *gorm.DB
}

func NewDuplicateRepository(db *gorm.DB) *DuplicateRepositoryImpl {
	return &DuplicateRepositoryImpl{DB: db}
}

func (r *DuplicateRepositoryImpl) CreateGroup(group *DuplicateGroup) error {
	return r.DB.Create(group).Error
}

func (r *DuplicateRepositoryImpl) GetGroupByID(id uint) (*DuplicateGroup, error) {
	var group DuplicateGroup
	if err := r.DB.First(&group, id).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

func (r *DuplicateRepositoryImpl) GetGroupByIDWithMembers(id uint) (*DuplicateGroup, error) {
	var group DuplicateGroup
	if err := r.DB.Preload("Members").First(&group, id).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

func (r *DuplicateRepositoryImpl) ListGroups(page, limit int, status string) ([]DuplicateGroup, int64, error) {
	var groups []DuplicateGroup
	var total int64

	query := r.DB.Model(&DuplicateGroup{})
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	if err := query.Preload("Members").
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&groups).Error; err != nil {
		return nil, 0, err
	}

	return groups, total, nil
}

func (r *DuplicateRepositoryImpl) UpdateGroupStatus(id uint, status string) error {
	updates := map[string]any{
		"status":     status,
		"updated_at": time.Now(),
	}
	if status == "resolved" || status == "dismissed" {
		now := time.Now()
		updates["resolved_at"] = now
	}
	return r.DB.Model(&DuplicateGroup{}).Where("id = ?", id).Updates(updates).Error
}

func (r *DuplicateRepositoryImpl) SetGroupWinner(id uint, winnerSceneID uint) error {
	return r.DB.Model(&DuplicateGroup{}).Where("id = ?", id).Updates(map[string]any{
		"winner_scene_id": winnerSceneID,
		"updated_at":      time.Now(),
	}).Error
}

func (r *DuplicateRepositoryImpl) DeleteGroup(id uint) error {
	return r.DB.Delete(&DuplicateGroup{}, id).Error
}

func (r *DuplicateRepositoryImpl) CountPendingGroups() (int64, error) {
	var count int64
	err := r.DB.Model(&DuplicateGroup{}).Where("status = 'pending'").Count(&count).Error
	return count, err
}

func (r *DuplicateRepositoryImpl) AddMember(member *DuplicateGroupMember) error {
	return r.DB.Create(member).Error
}

func (r *DuplicateRepositoryImpl) GetMembersForGroup(groupID uint) ([]DuplicateGroupMember, error) {
	var members []DuplicateGroupMember
	err := r.DB.Where("group_id = ?", groupID).Order("match_percentage DESC").Find(&members).Error
	return members, err
}

func (r *DuplicateRepositoryImpl) GetGroupForScene(sceneID uint) (*DuplicateGroup, error) {
	var member DuplicateGroupMember
	if err := r.DB.Where("scene_id = ?", sceneID).First(&member).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return r.GetGroupByIDWithMembers(member.GroupID)
}

func (r *DuplicateRepositoryImpl) SetMemberWinner(groupID, sceneID uint) error {
	return r.DB.Model(&DuplicateGroupMember{}).
		Where("group_id = ? AND scene_id = ?", groupID, sceneID).
		Update("is_winner", true).Error
}

func (r *DuplicateRepositoryImpl) ClearMemberWinners(groupID uint) error {
	return r.DB.Model(&DuplicateGroupMember{}).
		Where("group_id = ?", groupID).
		Update("is_winner", false).Error
}
