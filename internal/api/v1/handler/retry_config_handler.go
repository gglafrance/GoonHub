package handler

import (
	"goonhub/internal/api/v1/validators"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"net/http"

	"github.com/gin-gonic/gin"
)

// RetryConfigHandler handles retry configuration requests
type RetryConfigHandler struct {
	retryConfigRepo data.RetryConfigRepository
	retryScheduler  *core.RetryScheduler
}

// NewRetryConfigHandler creates a new RetryConfigHandler
func NewRetryConfigHandler(
	retryConfigRepo data.RetryConfigRepository,
	retryScheduler *core.RetryScheduler,
) *RetryConfigHandler {
	return &RetryConfigHandler{
		retryConfigRepo: retryConfigRepo,
		retryScheduler:  retryScheduler,
	}
}

// GetRetryConfig returns all retry configurations
func (h *RetryConfigHandler) GetRetryConfig(c *gin.Context) {
	if h.retryConfigRepo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Retry config not available"})
		return
	}

	configs, err := h.retryConfigRepo.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get retry config"})
		return
	}

	c.JSON(http.StatusOK, configs)
}

// UpdateRetryConfig updates a retry configuration
func (h *RetryConfigHandler) UpdateRetryConfig(c *gin.Context) {
	if h.retryConfigRepo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Retry config not available"})
		return
	}

	var req struct {
		Phase               string  `json:"phase"`
		MaxRetries          int     `json:"max_retries"`
		InitialDelaySeconds int     `json:"initial_delay_seconds"`
		MaxDelaySeconds     int     `json:"max_delay_seconds"`
		BackoffFactor       float64 `json:"backoff_factor"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate retry configuration
	if err := validators.ValidateRetryConfig(validators.RetryConfigInput{
		Phase:               req.Phase,
		MaxRetries:          req.MaxRetries,
		InitialDelaySeconds: req.InitialDelaySeconds,
		MaxDelaySeconds:     req.MaxDelaySeconds,
		BackoffFactor:       req.BackoffFactor,
	}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	record := &data.RetryConfigRecord{
		Phase:               req.Phase,
		MaxRetries:          req.MaxRetries,
		InitialDelaySeconds: req.InitialDelaySeconds,
		MaxDelaySeconds:     req.MaxDelaySeconds,
		BackoffFactor:       req.BackoffFactor,
	}

	if err := h.retryConfigRepo.Upsert(record); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update retry config"})
		return
	}

	// Refresh the retry scheduler's cache
	if h.retryScheduler != nil {
		if err := h.retryScheduler.RefreshConfigCache(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Retry config saved but failed to refresh cache"})
			return
		}
	}

	configs, err := h.retryConfigRepo.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Retry config saved but failed to reload"})
		return
	}
	c.JSON(http.StatusOK, configs)
}
