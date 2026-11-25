package handler

import (
	"encoding/json"
	"fmt"
	"goonhub/internal/api/middleware"
	"goonhub/internal/core"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type SSEHandler struct {
	eventBus         *core.EventBus
	authService      *core.AuthService
	jobStatusService *core.JobStatusService
	access           middleware.AccessResolver
	logger           *zap.Logger
}

func NewSSEHandler(eventBus *core.EventBus, authService *core.AuthService, jobStatusService *core.JobStatusService, access middleware.AccessResolver, logger *zap.Logger) *SSEHandler {
	return &SSEHandler{
		eventBus:         eventBus,
		authService:      authService,
		jobStatusService: jobStatusService,
		access:           access,
		logger:           logger.With(zap.String("handler", "sse")),
	}
}

// sseJobEventTypes are the job events (besides the jobs:* ones), sent only to the users
// with jobs:view_status.
var sseJobEventTypes = map[string]bool{
	"scene:retry_scheduled": true,
	"scene:dlq_added":       true,
	"scene:dlq_retry":       true,
	"scene:dlq_abandoned":   true,
}

// isJobEvent reports whether an event is about the job queue rather than a scene.
func isJobEvent(eventType string) bool {
	return strings.HasPrefix(eventType, "jobs:") || sseJobEventTypes[eventType]
}

// eventForUser returns the event as a user may see it, false when it must not be sent: job
// events need jobs:view_status, the error of a failed phase needs files:view_technical.
func eventForUser(event core.SceneEvent, access *core.UserAccess) (core.SceneEvent, bool) {
	if isJobEvent(event.Type) && !access.Has(core.PermJobsViewStatus) {
		return event, false
	}
	if data, ok := event.Data.(map[string]any); ok && !access.Has(core.PermFilesViewTech) {
		if _, hasErr := data["error"]; hasErr {
			cp := make(map[string]any, len(data))
			for k, v := range data {
				if k != "error" {
					cp[k] = v
				}
			}
			event.Data = cp
		}
	}
	return event, true
}

// currentAccess resolves the live access of the connected user (nil when it can no longer be
// resolved: the user was deleted).
func (h *SSEHandler) currentAccess(userID uint) *core.UserAccess {
	if h.access == nil {
		return nil
	}
	access, err := h.access.ResolveUser(userID)
	if err != nil {
		return nil
	}
	return access
}

// writeJobStatus marshals and writes the current job status as an SSE event.
func (h *SSEHandler) writeJobStatus(c *gin.Context) error {
	status := h.jobStatusService.GetJobStatus()
	statusData, err := json.Marshal(status)
	if err != nil {
		return nil // Marshal error, not a connection issue
	}
	_, writeErr := fmt.Fprintf(c.Writer, "event: jobs:status\ndata: %s\n\n", statusData)
	if writeErr != nil {
		return writeErr
	}
	c.Writer.Flush()
	return nil
}

func (h *SSEHandler) Stream(c *gin.Context) {
	// Get token from HTTP-only cookie only (query parameter auth removed for security)
	// Query params are logged in server access logs, browser history, and HTTP referrers
	token, err := c.Cookie(AuthCookieName)
	if err != nil || token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	payload, err := h.authService.ValidateToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	if h.access != nil {
		if _, err := h.access.ResolveUser(payload.UserID); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)

	// Send initial comment to establish connection
	fmt.Fprintf(c.Writer, ": connected\n\n")
	c.Writer.Flush()

	// Send initial job status immediately after connection (permissions are resolved again
	// for every event: a change applies without reconnecting)
	if h.jobStatusService != nil && h.currentAccess(payload.UserID).Has(core.PermJobsViewStatus) {
		h.writeJobStatus(c)
	}

	subscriberID, eventCh := h.eventBus.Subscribe()
	defer h.eventBus.Unsubscribe(subscriberID)

	h.logger.Debug("SSE client connected", zap.String("subscriber_id", subscriberID))

	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	statusTicker := time.NewTicker(3 * time.Second)
	defer statusTicker.Stop()

	clientGone := c.Request.Context().Done()

	for {
		select {
		case <-clientGone:
			h.logger.Debug("SSE client disconnected", zap.String("subscriber_id", subscriberID))
			return
		case event, ok := <-eventCh:
			if !ok {
				return
			}
			event, send := eventForUser(event, h.currentAccess(payload.UserID))
			if !send {
				continue
			}
			data, err := json.Marshal(event)
			if err != nil {
				h.logger.Error("Failed to marshal event", zap.Error(err))
				continue
			}
			_, writeErr := fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event.Type, string(data))
			if writeErr != nil {
				h.logger.Debug("SSE write failed, client likely disconnected",
					zap.String("subscriber_id", subscriberID),
					zap.Error(writeErr),
				)
				return
			}
			c.Writer.Flush()
		case <-statusTicker.C:
			if h.jobStatusService != nil && h.currentAccess(payload.UserID).Has(core.PermJobsViewStatus) {
				if writeErr := h.writeJobStatus(c); writeErr != nil {
					h.logger.Debug("SSE job status write failed, client likely disconnected",
						zap.String("subscriber_id", subscriberID),
						zap.Error(writeErr),
					)
					return
				}
			}
		case <-pingTicker.C:
			_, writeErr := fmt.Fprintf(c.Writer, ": ping\n\n")
			if writeErr != nil {
				h.logger.Debug("SSE ping failed, client likely disconnected",
					zap.String("subscriber_id", subscriberID),
					zap.Error(writeErr),
				)
				return
			}
			c.Writer.Flush()
		}
	}
}
