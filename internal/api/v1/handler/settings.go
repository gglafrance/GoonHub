package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"goonhub/internal/api/middleware"
	"goonhub/internal/api/v1/request"
	"goonhub/internal/core"
	"goonhub/internal/data"
)

type SettingsHandler struct {
	SettingsService *core.SettingsService
}

func NewSettingsHandler(settingsService *core.SettingsService) *SettingsHandler {
	return &SettingsHandler{
		SettingsService: settingsService,
	}
}

func (h *SettingsHandler) GetSettings(c *gin.Context) {
	userPayload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	settings, err := h.SettingsService.GetSettings(userPayload.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch settings"})
		return
	}

	c.JSON(http.StatusOK, settings)
}

func (h *SettingsHandler) UpdatePlayerSettings(c *gin.Context) {
	userPayload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req request.UpdatePlayerSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	settings, err := h.SettingsService.UpdatePlayerSettings(userPayload.UserID, req.Autoplay, req.DefaultVolume, req.Loop)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, settings)
}

func (h *SettingsHandler) UpdateAppSettings(c *gin.Context) {
	userPayload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req request.UpdateAppSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	settings, err := h.SettingsService.UpdateAppSettings(userPayload.UserID, req.VideosPerPage, req.DefaultSortOrder, req.MarkerThumbnailCycling)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, settings)
}

func (h *SettingsHandler) UpdateTagSettings(c *gin.Context) {
	userPayload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req request.UpdateTagSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	settings, err := h.SettingsService.UpdateTagSettings(userPayload.UserID, req.DefaultTagSort)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, settings)
}

func (h *SettingsHandler) ChangePassword(c *gin.Context) {
	userPayload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req request.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := h.SettingsService.ChangePassword(userPayload.UserID, req.CurrentPassword, req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}

func (h *SettingsHandler) ChangeUsername(c *gin.Context) {
	userPayload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req request.ChangeUsernameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := h.SettingsService.ChangeUsername(userPayload.UserID, req.Username); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Username changed successfully"})
}

func (h *SettingsHandler) GetHomepageConfig(c *gin.Context) {
	userPayload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	config, err := h.SettingsService.GetHomepageConfig(userPayload.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch homepage config"})
		return
	}

	c.JSON(http.StatusOK, config)
}

func (h *SettingsHandler) UpdateHomepageConfig(c *gin.Context) {
	userPayload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req request.UpdateHomepageConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Convert request to data model
	config := h.convertRequestToConfig(req)

	settings, err := h.SettingsService.UpdateHomepageConfig(userPayload.UserID, config)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, settings.HomepageConfig)
}

func (h *SettingsHandler) convertRequestToConfig(req request.UpdateHomepageConfigRequest) data.HomepageConfig {
	sections := make([]data.HomepageSection, len(req.Sections))
	for i, s := range req.Sections {
		sections[i] = data.HomepageSection{
			ID:      s.ID,
			Type:    s.Type,
			Title:   s.Title,
			Enabled: s.Enabled,
			Limit:   s.Limit,
			Order:   s.Order,
			Sort:    s.Sort,
			Config:  s.Config,
		}
	}
	return data.HomepageConfig{
		ShowUpload: req.ShowUpload,
		Sections:   sections,
	}
}

func (h *SettingsHandler) GetParsingRules(c *gin.Context) {
	userPayload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	parsingRules, err := h.SettingsService.GetParsingRules(userPayload.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch parsing rules"})
		return
	}

	c.JSON(http.StatusOK, parsingRules)
}

func (h *SettingsHandler) UpdateParsingRules(c *gin.Context) {
	userPayload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req request.UpdateParsingRulesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Convert request to data model
	parsingRules := h.convertRequestToParsingRules(req)

	settings, err := h.SettingsService.UpdateParsingRules(userPayload.UserID, parsingRules)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, settings.ParsingRules)
}

func (h *SettingsHandler) convertRequestToParsingRules(req request.UpdateParsingRulesRequest) data.ParsingRulesSettings {
	presets := make([]data.ParsingPreset, len(req.Presets))
	for i, p := range req.Presets {
		rules := make([]data.ParsingRule, len(p.Rules))
		for j, r := range p.Rules {
			rules[j] = data.ParsingRule{
				ID:      r.ID,
				Type:    r.Type,
				Enabled: r.Enabled,
				Order:   r.Order,
				Config: data.ParsingRuleConfig{
					KeepContent:   r.Config.KeepContent,
					Pattern:       r.Config.Pattern,
					Find:          r.Config.Find,
					Replace:       r.Config.Replace,
					CaseSensitive: r.Config.CaseSensitive,
					MinLength:     r.Config.MinLength,
					CaseType:      r.Config.CaseType,
				},
			}
		}
		presets[i] = data.ParsingPreset{
			ID:        p.ID,
			Name:      p.Name,
			IsBuiltIn: p.IsBuiltIn,
			Rules:     rules,
		}
	}
	return data.ParsingRulesSettings{
		Presets:        presets,
		ActivePresetID: req.ActivePresetID,
	}
}
