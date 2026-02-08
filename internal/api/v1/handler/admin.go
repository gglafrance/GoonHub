package handler

import (
	"goonhub/internal/api/middleware"
	"goonhub/internal/api/v1/request"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type AdminHandler struct {
	AdminService    *core.AdminService
	RBACService     *core.RBACService
	SceneService    *core.SceneService
	AppSettingsRepo data.AppSettingsRepository
}

func NewAdminHandler(adminService *core.AdminService, rbacService *core.RBACService, sceneService *core.SceneService, appSettingsRepo data.AppSettingsRepository) *AdminHandler {
	return &AdminHandler{
		AdminService:    adminService,
		RBACService:     rbacService,
		SceneService:    sceneService,
		AppSettingsRepo: appSettingsRepo,
	}
}

func (h *AdminHandler) ListUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	users, total, err := h.AdminService.ListUsers(page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list users"})
		return
	}

	type userResponse struct {
		ID          uint   `json:"id"`
		Username    string `json:"username"`
		Role        string `json:"role"`
		CreatedAt   string `json:"created_at"`
		LastLoginAt string `json:"last_login_at,omitempty"`
	}

	userList := make([]userResponse, 0, len(users))
	for _, u := range users {
		ur := userResponse{
			ID:        u.ID,
			Username:  u.Username,
			Role:      u.Role,
			CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
		if u.LastLoginAt != nil {
			ur.LastLoginAt = u.LastLoginAt.Format("2006-01-02T15:04:05Z")
		}
		userList = append(userList, ur)
	}

	c.JSON(http.StatusOK, gin.H{
		"users": userList,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func (h *AdminHandler) CreateUser(c *gin.Context) {
	var req request.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	if err := h.AdminService.CreateUser(req.Username, req.Password, req.Role); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "User created successfully"})
}

func (h *AdminHandler) UpdateUserRole(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req request.UpdateUserRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	if err := h.AdminService.UpdateUserRole(uint(id), req.Role); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User role updated successfully"})
}

func (h *AdminHandler) ResetUserPassword(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req request.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	if err := h.AdminService.ResetUserPassword(uint(id), req.NewPassword); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password reset successfully"})
}

func (h *AdminHandler) DeleteUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	userPayload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	if err := h.AdminService.DeleteUser(uint(id), userPayload.UserID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

func (h *AdminHandler) ListRoles(c *gin.Context) {
	roles, err := h.RBACService.GetRoles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list roles"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"roles": roles})
}

func (h *AdminHandler) ListPermissions(c *gin.Context) {
	permissions, err := h.RBACService.GetPermissions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list permissions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"permissions": permissions})
}

func (h *AdminHandler) SyncRolePermissions(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role ID"})
		return
	}

	var req request.SyncRolePermissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	if err := h.RBACService.SyncRolePermissions(uint(id), req.PermissionIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Role permissions updated successfully"})
}

// Trash management endpoints

func (h *AdminHandler) ListTrash(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	scenes, total, err := h.SceneService.ListTrashedScenes(page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list trashed scenes"})
		return
	}

	// Calculate expiry dates
	retentionDays := h.SceneService.GetTrashRetentionDays()

	type trashedSceneResponse struct {
		ID            uint   `json:"id"`
		Title         string `json:"title"`
		ThumbnailPath string `json:"thumbnail_path"`
		TrashedAt     string `json:"trashed_at"`
		ExpiresAt     string `json:"expires_at"`
	}

	results := make([]trashedSceneResponse, 0, len(scenes))
	for _, s := range scenes {
		if s.TrashedAt == nil {
			continue
		}
		expiresAt := s.TrashedAt.AddDate(0, 0, retentionDays)
		results = append(results, trashedSceneResponse{
			ID:            s.ID,
			Title:         s.Title,
			ThumbnailPath: s.ThumbnailPath,
			TrashedAt:     s.TrashedAt.Format("2006-01-02T15:04:05Z"),
			ExpiresAt:     expiresAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":           results,
		"total":          total,
		"page":           page,
		"limit":          limit,
		"retention_days": retentionDays,
	})
}

func (h *AdminHandler) RestoreScene(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scene ID"})
		return
	}

	if err := h.SceneService.RestoreSceneFromTrash(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Scene restored from trash"})
}

func (h *AdminHandler) PermanentDeleteScene(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scene ID"})
		return
	}

	if err := h.SceneService.HardDeleteScene(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *AdminHandler) EmptyTrash(c *gin.Context) {
	deleted, err := h.SceneService.EmptyTrash()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Trash emptied",
		"deleted": deleted,
	})
}

// App settings endpoints

func (h *AdminHandler) GetAppSettings(c *gin.Context) {
	settings, err := h.AppSettingsRepo.Get()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get app settings"})
		return
	}

	c.JSON(http.StatusOK, settings)
}

func (h *AdminHandler) UpdateAppSettings(c *gin.Context) {
	var req data.AppSettingsRecord
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	if err := h.AppSettingsRepo.Upsert(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update app settings"})
		return
	}

	updated, err := h.AppSettingsRepo.Get()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read updated settings"})
		return
	}

	c.JSON(http.StatusOK, updated)
}
