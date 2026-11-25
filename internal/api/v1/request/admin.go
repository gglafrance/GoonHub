package request

type CreateUserRequest struct {
	Username string `json:"username" binding:"required,min=3,max=64"`
	Password string `json:"password" binding:"required,min=12,max=128"`
	Role     string `json:"role" binding:"required"`
}

type UpdateUserRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

// UpdateUserShareMarkersRequest sets whether a user's markers are visible to every user.
type UpdateUserShareMarkersRequest struct {
	ShareMarkers *bool `json:"share_markers" binding:"required"`
}

type ResetPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required,min=12,max=128"`
}

// RoleRequest is the body of a role creation or update.
type RoleRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	ParentIDs   []uint `json:"parent_ids"`
}

// PermissionSettingRequest is a role setting or a user override: allow or deny.
type PermissionSettingRequest struct {
	PermissionID uint   `json:"permission_id" binding:"required"`
	Effect       string `json:"effect" binding:"required,oneof=allow deny"`
}

// SetRolePermissionsRequest replaces a role's own settings (absent permission = inherit).
type SetRolePermissionsRequest struct {
	Settings []PermissionSettingRequest `json:"settings" binding:"dive"`
}

// SetUserPermissionsRequest replaces a user's overrides.
type SetUserPermissionsRequest struct {
	Overrides []PermissionSettingRequest `json:"overrides" binding:"dive"`
}
