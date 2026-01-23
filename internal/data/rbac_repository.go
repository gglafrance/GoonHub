package data

import "gorm.io/gorm"

type RoleRepository interface {
	List() ([]Role, error)
	GetByName(name string) (*Role, error)
	GetByID(id uint) (*Role, error)
	Create(role *Role) error
	Update(role *Role) error
	Delete(id uint) error
	GetAllRolePermissions() (map[string][]string, error)
	CountUsersByRole(roleName string) (int64, error)
}

type PermissionRepository interface {
	List() ([]Permission, error)
	SyncRolePermissions(roleID uint, permissionIDs []uint) error
}

type RoleRepositoryImpl struct {
	DB *gorm.DB
}

func NewRoleRepository(db *gorm.DB) *RoleRepositoryImpl {
	return &RoleRepositoryImpl{DB: db}
}

func (r *RoleRepositoryImpl) List() ([]Role, error) {
	var roles []Role
	if err := r.DB.Preload("Permissions").Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

func (r *RoleRepositoryImpl) GetByName(name string) (*Role, error) {
	var role Role
	if err := r.DB.Preload("Permissions").Where("name = ?", name).First(&role).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *RoleRepositoryImpl) GetByID(id uint) (*Role, error) {
	var role Role
	if err := r.DB.Preload("Permissions").First(&role, id).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *RoleRepositoryImpl) Create(role *Role) error {
	return r.DB.Create(role).Error
}

func (r *RoleRepositoryImpl) Update(role *Role) error {
	return r.DB.Save(role).Error
}

func (r *RoleRepositoryImpl) Delete(id uint) error {
	return r.DB.Delete(&Role{}, id).Error
}

func (r *RoleRepositoryImpl) GetAllRolePermissions() (map[string][]string, error) {
	var roles []Role
	if err := r.DB.Preload("Permissions").Find(&roles).Error; err != nil {
		return nil, err
	}

	result := make(map[string][]string)
	for _, role := range roles {
		perms := make([]string, 0, len(role.Permissions))
		for _, p := range role.Permissions {
			perms = append(perms, p.Name)
		}
		result[role.Name] = perms
	}
	return result, nil
}

func (r *RoleRepositoryImpl) CountUsersByRole(roleName string) (int64, error) {
	var count int64
	if err := r.DB.Model(&User{}).Where("role = ?", roleName).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

type PermissionRepositoryImpl struct {
	DB *gorm.DB
}

func NewPermissionRepository(db *gorm.DB) *PermissionRepositoryImpl {
	return &PermissionRepositoryImpl{DB: db}
}

func (r *PermissionRepositoryImpl) List() ([]Permission, error) {
	var permissions []Permission
	if err := r.DB.Find(&permissions).Error; err != nil {
		return nil, err
	}
	return permissions, nil
}

func (r *PermissionRepositoryImpl) SyncRolePermissions(roleID uint, permissionIDs []uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&RolePermission{}).Error; err != nil {
			return err
		}

		for _, permID := range permissionIDs {
			rp := RolePermission{
				RoleID:       roleID,
				PermissionID: permID,
			}
			if err := tx.Create(&rp).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
