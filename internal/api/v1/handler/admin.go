package handler

import (
	"errors"
	"goonhub/internal/api/middleware"
	"goonhub/internal/api/v1/request"
	"goonhub/internal/api/v1/response"
	"goonhub/internal/apperrors"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type AdminHandler struct {
	AdminService    *core.AdminService
	RBACService     RBACManager
	SceneService    *core.SceneService
	AppSettingsRepo data.AppSettingsRepository
}

func NewAdminHandler(adminService *core.AdminService, rbacService RBACManager, sceneService *core.SceneService, appSettingsRepo data.AppSettingsRepository) *AdminHandler {
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
		ID           uint   `json:"id"`
		Username     string `json:"username"`
		Role         string `json:"role"`
		ShareMarkers bool   `json:"share_markers"`
		CreatedAt    string `json:"created_at"`
		LastLoginAt  string `json:"last_login_at,omitempty"`
	}

	userList := make([]userResponse, 0, len(users))
	for _, u := range users {
		ur := userResponse{
			ID:           u.ID,
			Username:     u.Username,
			Role:         u.Role,
			ShareMarkers: u.ShareMarkers,
			CreatedAt:    u.CreatedAt.Format("2006-01-02T15:04:05Z"),
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

	if err := h.AdminService.CreateUser(middleware.GetAccess(c), req.Username, req.Password, req.Role); err != nil {
		adminUserError(c, err)
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

	if err := h.AdminService.UpdateUserRole(middleware.GetAccess(c), uint(id), req.Role); err != nil {
		adminUserError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User role updated successfully"})
}

// UpdateUserShareMarkers sets whether a user's markers are visible (read-only) to every user.
func (h *AdminHandler) UpdateUserShareMarkers(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Error(c, apperrors.NewValidationError("invalid user ID"))
		return
	}

	var req request.UpdateUserShareMarkersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.NewValidationError("invalid request: "+err.Error()))
		return
	}

	if err := h.AdminService.SetUserShareMarkers(uint(id), *req.ShareMarkers); err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{"id": uint(id), "share_markers": *req.ShareMarkers})
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

	if err := h.AdminService.ResetUserPassword(middleware.GetAccess(c), uint(id), req.NewPassword); err != nil {
		var appErr apperrors.AppError
		if errors.As(err, &appErr) {
			response.Error(c, err)
			return
		}
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

	if err := h.AdminService.DeleteUser(middleware.GetAccess(c), uint(id)); err != nil {
		adminUserError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

// Trash management endpoints

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
	// Start from the current settings so that fields missing from the request keep their
	// value instead of being reset (e.g. transcoding_enabled from an older client)
	current, err := h.AppSettingsRepo.Get()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get app settings"})
		return
	}
	req := *current
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
