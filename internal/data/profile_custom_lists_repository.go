package data

import (
	"context"
	"time"
)

// Custom lists of the profiles (user_profile_lists, docs/PROFILES.md "Custom lists").

func (r *ProfileRepositoryImpl) Lists(ctx context.Context, userID uint) ([]ProfileList, error) {
	var lists []ProfileList
	if err := r.DB.WithContext(ctx).Where("user_id = ?", userID).Order("id").Find(&lists).Error; err != nil {
		return nil, err
	}
	return lists, nil
}

func (r *ProfileRepositoryImpl) ListsByIDs(ctx context.Context, userID uint, ids []uint) ([]ProfileList, error) {
	if len(ids) == 0 {
		return []ProfileList{}, nil
	}
	var lists []ProfileList
	if err := r.DB.WithContext(ctx).Where("user_id = ? AND id IN ?", userID, ids).Find(&lists).Error; err != nil {
		return nil, err
	}
	return lists, nil
}

func (r *ProfileRepositoryImpl) GetList(ctx context.Context, userID, id uint) (*ProfileList, error) {
	var list ProfileList
	if err := r.DB.WithContext(ctx).Where("user_id = ? AND id = ?", userID, id).Take(&list).Error; err != nil {
		return nil, err
	}
	return &list, nil
}

func (r *ProfileRepositoryImpl) CountLists(ctx context.Context, userID uint) (int64, error) {
	var n int64
	err := r.DB.WithContext(ctx).Model(&ProfileList{}).Where("user_id = ?", userID).Count(&n).Error
	return n, err
}

func (r *ProfileRepositoryImpl) CreateList(ctx context.Context, list *ProfileList) error {
	now := time.Now()
	list.CreatedAt, list.UpdatedAt = now, now
	return r.DB.WithContext(ctx).Create(list).Error
}

func (r *ProfileRepositoryImpl) UpdateList(ctx context.Context, list *ProfileList) error {
	list.UpdatedAt = time.Now()
	return r.DB.WithContext(ctx).Model(&ProfileList{}).Where("user_id = ? AND id = ?", list.UserID, list.ID).Updates(map[string]any{
		"title":             list.Title,
		"filters":           list.Filters,
		"saved_search_uuid": list.SavedSearchUUID,
		"audience":          list.Audience,
		"updated_at":        list.UpdatedAt,
	}).Error
}

func (r *ProfileRepositoryImpl) DeleteList(ctx context.Context, userID, id uint) (bool, error) {
	res := r.DB.WithContext(ctx).Where("user_id = ? AND id = ?", userID, id).Delete(&ProfileList{})
	return res.RowsAffected > 0, res.Error
}

func (r *ProfileRepositoryImpl) SaveTabs(ctx context.Context, userID uint, tabs ProfileTabs) error {
	updates := map[string]any{"tabs": tabs}
	if tabs == nil {
		updates["layout"] = ProfileLayout(nil)
	}
	return r.upsert(ctx, userID, updates)
}
