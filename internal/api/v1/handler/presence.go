package handler

import (
	"goonhub/internal/api/middleware"
	"goonhub/internal/api/v1/request"
	"goonhub/internal/api/v1/response"
	"goonhub/internal/apperrors"
	"goonhub/internal/core"

	"github.com/gin-gonic/gin"
)

// PresenceHandler serves the presence (docs/PRESENCE.md): the snapshot of the contacts'
// presence, the reports of the client (heartbeat, activity) and the invisible mode. Every route
// needs a signed-in user; privacy is checked by the service for every viewer (no admin bypass).
type PresenceHandler struct {
	presence *core.PresenceService
}

func NewPresenceHandler(presence *core.PresenceService) *PresenceHandler {
	return &PresenceHandler{presence: presence}
}

func presenceUserID(c *gin.Context) (uint, bool) {
	user, exists := c.Get("user")
	if !exists {
		return 0, false
	}
	payload, ok := user.(*core.UserPayload)
	if !ok || payload.UserID == 0 {
		return 0, false
	}
	return payload.UserID, true
}

// Snapshot is GET /social/presence: the friends and followed users whose presence I may see.
func (h *PresenceHandler) Snapshot(c *gin.Context) {
	userID, ok := presenceUserID(c)
	if !ok {
		socialUnauthorized(c)
		return
	}
	snap, err := h.presence.Snapshot(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, response.NewPresenceSnapshotResponse(snap))
}

// GetUser is GET /social/presence/users/:userId: the presence of one user as I may see it
// ({visible: false} when hidden).
func (h *PresenceHandler) GetUser(c *gin.Context) {
	userID, ok := presenceUserID(c)
	if !ok {
		socialUnauthorized(c)
		return
	}
	target, ok := targetID(c)
	if !ok {
		return
	}
	view, err := h.presence.ViewOf(c.Request.Context(), userID, target)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, view)
}

// Heartbeat is POST /social/presence/heartbeat {active}: the tab was used (not idle).
func (h *PresenceHandler) Heartbeat(c *gin.Context) {
	userID, ok := presenceUserID(c)
	if !ok {
		socialUnauthorized(c)
		return
	}
	var req request.PresenceHeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	h.presence.Heartbeat(userID, req.Active)
	response.NoContent(c)
}

// UpdateActivity is PUT /social/presence/activity: the activity of a tab (start, progress,
// pause). Watching a scene needs scenes:view.
func (h *PresenceHandler) UpdateActivity(c *gin.Context) {
	userID, ok := presenceUserID(c)
	if !ok {
		socialUnauthorized(c)
		return
	}
	var req request.PresenceActivityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	if req.Type == core.ActivityWatchingScene && !middleware.HasPermission(c, core.PermScenesView) {
		response.Error(c, apperrors.NewForbiddenError("watching a scene needs the scenes:view permission"))
		return
	}
	err := h.presence.UpdateActivity(userID, core.PresenceActivityInput{
		TabID:    req.TabID,
		Type:     req.Type,
		SceneID:  req.SceneID,
		Position: req.Position,
		Paused:   req.Paused,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

// ClearActivity is DELETE /social/presence/activity?tab_id=: the tab left its activity.
func (h *PresenceHandler) ClearActivity(c *gin.Context) {
	userID, ok := presenceUserID(c)
	if !ok {
		socialUnauthorized(c)
		return
	}
	if err := h.presence.ClearActivity(userID, c.Query("tab_id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

// GetSettings is GET /social/presence/settings.
func (h *PresenceHandler) GetSettings(c *gin.Context) {
	userID, ok := presenceUserID(c)
	if !ok {
		socialUnauthorized(c)
		return
	}
	invisible, err := h.presence.Invisible(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, response.PresenceSettingsResponse{Invisible: invisible})
}

// UpdateSettings is PUT /social/presence/settings {invisible}.
func (h *PresenceHandler) UpdateSettings(c *gin.Context) {
	userID, ok := presenceUserID(c)
	if !ok {
		socialUnauthorized(c)
		return
	}
	var req request.PresenceSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	if err := h.presence.SetInvisible(c.Request.Context(), userID, *req.Invisible); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, response.PresenceSettingsResponse{Invisible: *req.Invisible})
}
