package handler

import (
	"goonhub/internal/api/middleware"
	"goonhub/internal/api/v1/request"
	"goonhub/internal/core"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type InteractionHandler struct {
	Service *core.InteractionService
}

func NewInteractionHandler(service *core.InteractionService) *InteractionHandler {
	return &InteractionHandler{Service: service}
}

func (h *InteractionHandler) GetRating(c *gin.Context) {
	payload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	sceneID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scene ID"})
		return
	}

	rating, err := h.Service.GetRating(payload.UserID, uint(sceneID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get rating"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"rating": rating})
}

func (h *InteractionHandler) SetRating(c *gin.Context) {
	payload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	sceneID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scene ID"})
		return
	}

	var req request.SetRatingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid rating value"})
		return
	}

	if err := h.Service.SetRating(payload.UserID, uint(sceneID), req.Rating); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"rating": req.Rating})
}

func (h *InteractionHandler) DeleteRating(c *gin.Context) {
	payload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	sceneID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scene ID"})
		return
	}

	if err := h.Service.ClearRating(payload.UserID, uint(sceneID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete rating"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *InteractionHandler) GetLike(c *gin.Context) {
	payload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	sceneID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scene ID"})
		return
	}

	liked, err := h.Service.IsLiked(payload.UserID, uint(sceneID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get like status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"liked": liked})
}

func (h *InteractionHandler) ToggleLike(c *gin.Context) {
	payload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	sceneID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scene ID"})
		return
	}

	liked, err := h.Service.ToggleLike(payload.UserID, uint(sceneID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to toggle like"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"liked": liked})
}

func (h *InteractionHandler) GetJizzed(c *gin.Context) {
	payload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	sceneID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scene ID"})
		return
	}

	count, err := h.Service.GetJizzedCount(payload.UserID, uint(sceneID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get jizzed count"})
		return
	}

	h.respondJizzed(c, payload.UserID, uint(sceneID), count)
}

// respondJizzed returns the jizz count with the dated o history (most recent first).
func (h *InteractionHandler) respondJizzed(c *gin.Context, userID, sceneID uint, count int) {
	history, err := h.Service.GetJizzHistory(userID, sceneID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get jizz history"})
		return
	}
	events := make([]gin.H, len(history.Events))
	var last *time.Time
	for i, at := range history.Events {
		events[i] = gin.H{"jizzed_at": at.UTC().Format(time.RFC3339)}
		if i == 0 {
			t := at
			last = &t
		}
	}
	resp := gin.H{"count": count, "history": events, "history_total": history.Total, "last_jizzed_at": nil}
	if last != nil {
		resp["last_jizzed_at"] = last.UTC().Format(time.RFC3339)
	}
	c.JSON(http.StatusOK, resp)
}

func (h *InteractionHandler) ToggleJizzed(c *gin.Context) {
	payload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	sceneID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scene ID"})
		return
	}

	count, err := h.Service.IncrementJizzed(payload.UserID, uint(sceneID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to increment jizzed"})
		return
	}

	h.respondJizzed(c, payload.UserID, uint(sceneID), count)
}

func (h *InteractionHandler) GetInteractions(c *gin.Context) {
	payload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	sceneID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scene ID"})
		return
	}

	interactions, err := h.Service.GetAllInteractions(payload.UserID, uint(sceneID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get interactions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"rating":       interactions.Rating,
		"liked":        interactions.Liked,
		"jizzed_count": interactions.JizzedCount,
	})
}
