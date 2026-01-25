package handler

import (
	"fmt"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
)

type JobHandler struct {
	jobHistoryService    *core.JobHistoryService
	processingService    *core.VideoProcessingService
	poolConfigRepo       data.PoolConfigRepository
	processingConfigRepo data.ProcessingConfigRepository
	triggerConfigRepo    data.TriggerConfigRepository
	triggerScheduler     *core.TriggerScheduler
}

func NewJobHandler(jobHistoryService *core.JobHistoryService, processingService *core.VideoProcessingService, poolConfigRepo data.PoolConfigRepository, processingConfigRepo data.ProcessingConfigRepository, triggerConfigRepo data.TriggerConfigRepository, triggerScheduler *core.TriggerScheduler) *JobHandler {
	return &JobHandler{
		jobHistoryService:    jobHistoryService,
		processingService:    processingService,
		poolConfigRepo:       poolConfigRepo,
		processingConfigRepo: processingConfigRepo,
		triggerConfigRepo:    triggerConfigRepo,
		triggerScheduler:     triggerScheduler,
	}
}

func (h *JobHandler) ListJobs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	jobs, total, err := h.jobHistoryService.ListJobs(page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list jobs"})
		return
	}

	activeJobs, err := h.jobHistoryService.ListActiveJobs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list active jobs"})
		return
	}

	poolConfig := h.processingService.GetPoolConfig()
	queueStatus := h.processingService.GetQueueStatus()

	// Count active (submitted) jobs per phase from DB
	metadataActive := 0
	thumbnailActive := 0
	spritesActive := 0
	for _, aj := range activeJobs {
		switch aj.Phase {
		case "metadata":
			metadataActive++
		case "thumbnail":
			thumbnailActive++
		case "sprites":
			spritesActive++
		}
	}

	// True running = active in DB minus those still in the queue buffer
	metadataRunning := metadataActive - queueStatus.MetadataQueued
	thumbnailRunning := thumbnailActive - queueStatus.ThumbnailQueued
	spritesRunning := spritesActive - queueStatus.SpritesQueued
	if metadataRunning < 0 {
		metadataRunning = 0
	}
	if thumbnailRunning < 0 {
		thumbnailRunning = 0
	}
	if spritesRunning < 0 {
		spritesRunning = 0
	}

	c.JSON(http.StatusOK, gin.H{
		"data":         jobs,
		"total":        total,
		"page":         page,
		"limit":        limit,
		"active_count": len(activeJobs),
		"active_jobs":  activeJobs,
		"retention":    h.jobHistoryService.GetRetention(),
		"pool_config":  poolConfig,
		"queue_status": gin.H{
			"metadata_queued":  queueStatus.MetadataQueued,
			"thumbnail_queued": queueStatus.ThumbnailQueued,
			"sprites_queued":   queueStatus.SpritesQueued,
			"metadata_running":  metadataRunning,
			"thumbnail_running": thumbnailRunning,
			"sprites_running":   spritesRunning,
		},
	})
}

func (h *JobHandler) GetPoolConfig(c *gin.Context) {
	poolConfig := h.processingService.GetPoolConfig()
	c.JSON(http.StatusOK, poolConfig)
}

func (h *JobHandler) UpdatePoolConfig(c *gin.Context) {
	var req core.PoolConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if req.MetadataWorkers < 1 || req.MetadataWorkers > 10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "metadata_workers must be between 1 and 10"})
		return
	}
	if req.ThumbnailWorkers < 1 || req.ThumbnailWorkers > 10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "thumbnail_workers must be between 1 and 10"})
		return
	}
	if req.SpritesWorkers < 1 || req.SpritesWorkers > 10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sprites_workers must be between 1 and 10"})
		return
	}

	if err := h.processingService.UpdatePoolConfig(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update pool config: " + err.Error()})
		return
	}

	record := &data.PoolConfigRecord{
		MetadataWorkers:  req.MetadataWorkers,
		ThumbnailWorkers: req.ThumbnailWorkers,
		SpritesWorkers:   req.SpritesWorkers,
	}
	if err := h.poolConfigRepo.Upsert(record); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Pool config applied but failed to persist: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, h.processingService.GetPoolConfig())
}

func (h *JobHandler) GetProcessingConfig(c *gin.Context) {
	cfg := h.processingService.GetProcessingQualityConfig()
	c.JSON(http.StatusOK, cfg)
}

func (h *JobHandler) UpdateProcessingConfig(c *gin.Context) {
	var req core.ProcessingQualityConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := h.processingService.UpdateProcessingQualityConfig(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	record := &data.ProcessingConfigRecord{
		MaxFrameDimensionSm: req.MaxFrameDimensionSm,
		MaxFrameDimensionLg: req.MaxFrameDimensionLg,
		FrameQualitySm:      req.FrameQualitySm,
		FrameQualityLg:      req.FrameQualityLg,
		FrameQualitySprites: req.FrameQualitySprites,
		SpritesConcurrency:  req.SpritesConcurrency,
	}
	if err := h.processingConfigRepo.Upsert(record); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Processing config applied but failed to persist: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, h.processingService.GetProcessingQualityConfig())
}

func (h *JobHandler) GetTriggerConfig(c *gin.Context) {
	configs, err := h.triggerConfigRepo.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get trigger config"})
		return
	}
	c.JSON(http.StatusOK, configs)
}

var validPhases = map[string]bool{"metadata": true, "thumbnail": true, "sprites": true, "scan": true}
var validProcessingPhases = map[string]bool{"metadata": true, "thumbnail": true, "sprites": true}
var validTriggerTypes = map[string]bool{"on_import": true, "after_job": true, "manual": true, "scheduled": true}
var validScanTriggerTypes = map[string]bool{"manual": true, "scheduled": true}

func (h *JobHandler) UpdateTriggerConfig(c *gin.Context) {
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

	if !validPhases[req.Phase] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "phase must be one of: metadata, thumbnail, sprites, scan"})
		return
	}

	// Scan phase only supports manual and scheduled triggers
	if req.Phase == "scan" {
		if !validScanTriggerTypes[req.TriggerType] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "scan phase only supports manual or scheduled triggers"})
			return
		}
	} else {
		if !validTriggerTypes[req.TriggerType] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "trigger_type must be one of: on_import, after_job, manual, scheduled"})
			return
		}
	}

	// Only metadata can be on_import
	if req.TriggerType == "on_import" && req.Phase != "metadata" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only metadata phase can use on_import trigger"})
		return
	}

	// after_job requires valid after_phase (only for processing phases, not scan)
	if req.TriggerType == "after_job" {
		if req.AfterPhase == nil || *req.AfterPhase == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "after_phase is required when trigger_type is after_job"})
			return
		}
		if !validProcessingPhases[*req.AfterPhase] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "after_phase must be one of: metadata, thumbnail, sprites"})
			return
		}
		if *req.AfterPhase == req.Phase {
			c.JSON(http.StatusBadRequest, gin.H{"error": "after_phase cannot be the same as phase"})
			return
		}

		// Circular dependency detection
		if err := h.detectCycle(req.Phase, *req.AfterPhase); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	// scheduled requires valid cron expression
	if req.TriggerType == "scheduled" {
		if req.CronExpression == nil || *req.CronExpression == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cron_expression is required when trigger_type is scheduled"})
			return
		}
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		if _, err := parser.Parse(*req.CronExpression); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid cron expression: %s", err.Error())})
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

func (h *JobHandler) detectCycle(phase string, afterPhase string) error {
	configs, err := h.triggerConfigRepo.GetAll()
	if err != nil {
		return fmt.Errorf("failed to check dependencies")
	}

	// Build adjacency: phase -> after_phase (what this phase depends on)
	dependsOn := make(map[string]string)
	for _, cfg := range configs {
		if cfg.TriggerType == "after_job" && cfg.AfterPhase != nil {
			dependsOn[cfg.Phase] = *cfg.AfterPhase
		}
	}

	// Apply the proposed change
	dependsOn[phase] = afterPhase

	// Walk from phase following the chain to detect a cycle
	visited := make(map[string]bool)
	current := phase
	for {
		if visited[current] {
			return fmt.Errorf("circular dependency detected: %s would create a cycle", phase)
		}
		visited[current] = true
		next, exists := dependsOn[current]
		if !exists {
			break
		}
		current = next
	}
	return nil
}

func (h *JobHandler) TriggerPhase(c *gin.Context) {
	idStr := c.Param("id")
	videoID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"})
		return
	}

	phase := c.Param("phase")
	if !validPhases[phase] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "phase must be one of: metadata, thumbnail, sprites"})
		return
	}

	if err := h.processingService.SubmitPhase(uint(videoID), phase); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Phase %s triggered for video %d", phase, videoID)})
}

var validModes = map[string]bool{"missing": true, "all": true}

func (h *JobHandler) TriggerBulkPhase(c *gin.Context) {
	var req struct {
		Phase string `json:"phase"`
		Mode  string `json:"mode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if !validPhases[req.Phase] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "phase must be one of: metadata, thumbnail, sprites"})
		return
	}

	if !validModes[req.Mode] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mode must be one of: missing, all"})
		return
	}

	result, err := h.processingService.SubmitBulkPhase(req.Phase, req.Mode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   fmt.Sprintf("Bulk %s phase triggered (%s mode)", req.Phase, req.Mode),
		"submitted": result.Submitted,
		"skipped":   result.Skipped,
		"errors":    result.Errors,
	})
}
