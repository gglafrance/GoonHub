package handler

import (
	"goonhub/internal/core"
	"net/http"

	"github.com/gin-gonic/gin"
)

type SearchHandler struct {
	searchService *core.SearchService
}

func NewSearchHandler(searchService *core.SearchService) *SearchHandler {
	return &SearchHandler{
		searchService: searchService,
	}
}

// ReindexAll triggers a full reindex of all videos in Meilisearch.
// POST /admin/search/reindex
func (h *SearchHandler) ReindexAll(c *gin.Context) {
	if !h.searchService.IsAvailable() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Meilisearch is not available"})
		return
	}

	// Run reindex in background
	go func() {
		if err := h.searchService.ReindexAll(); err != nil {
			// Error is logged in ReindexAll
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{"message": "Reindex started"})
}

// GetStatus returns the status of the search service.
// GET /admin/search/status
func (h *SearchHandler) GetStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"available": h.searchService.IsAvailable(),
	})
}
