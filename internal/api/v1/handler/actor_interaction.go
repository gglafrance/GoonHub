package handler

import (
	"goonhub/internal/api/middleware"
	"goonhub/internal/api/v1/request"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ActorInteractionHandler struct {
	Service   *core.ActorInteractionService
	ActorRepo data.ActorRepository
}

func NewActorInteractionHandler(service *core.ActorInteractionService, actorRepo data.ActorRepository) *ActorInteractionHandler {
	return &ActorInteractionHandler{
		Service:   service,
		ActorRepo: actorRepo,
	}
}

func (h *ActorInteractionHandler) getActorIDFromUUID(c *gin.Context) (uint, bool) {
	uuid := c.Param("uuid")
	actor, err := h.ActorRepo.GetByUUID(uuid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Actor not found"})
		return 0, false
	}
	return actor.ID, true
}

func (h *ActorInteractionHandler) GetInteractions(c *gin.Context) {
	payload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	actorID, ok := h.getActorIDFromUUID(c)
	if !ok {
		return
	}

	interactions, err := h.Service.GetAllInteractions(payload.UserID, actorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get interactions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"rating": interactions.Rating,
		"liked":  interactions.Liked,
	})
}

func (h *ActorInteractionHandler) SetRating(c *gin.Context) {
	payload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	actorID, ok := h.getActorIDFromUUID(c)
	if !ok {
		return
	}

	var req request.SetRatingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid rating value"})
		return
	}

	if err := h.Service.SetRating(payload.UserID, actorID, req.Rating); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"rating": req.Rating})
}

func (h *ActorInteractionHandler) DeleteRating(c *gin.Context) {
	payload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	actorID, ok := h.getActorIDFromUUID(c)
	if !ok {
		return
	}

	if err := h.Service.ClearRating(payload.UserID, actorID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete rating"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *ActorInteractionHandler) ToggleLike(c *gin.Context) {
	payload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	actorID, ok := h.getActorIDFromUUID(c)
	if !ok {
		return
	}

	liked, err := h.Service.ToggleLike(payload.UserID, actorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to toggle like"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"liked": liked})
}
