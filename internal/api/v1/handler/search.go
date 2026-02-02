package handler

import (
	"goonhub/internal/api/v1/response"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"net/http"

	"github.com/gin-gonic/gin"
)

type SearchHandler struct {
	searchService    *core.SearchService
	searchConfigRepo data.SearchConfigRepository
}

func NewSearchHandler(searchService *core.SearchService, searchConfigRepo data.SearchConfigRepository) *SearchHandler {
	return &SearchHandler{
		searchService:    searchService,
		searchConfigRepo: searchConfigRepo,
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

// GetSearchConfig returns the current search configuration.
// GET /admin/search/config
func (h *SearchHandler) GetSearchConfig(c *gin.Context) {
	record, err := h.searchConfigRepo.Get()
	if err != nil {
		response.InternalError(c, "failed to get search config: "+err.Error())
		return
	}

	if record == nil {
		response.OK(c, gin.H{
			"max_total_hits": 100000,
		})
		return
	}

	response.OK(c, gin.H{
		"max_total_hits": record.MaxTotalHits,
	})
}

type updateSearchConfigRequest struct {
	MaxTotalHits int64 `json:"max_total_hits" binding:"required"`
}

// UpdateSearchConfig updates the search configuration.
// PUT /admin/search/config
func (h *SearchHandler) UpdateSearchConfig(c *gin.Context) {
	var req updateSearchConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	if req.MaxTotalHits < 1000 {
		response.BadRequest(c, "max_total_hits must be at least 1000")
		return
	}

	// Persist to database
	record := &data.SearchConfigRecord{
		MaxTotalHits: req.MaxTotalHits,
	}
	if err := h.searchConfigRepo.Upsert(record); err != nil {
		response.InternalError(c, "failed to persist search config: "+err.Error())
		return
	}

	// Apply to Meilisearch
	if err := h.searchService.UpdateMaxTotalHits(req.MaxTotalHits); err != nil {
		response.InternalError(c, "search config saved but failed to apply to Meilisearch: "+err.Error())
		return
	}

	response.OK(c, gin.H{
		"max_total_hits": req.MaxTotalHits,
	})
}
