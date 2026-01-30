package handler

import (
	"goonhub/internal/api/middleware"
	"goonhub/internal/api/v1/request"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"net/http"

	"github.com/gin-gonic/gin"
)

type StudioInteractionHandler struct {
	Service    *core.StudioInteractionService
	StudioRepo data.StudioRepository
}

func NewStudioInteractionHandler(service *core.StudioInteractionService, studioRepo data.StudioRepository) *StudioInteractionHandler {
	return &StudioInteractionHandler{
		Service:    service,
		StudioRepo: studioRepo,
	}
}

func (h *StudioInteractionHandler) getStudioIDFromUUID(c *gin.Context) (uint, bool) {
	uuid := c.Param("uuid")
	studio, err := h.StudioRepo.GetByUUID(uuid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Studio not found"})
		return 0, false
	}
	return studio.ID, true
}

func (h *StudioInteractionHandler) GetInteractions(c *gin.Context) {
	payload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	studioID, ok := h.getStudioIDFromUUID(c)
	if !ok {
		return
	}

	interactions, err := h.Service.GetAllInteractions(payload.UserID, studioID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get interactions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"rating": interactions.Rating,
		"liked":  interactions.Liked,
	})
}

func (h *StudioInteractionHandler) SetRating(c *gin.Context) {
	payload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	studioID, ok := h.getStudioIDFromUUID(c)
	if !ok {
		return
	}

	var req request.SetRatingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid rating value"})
		return
	}

	if err := h.Service.SetRating(payload.UserID, studioID, req.Rating); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"rating": req.Rating})
}

func (h *StudioInteractionHandler) DeleteRating(c *gin.Context) {
	payload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	studioID, ok := h.getStudioIDFromUUID(c)
	if !ok {
		return
	}

	if err := h.Service.ClearRating(payload.UserID, studioID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete rating"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *StudioInteractionHandler) ToggleLike(c *gin.Context) {
	payload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	studioID, ok := h.getStudioIDFromUUID(c)
	if !ok {
		return
	}

	liked, err := h.Service.ToggleLike(payload.UserID, studioID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to toggle like"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"liked": liked})
}
