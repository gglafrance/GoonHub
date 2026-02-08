package handler

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"goonhub/internal/api/v1/request"
	"goonhub/internal/api/v1/response"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/streaming"

	"github.com/gin-gonic/gin"
)

type ShareHandler struct {
	ShareService  *core.ShareService
	AuthService   *core.AuthService
	StreamManager *streaming.Manager
	ShareBaseURL  string
}

func NewShareHandler(
	shareService *core.ShareService,
	authService *core.AuthService,
	streamManager *streaming.Manager,
	shareBaseURL string,
) *ShareHandler {
	return &ShareHandler{
		ShareService:  shareService,
		AuthService:   authService,
		StreamManager: streamManager,
		ShareBaseURL:  shareBaseURL,
	}
}

func (h *ShareHandler) getUserID(c *gin.Context) (uint, bool) {
	user, exists := c.Get("user")
	if !exists {
		return 0, false
	}
	userPayload, ok := user.(*core.UserPayload)
	if !ok {
		return 0, false
	}
	return userPayload.UserID, true
}

// CreateShareLink creates a new share link for a scene.
func (h *ShareHandler) CreateShareLink(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		response.BadRequest(c, "failed to get user")
		return
	}

	sceneIDStr := c.Param("id")
	sceneID, err := strconv.ParseUint(sceneIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "invalid scene ID")
		return
	}

	var req request.CreateShareLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	link, err := h.ShareService.CreateShareLink(userID, uint(sceneID), req.ShareType, req.ExpiresIn)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Created(c, link)
}

// ListShareLinks returns the caller's share links for a scene.
func (h *ShareHandler) ListShareLinks(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		response.BadRequest(c, "failed to get user")
		return
	}

	sceneIDStr := c.Param("id")
	sceneID, err := strconv.ParseUint(sceneIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "invalid scene ID")
		return
	}

	links, err := h.ShareService.ListShareLinks(userID, uint(sceneID))
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, gin.H{
		"share_links":    links,
		"share_base_url": h.ShareBaseURL,
	})
}

// DeleteShareLink deletes a share link owned by the caller.
func (h *ShareHandler) DeleteShareLink(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		response.BadRequest(c, "failed to get user")
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "invalid share link ID")
		return
	}

	if err := h.ShareService.DeleteShareLink(uint(id), userID); err != nil {
		response.Error(c, err)
		return
	}

	response.NoContent(c)
}

// isAuthenticated checks if the request has a valid auth cookie/token.
func (h *ShareHandler) isAuthenticated(c *gin.Context) bool {
	token := ""

	if cookie, err := c.Cookie("goonhub_auth"); err == nil && cookie != "" {
		token = cookie
	}

	if token == "" {
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			t := strings.TrimPrefix(authHeader, "Bearer ")
			if t != authHeader {
				token = t
			}
		}
	}

	if token == "" {
		return false
	}

	_, err := h.AuthService.ValidateToken(token)
	return err == nil
}

// ResolveShareLink resolves a share token to scene data.
func (h *ShareHandler) ResolveShareLink(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		response.BadRequest(c, "missing share token")
		return
	}

	resolved, err := h.ShareService.ResolveShareLink(token, h.isAuthenticated(c))
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, resolved)
}

// StreamShareLink streams the video for a share link.
func (h *ShareHandler) StreamShareLink(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		response.BadRequest(c, "missing share token")
		return
	}

	// Validate the share link
	link, err := h.ShareService.GetShareLinkByToken(token)
	if err != nil {
		response.Error(c, err)
		return
	}

	// Check expiry
	if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
		c.JSON(http.StatusGone, gin.H{"error": "share link has expired", "code": "SHARE_LINK_EXPIRED"})
		return
	}

	// Check auth for auth_required type
	if link.ShareType == data.ShareTypeAuthRequired && !h.isAuthenticated(c) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required", "code": "AUTH_REQUIRED"})
		return
	}

	sceneID := link.SceneID
	clientIP := c.ClientIP()

	// Acquire stream slot
	if !h.StreamManager.Limiter().Acquire(clientIP, sceneID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Too many concurrent streams",
			"code":  "STREAM_LIMIT_EXCEEDED",
		})
		return
	}
	defer h.StreamManager.Limiter().Release(clientIP, sceneID)

	// Get cached path
	filePath, err := h.StreamManager.GetScenePath(sceneID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Scene not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get scene"})
		return
	}

	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Scene file not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open scene file"})
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to access scene file"})
		return
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = "video/mp4"
	}

	c.Header("Content-Type", mimeType)
	c.Header("Cache-Control", "public, max-age=86400")

	http.ServeContent(c.Writer, c.Request, filepath.Base(filePath), fileInfo.ModTime(), file)
}
