package handler

import (
	"goonhub/internal/core"
	"net/http"

	"github.com/gin-gonic/gin"
)

type PornDBHandler struct {
	Service *core.PornDBService
}

func NewPornDBHandler(service *core.PornDBService) *PornDBHandler {
	return &PornDBHandler{
		Service: service,
	}
}

// GetStatus returns whether the PornDB integration is configured
func (h *PornDBHandler) GetStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"configured": h.Service.IsConfigured(),
	})
}

// SearchPerformers searches for performers by name
func (h *PornDBHandler) SearchPerformers(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query parameter 'q' is required"})
		return
	}

	if !h.Service.IsConfigured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "PornDB integration is not configured"})
		return
	}

	performers, err := h.Service.SearchPerformers(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": performers,
	})
}

// GetPerformer returns detailed information about a performer
func (h *PornDBHandler) GetPerformer(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Performer ID is required"})
		return
	}

	if !h.Service.IsConfigured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "PornDB integration is not configured"})
		return
	}

	performer, err := h.Service.GetPerformerDetails(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": performer,
	})
}
