package handler

import (
	"errors"
	"net/http"
	"strconv"

	"goonhub/internal/api/v1/request"
	"goonhub/internal/api/v1/response"
	"goonhub/internal/apperrors"
	"goonhub/internal/core"

	"github.com/gin-gonic/gin"
)

// RBACManager manages roles, their settings and user overrides (implemented by
// core.RBACService). See docs/RBAC.md.
type RBACManager interface {
	ListPermissions() []core.PermissionView
	ListRoles() ([]core.RoleView, error)
	GetRole(id uint) (*core.RoleView, error)
	CreateRole(in core.RoleInput) (*core.RoleView, error)
	UpdateRole(id uint, in core.RoleInput) (*core.RoleView, error)
	DeleteRole(id uint) error
	SetRoleSettings(id uint, settings []core.PermissionSetting) error
	RoleEffective(id uint) ([]core.EffectivePermission, error)
	UserPermissions(userID uint) (*core.UserPermissionsView, error)
	SetUserOverrides(userID uint, overrides []core.PermissionSetting) error
}

// adminUserError answers a user management error: typed errors with their status, the
// others (plain validation messages of the admin service) with 400.
func adminUserError(c *gin.Context, err error) {
	var appErr apperrors.AppError
	if errors.As(err, &appErr) {
		response.Error(c, err)
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
}

func parseIDParam(c *gin.Context, what string) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		response.Error(c, apperrors.NewValidationError("invalid "+what+" ID"))
		return 0, false
	}
	return uint(id), true
}

func toPermissionSettings(in []request.PermissionSettingRequest) []core.PermissionSetting {
	out := make([]core.PermissionSetting, len(in))
	for i, s := range in {
		out[i] = core.PermissionSetting{PermissionID: s.PermissionID, Effect: s.Effect}
	}
	return out
}

// ListPermissions lists the catalog permissions (deprecated ones excluded), in catalog order.
// GET /admin/permissions
func (h *AdminHandler) ListPermissions(c *gin.Context) {
	response.OK(c, gin.H{"permissions": h.RBACService.ListPermissions()})
}

// ListRoles lists the roles with their parents, own settings and number of users.
// GET /admin/roles
func (h *AdminHandler) ListRoles(c *gin.Context) {
	roles, err := h.RBACService.ListRoles()
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"roles": roles})
}

// CreateRole creates a role and returns it (201). POST /admin/roles
func (h *AdminHandler) CreateRole(c *gin.Context) {
	var req request.RoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.NewValidationError("invalid request: "+err.Error()))
		return
	}
	role, err := h.RBACService.CreateRole(core.RoleInput{Name: req.Name, Description: req.Description, ParentIDs: req.ParentIDs})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, role)
}

// UpdateRole renames a role, changes its description and replaces its parents, and returns
// it. PUT /admin/roles/:id
func (h *AdminHandler) UpdateRole(c *gin.Context) {
	id, ok := parseIDParam(c, "role")
	if !ok {
		return
	}
	var req request.RoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.NewValidationError("invalid request: "+err.Error()))
		return
	}
	role, err := h.RBACService.UpdateRole(id, core.RoleInput{Name: req.Name, Description: req.Description, ParentIDs: req.ParentIDs})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, role)
}

// DeleteRole deletes a role: 400 for a system role, 409 (with user_count) while users hold
// it. DELETE /admin/roles/:id
func (h *AdminHandler) DeleteRole(c *gin.Context) {
	id, ok := parseIDParam(c, "role")
	if !ok {
		return
	}
	if err := h.RBACService.DeleteRole(id); err != nil {
		var inUse *core.RoleInUseError
		if errors.As(err, &inUse) {
			c.JSON(http.StatusConflict, gin.H{"error": inUse.Error(), "code": "ROLE_IN_USE", "user_count": inUse.UserCount})
			return
		}
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"message": "Role deleted"})
}

// SetRolePermissions replaces a role's own settings and returns the role.
// PUT /admin/roles/:id/permissions
func (h *AdminHandler) SetRolePermissions(c *gin.Context) {
	id, ok := parseIDParam(c, "role")
	if !ok {
		return
	}
	var req request.SetRolePermissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.NewValidationError("invalid request: "+err.Error()))
		return
	}
	if req.Settings == nil {
		response.Error(c, apperrors.NewValidationErrorWithField("settings", "settings is required (an empty list clears the settings of the role)"))
		return
	}
	if err := h.RBACService.SetRoleSettings(id, toPermissionSettings(req.Settings)); err != nil {
		response.Error(c, err)
		return
	}
	role, err := h.RBACService.GetRole(id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, role)
}

// GetRoleEffective returns the effective permissions of a role with their source.
// GET /admin/roles/:id/effective
func (h *AdminHandler) GetRoleEffective(c *gin.Context) {
	id, ok := parseIDParam(c, "role")
	if !ok {
		return
	}
	perms, err := h.RBACService.RoleEffective(id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"permissions": perms})
}

// GetUserPermissions returns a user's role, overrides and effective permissions.
// GET /admin/users/:id/permissions
func (h *AdminHandler) GetUserPermissions(c *gin.Context) {
	id, ok := parseIDParam(c, "user")
	if !ok {
		return
	}
	view, err := h.RBACService.UserPermissions(id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, view)
}

// SetUserPermissions replaces a user's overrides and returns the updated view.
// PUT /admin/users/:id/permissions
func (h *AdminHandler) SetUserPermissions(c *gin.Context) {
	id, ok := parseIDParam(c, "user")
	if !ok {
		return
	}
	var req request.SetUserPermissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.NewValidationError("invalid request: "+err.Error()))
		return
	}
	if req.Overrides == nil {
		response.Error(c, apperrors.NewValidationErrorWithField("overrides", "overrides is required (an empty list clears the overrides of the user)"))
		return
	}
	if err := h.RBACService.SetUserOverrides(id, toPermissionSettings(req.Overrides)); err != nil {
		response.Error(c, err)
		return
	}
	view, err := h.RBACService.UserPermissions(id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, view)
}
