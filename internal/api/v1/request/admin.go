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

type SyncRolePermissionsRequest struct {
	PermissionIDs []uint `json:"permission_ids" binding:"required"`
}
