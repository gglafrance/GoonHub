package handler

import (
	"goonhub/internal/api/v1/validators"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"net/http"

	"github.com/gin-gonic/gin"
)

// TriggerConfigHandler handles trigger configuration requests
type TriggerConfigHandler struct {
	triggerConfigRepo data.TriggerConfigRepository
	processingService *core.VideoProcessingService
	triggerScheduler  *core.TriggerScheduler
}

// NewTriggerConfigHandler creates a new TriggerConfigHandler
func NewTriggerConfigHandler(
	triggerConfigRepo data.TriggerConfigRepository,
	processingService *core.VideoProcessingService,
	triggerScheduler *core.TriggerScheduler,
) *TriggerConfigHandler {
	return &TriggerConfigHandler{
		triggerConfigRepo: triggerConfigRepo,
		processingService: processingService,
		triggerScheduler:  triggerScheduler,
	}
}

// GetTriggerConfig returns all trigger configurations
func (h *TriggerConfigHandler) GetTriggerConfig(c *gin.Context) {
	configs, err := h.triggerConfigRepo.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get trigger config"})
		return
	}
	c.JSON(http.StatusOK, configs)
}

// UpdateTriggerConfig updates a trigger configuration
func (h *TriggerConfigHandler) UpdateTriggerConfig(c *gin.Context) {
	var req struct {
		Phase          string  `json:"phase"`
		TriggerType    string  `json:"trigger_type"`
		AfterPhase     *string `json:"after_phase"`
		CronExpression *string `json:"cron_expression"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate phase
	if err := validators.ValidatePhase(req.Phase); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate trigger type for phase
	if err := validators.ValidateTriggerType(req.Phase, req.TriggerType); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate on_import is only for metadata
	if err := validators.ValidateOnImportTrigger(req.Phase, req.TriggerType); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate after_job configuration
	if req.TriggerType == "after_job" {
		if err := validators.ValidateAfterJobTrigger(req.Phase, req.AfterPhase); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Check for circular dependencies
		if err := h.checkCycleDependency(req.Phase, *req.AfterPhase); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	// Validate scheduled cron expression
	if req.TriggerType == "scheduled" {
		cronExpr := ""
		if req.CronExpression != nil {
			cronExpr = *req.CronExpression
		}
		if err := validators.ValidateCronExpression(cronExpr); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	record := &data.TriggerConfigRecord{
		Phase:          req.Phase,
		TriggerType:    req.TriggerType,
		AfterPhase:     req.AfterPhase,
		CronExpression: req.CronExpression,
	}

	if err := h.triggerConfigRepo.Upsert(record); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update trigger config"})
		return
	}

	// Refresh caches
	if err := h.processingService.RefreshTriggerCache(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Trigger config saved but failed to refresh cache"})
		return
	}

	if h.triggerScheduler != nil {
		if err := h.triggerScheduler.RefreshSchedules(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Trigger config saved but failed to refresh scheduler"})
			return
		}
	}

	configs, err := h.triggerConfigRepo.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Trigger config saved but failed to reload"})
		return
	}
	c.JSON(http.StatusOK, configs)
}

func (h *TriggerConfigHandler) checkCycleDependency(phase, afterPhase string) error {
	configs, err := h.triggerConfigRepo.GetAll()
	if err != nil {
		return err
	}

	// Convert to validator format
	triggerConfigs := make([]validators.TriggerConfig, len(configs))
	for i, cfg := range configs {
		triggerConfigs[i] = validators.TriggerConfig{
			Phase:       cfg.Phase,
			TriggerType: cfg.TriggerType,
			AfterPhase:  cfg.AfterPhase,
		}
	}

	return validators.DetectTriggerCycle(triggerConfigs, phase, afterPhase)
}
