package handler

import (
	"encoding/json"
	"fmt"
	"goonhub/internal/core"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type SSEHandler struct {
	eventBus         *core.EventBus
	authService      *core.AuthService
	jobStatusService *core.JobStatusService
	logger           *zap.Logger
}

func NewSSEHandler(eventBus *core.EventBus, authService *core.AuthService, jobStatusService *core.JobStatusService, logger *zap.Logger) *SSEHandler {
	return &SSEHandler{
		eventBus:         eventBus,
		authService:      authService,
		jobStatusService: jobStatusService,
		logger:           logger.With(zap.String("handler", "sse")),
	}
}

// isJobRelatedEvent returns true for events that indicate job state changes
func isJobRelatedEvent(eventType string) bool {
	switch eventType {
	case "scene:metadata_complete", "scene:thumbnail_complete", "scene:sprites_complete",
		"scene:completed", "scene:failed", "scene:cancelled", "scene:timed_out":
		return true
	default:
		return false
	}
}

func (h *SSEHandler) Stream(c *gin.Context) {
	// Get token from HTTP-only cookie only (query parameter auth removed for security)
	// Query params are logged in server access logs, browser history, and HTTP referrers
	token, err := c.Cookie(AuthCookieName)
	if err != nil || token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	_, err = h.authService.ValidateToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)

	// Send initial comment to establish connection
	fmt.Fprintf(c.Writer, ": connected\n\n")
	c.Writer.Flush()

	// Send initial job status immediately after connection
	if h.jobStatusService != nil {
		status := h.jobStatusService.GetJobStatus()
		if statusData, err := json.Marshal(status); err == nil {
			fmt.Fprintf(c.Writer, "event: jobs:status\ndata: %s\n\n", statusData)
			c.Writer.Flush()
		}
	}

	subscriberID, eventCh := h.eventBus.Subscribe()
	defer h.eventBus.Unsubscribe(subscriberID)

	h.logger.Debug("SSE client connected", zap.String("subscriber_id", subscriberID))

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

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

			// Broadcast job status after job-related events
			if h.jobStatusService != nil && isJobRelatedEvent(event.Type) {
				h.logger.Debug("Broadcasting job status after event", zap.String("event_type", event.Type))
				status := h.jobStatusService.GetJobStatus()
				if statusData, err := json.Marshal(status); err == nil {
					fmt.Fprintf(c.Writer, "event: jobs:status\ndata: %s\n\n", statusData)
					c.Writer.Flush()
				}
			}
		case <-ticker.C:
			_, writeErr := fmt.Fprintf(c.Writer, ": ping\n\n")
			if writeErr != nil {
				h.logger.Debug("SSE ping failed, client likely disconnected",
					zap.String("subscriber_id", subscriberID),
					zap.Error(writeErr),
				)
				return
			}
			c.Writer.Flush()

			// Broadcast job status with each ping
			if h.jobStatusService != nil {
				status := h.jobStatusService.GetJobStatus()
				statusData, err := json.Marshal(status)
				if err == nil {
					_, writeErr = fmt.Fprintf(c.Writer, "event: jobs:status\ndata: %s\n\n", statusData)
					if writeErr != nil {
						h.logger.Debug("SSE job status write failed, client likely disconnected",
							zap.String("subscriber_id", subscriberID),
							zap.Error(writeErr),
						)
						return
					}
					c.Writer.Flush()
				}
			}
		}
	}
}
