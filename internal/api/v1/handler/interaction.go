package handler

import (
	"goonhub/internal/api/middleware"
	"goonhub/internal/api/v1/request"
	"goonhub/internal/core"
	"net/http"
	"strconv"

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

	c.JSON(http.StatusOK, gin.H{"count": count})
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

	c.JSON(http.StatusOK, gin.H{"count": count})
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
