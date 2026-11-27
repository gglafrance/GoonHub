package data

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RBACGraph is everything needed to resolve role permissions: the roles, their parents and
// their own settings.
type RBACGraph struct {
	Roles    []Role
	Parents  []RoleParent
	Settings []RolePermission
}

type RoleRepository interface {
	List() ([]Role, error)
	GetByName(name string) (*Role, error)
	GetByID(id uint) (*Role, error)
	// Create inserts the role and its parents.
	Create(role *Role, parentIDs []uint) error
	// Update saves the role's name and description and replaces its parents.
	Update(role *Role, parentIDs []uint) error
	Delete(id uint) error
	CountUsersByRole(roleName string) (int64, error)
	// CountUsersPerRole returns the number of users holding each role, by role name.
	CountUsersPerRole() (map[string]int64, error)
	// LoadGraph returns every role with its parents and own settings.
	LoadGraph() (*RBACGraph, error)
	// ReplaceSettings replaces the role's own settings (absent permission = inherit).
	ReplaceSettings(roleID uint, settings []RolePermission) error
}

type PermissionRepository interface {
	List() ([]Permission, error)
	Create(permission *Permission) error
	// UpdateMeta updates the group and description of a permission.
	UpdateMeta(id uint, group, description string) error
	// GrantToRoles adds an allow setting for the permission to each role (an existing
	// setting is kept).
	GrantToRoles(permissionID uint, roleIDs []uint) error
	ListUserOverrides(userID uint) ([]UserPermissionOverride, error)
	// ReplaceUserOverrides replaces the user's overrides.
	ReplaceUserOverrides(userID uint, overrides []UserPermissionOverride) error
}

type RoleRepositoryImpl struct {
	DB *gorm.DB
}

func NewRoleRepository(db *gorm.DB) *RoleRepositoryImpl {
	return &RoleRepositoryImpl{DB: db}
}

func (r *RoleRepositoryImpl) List() ([]Role, error) {
	var roles []Role
	if err := r.DB.Order("id").Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

func (r *RoleRepositoryImpl) GetByName(name string) (*Role, error) {
	var role Role
	if err := r.DB.Where("name = ?", name).First(&role).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *RoleRepositoryImpl) GetByID(id uint) (*Role, error) {
	var role Role
	if err := r.DB.First(&role, id).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *RoleRepositoryImpl) Create(role *Role, parentIDs []uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(role).Error; err != nil {
			return err
		}
		return insertRoleParents(tx, role.ID, parentIDs)
	})
}

func (r *RoleRepositoryImpl) Update(role *Role, parentIDs []uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Role{}).Where("id = ?", role.ID).Updates(map[string]any{
			"name":        role.Name,
			"description": role.Description,
			"updated_at":  gorm.Expr("NOW()"),
		}).Error; err != nil {
			return err
		}
		if err := tx.Where("role_id = ?", role.ID).Delete(&RoleParent{}).Error; err != nil {
			return err
		}
		return insertRoleParents(tx, role.ID, parentIDs)
	})
}

func insertRoleParents(tx *gorm.DB, roleID uint, parentIDs []uint) error {
	if len(parentIDs) == 0 {
		return nil
	}
	rows := make([]RoleParent, len(parentIDs))
	for i, pid := range parentIDs {
		rows[i] = RoleParent{RoleID: roleID, ParentID: pid}
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
}

func (r *RoleRepositoryImpl) Delete(id uint) error {
	return r.DB.Delete(&Role{}, id).Error
}

func (r *RoleRepositoryImpl) CountUsersByRole(roleName string) (int64, error) {
	var count int64
	if err := r.DB.Model(&User{}).Where("role = ?", roleName).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *RoleRepositoryImpl) CountUsersPerRole() (map[string]int64, error) {
	var rows []struct {
		Role  string
		Count int64
	}
	if err := r.DB.Model(&User{}).Select("role, COUNT(*) AS count").Group("role").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, row := range rows {
		out[row.Role] = row.Count
	}
	return out, nil
}

func (r *RoleRepositoryImpl) LoadGraph() (*RBACGraph, error) {
	g := &RBACGraph{}
	if err := r.DB.Order("id").Find(&g.Roles).Error; err != nil {
		return nil, err
	}
	if err := r.DB.Order("role_id, parent_id").Find(&g.Parents).Error; err != nil {
		return nil, err
	}
	if err := r.DB.Order("role_id, permission_id").Find(&g.Settings).Error; err != nil {
		return nil, err
	}
	return g, nil
}

func (r *RoleRepositoryImpl) ReplaceSettings(roleID uint, settings []RolePermission) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&RolePermission{}).Error; err != nil {
			return err
		}
		if len(settings) == 0 {
			return nil
		}
		rows := make([]RolePermission, len(settings))
		for i, s := range settings {
			rows[i] = RolePermission{RoleID: roleID, PermissionID: s.PermissionID, Effect: s.Effect}
		}
		return tx.Create(&rows).Error
	})
}

type PermissionRepositoryImpl struct {
	DB *gorm.DB
}

func NewPermissionRepository(db *gorm.DB) *PermissionRepositoryImpl {
	return &PermissionRepositoryImpl{DB: db}
}

func (r *PermissionRepositoryImpl) List() ([]Permission, error) {
	var permissions []Permission
	if err := r.DB.Order("id").Find(&permissions).Error; err != nil {
		return nil, err
	}
	return permissions, nil
}

func (r *PermissionRepositoryImpl) Create(permission *Permission) error {
	return r.DB.Create(permission).Error
}

func (r *PermissionRepositoryImpl) UpdateMeta(id uint, group, description string) error {
	return r.DB.Model(&Permission{}).Where("id = ?", id).Updates(map[string]any{
		"group_name":  group,
		"description": description,
	}).Error
}

func (r *PermissionRepositoryImpl) GrantToRoles(permissionID uint, roleIDs []uint) error {
	if len(roleIDs) == 0 {
		return nil
	}
	rows := make([]RolePermission, len(roleIDs))
	for i, rid := range roleIDs {
		rows[i] = RolePermission{RoleID: rid, PermissionID: permissionID, Effect: PermissionEffectAllow}
	}
	return r.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
}

func (r *PermissionRepositoryImpl) ListUserOverrides(userID uint) ([]UserPermissionOverride, error) {
	var overrides []UserPermissionOverride
	if err := r.DB.Where("user_id = ?", userID).Order("permission_id").Find(&overrides).Error; err != nil {
		return nil, err
	}
	return overrides, nil
}

func (r *PermissionRepositoryImpl) ReplaceUserOverrides(userID uint, overrides []UserPermissionOverride) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&UserPermissionOverride{}).Error; err != nil {
			return err
		}
		if len(overrides) == 0 {
			return nil
		}
		rows := make([]UserPermissionOverride, len(overrides))
		for i, o := range overrides {
			rows[i] = UserPermissionOverride{UserID: userID, PermissionID: o.PermissionID, Effect: o.Effect}
		}
		return tx.Create(&rows).Error
	})
}
