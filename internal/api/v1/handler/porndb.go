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

// GetPerformer returns detailed information about a performer from the /performers endpoint
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

// GetPerformerSite returns detailed information about a performer from the /performer-sites endpoint
// This is needed because IDs from performer-sites search cannot be used with the /performers endpoint
func (h *PornDBHandler) GetPerformerSite(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Performer site ID is required"})
		return
	}

	if !h.Service.IsConfigured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "PornDB integration is not configured"})
		return
	}

	performer, err := h.Service.GetPerformerSiteDetails(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": performer,
	})
}

// SearchScenes searches for scenes with optional filters
func (h *PornDBHandler) SearchScenes(c *gin.Context) {
	opts := core.SceneSearchOptions{
		Title: c.Query("title"),
	}

	// Require at least one search parameter
	if opts.IsEmpty() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one search parameter is required"})
		return
	}

	if !h.Service.IsConfigured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "PornDB integration is not configured"})
		return
	}

	scenes, err := h.Service.SearchScenes(opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": scenes,
	})
}

// GetScene returns detailed information about a scene
func (h *PornDBHandler) GetScene(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Scene ID is required"})
		return
	}

	if !h.Service.IsConfigured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "PornDB integration is not configured"})
		return
	}

	scene, err := h.Service.GetSceneDetails(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": scene,
	})
}

// SearchSites searches for sites/studios by name
func (h *PornDBHandler) SearchSites(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query parameter 'q' is required"})
		return
	}

	if !h.Service.IsConfigured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "PornDB integration is not configured"})
		return
	}

	sites, err := h.Service.SearchSites(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": sites,
	})
}

// GetSite returns detailed information about a site/studio
func (h *PornDBHandler) GetSite(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Site ID is required"})
		return
	}

	if !h.Service.IsConfigured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "PornDB integration is not configured"})
		return
	}

	site, err := h.Service.GetSiteDetails(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": site,
	})
}
