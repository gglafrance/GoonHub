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
	MaxItemsPerPage int
}

func NewSettingsHandler(settingsService *core.SettingsService, maxItemsPerPage int) *SettingsHandler {
	return &SettingsHandler{
		SettingsService: settingsService,
		MaxItemsPerPage: maxItemsPerPage,
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

	settings.MaxItemsPerPage = h.MaxItemsPerPage
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

func convertHomepageSections(req []request.HomepageSectionRequest) []data.HomepageSection {
	sections := make([]data.HomepageSection, len(req))
	for i, s := range req {
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
	return sections
}

func (h *SettingsHandler) convertRequestToConfig(req request.UpdateHomepageConfigRequest) data.HomepageConfig {
	tabs := make([]data.HomepageTab, len(req.Tabs))
	for i, t := range req.Tabs {
		tabs[i] = data.HomepageTab{
			ID:       t.ID,
			Name:     t.Name,
			Icon:     t.Icon,
			Sections: convertHomepageSections(t.Sections),
		}
	}
	return data.HomepageConfig{
		ShowUpload:   req.ShowUpload,
		Sections:     convertHomepageSections(req.Sections),
		Tabs:         tabs,
		DefaultTabID: req.DefaultTabID,
		OpenOn:       req.OpenOn,
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

func (h *SettingsHandler) UpdateAllSettings(c *gin.Context) {
	userPayload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req request.UpdateAllSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Cap videos per page to admin-configured maximum
	if req.VideosPerPage > h.MaxItemsPerPage {
		req.VideosPerPage = h.MaxItemsPerPage
	}

	homepageConfig := h.convertRequestToConfig(req.HomepageConfig)
	parsingRules := h.convertRequestToParsingRules(req.ParsingRules)
	sortPrefs := convertSortPreferences(req.SortPreferences)
	sceneCardConfig := h.convertRequestToSceneCardConfig(req.SceneCardConfig)
	var sceneListView *data.SceneListView
	if req.SceneListView != nil {
		sceneListView = &data.SceneListView{Pages: req.SceneListView.Pages, Columns: req.SceneListView.Columns}
	}

	settings, err := h.SettingsService.UpdateAllSettings(
		userPayload.UserID,
		req.Autoplay,
		req.DefaultVolume,
		req.Loop,
		req.AbLoopControls,
		req.MarkerLoop,
		req.VideosPerPage,
		req.DefaultSortOrder,
		req.DefaultTagSort,
		req.MarkerThumbnailCycling,
		homepageConfig,
		parsingRules,
		sortPrefs,
		req.PlaylistAutoAdvance,
		req.PlaylistCountdownSeconds,
		req.ShowPageSizeSelector,
		req.AdvancedMarkerFilters,
		req.DefaultMarkerView,
		sceneCardConfig,
		core.WithPaginationPosition(req.PaginationPosition),
		core.WithDateFormat(req.DateFormat),
		core.WithSceneListView(sceneListView),
		core.WithGridDensity(req.GridDensity),
		core.WithInfiniteScroll(req.InfiniteScroll),
		core.WithPlayerGestures(req.PlayerGestures, req.GestureSeekStep),
		core.WithReplayHeatmap(req.ShowReplayHeatmap, req.RecordReplayHeatmap),
		core.WithScrubber(req.ShowScrubber),
		core.WithScrubberMode(req.ScrubberMode),
		core.WithContentWidth(req.ContentWidth),
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	settings.MaxItemsPerPage = h.MaxItemsPerPage
	c.JSON(http.StatusOK, settings)
}

// PatchSettings updates only the settings present in the body (PATCH /settings) and returns
// all the settings, like UpdateAllSettings.
func (h *SettingsHandler) PatchSettings(c *gin.Context) {
	userPayload, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	req, err := request.DecodePatchSettingsRequest(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	settings, err := h.SettingsService.UpdateSettings(userPayload.UserID, h.patchOptions(req)...)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	settings.MaxItemsPerPage = h.MaxItemsPerPage
	c.JSON(http.StatusOK, settings)
}

// patchOptions maps the fields present in a PATCH body to the settings options, which validate
// them with the same rules as PUT.
func (h *SettingsHandler) patchOptions(req *request.PatchSettingsRequest) []core.SettingsOption {
	var opts []core.SettingsOption
	addBool := func(v *bool, with func(bool) core.SettingsOption) {
		if v != nil {
			opts = append(opts, with(*v))
		}
	}
	addString := func(v *string, with func(string) core.SettingsOption) {
		if v != nil {
			opts = append(opts, with(*v))
		}
	}
	addInt := func(v *int, with func(int) core.SettingsOption) {
		if v != nil {
			opts = append(opts, with(*v))
		}
	}

	addBool(req.Autoplay, core.WithAutoplay)
	addInt(req.DefaultVolume, core.WithDefaultVolume)
	addBool(req.Loop, core.WithLoop)
	addBool(req.AbLoopControls, core.WithAbLoopControls)
	addBool(req.MarkerLoop, core.WithMarkerLoop)
	if req.VideosPerPage != nil {
		// Cap videos per page to admin-configured maximum
		opts = append(opts, core.WithVideosPerPage(min(*req.VideosPerPage, h.MaxItemsPerPage)))
	}
	addString(req.DefaultSortOrder, core.WithDefaultSortOrder)
	addString(req.DefaultTagSort, core.WithDefaultTagSort)
	addBool(req.MarkerThumbnailCycling, core.WithMarkerThumbnailCycling)
	if req.HomepageConfig != nil {
		opts = append(opts, h.SettingsService.WithHomepageConfig(h.convertRequestToConfig(*req.HomepageConfig)))
	}
	if req.ParsingRules != nil {
		opts = append(opts, h.SettingsService.WithParsingRules(h.convertRequestToParsingRules(*req.ParsingRules)))
	}
	if req.SortPreferences != nil {
		opts = append(opts, core.WithSortPreferences(convertSortPreferences(*req.SortPreferences)))
	}
	addString(req.PlaylistAutoAdvance, core.WithPlaylistAutoAdvance)
	addInt(req.PlaylistCountdownSeconds, core.WithPlaylistCountdownSeconds)
	addBool(req.ShowPageSizeSelector, core.WithShowPageSizeSelector)
	addBool(req.AdvancedMarkerFilters, core.WithAdvancedMarkerFilters)
	addString(req.DefaultMarkerView, core.WithDefaultMarkerView)
	addString(req.PaginationPosition, core.WithPaginationPosition)
	addString(req.DateFormat, core.WithDateFormat)
	if req.PlayerGestures != nil {
		opts = append(opts, core.WithPlayerGestures(req.PlayerGestures, 0))
	}
	addInt(req.GestureSeekStep, core.WithGestureSeekStep)
	if req.ShowReplayHeatmap != nil || req.RecordReplayHeatmap != nil {
		opts = append(opts, core.WithReplayHeatmap(req.ShowReplayHeatmap, req.RecordReplayHeatmap))
	}
	if req.SceneCardConfig != nil {
		opts = append(opts, h.SettingsService.WithSceneCardConfig(h.convertRequestToSceneCardConfig(*req.SceneCardConfig)))
	}
	addString(req.GridDensity, core.WithGridDensity)
	if req.InfiniteScroll != nil {
		opts = append(opts, core.WithInfiniteScroll(req.InfiniteScroll))
	}
	if req.SceneListView != nil {
		opts = append(opts, core.WithSceneListView(&data.SceneListView{Pages: req.SceneListView.Pages, Columns: req.SceneListView.Columns}))
	}
	if req.ShowScrubber != nil {
		opts = append(opts, core.WithScrubber(req.ShowScrubber))
	}
	if req.ScrubberMode != nil {
		opts = append(opts, core.WithScrubberMode(req.ScrubberMode))
	}
	if req.ContentWidth != nil {
		opts = append(opts, core.WithContentWidth(req.ContentWidth))
	}
	return opts
}

func convertSortPreferences(req request.UpdateSortPreferencesRequest) data.SortPreferences {
	return data.SortPreferences{
		Actors:       req.Actors,
		Studios:      req.Studios,
		Markers:      req.Markers,
		ActorScenes:  req.ActorScenes,
		StudioScenes: req.StudioScenes,
	}
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

func (h *SettingsHandler) convertRequestToSceneCardConfig(req request.UpdateSceneCardConfigRequest) data.SceneCardConfig {
	convertZone := func(z request.BadgeZoneRequest) data.BadgeZone {
		items := z.Items
		if items == nil {
			items = []string{}
		}
		direction := z.Direction
		if direction == "" {
			direction = "vertical"
		}
		return data.BadgeZone{Items: items, Direction: direction, Columns: z.Columns}
	}

	rows := make([]data.ContentRow, len(req.ContentRows))
	for i, r := range req.ContentRows {
		rows[i] = data.ContentRow{
			Type:      r.Type,
			Field:     r.Field,
			Mode:      r.Mode,
			Left:      r.Left,
			Right:     r.Right,
			LeftMode:  r.LeftMode,
			RightMode: r.RightMode,
		}
	}
	if rows == nil {
		rows = []data.ContentRow{}
	}

	return data.SceneCardConfig{
		Badges: data.BadgeZones{
			TopLeft:     convertZone(req.Badges.TopLeft),
			TopRight:    convertZone(req.Badges.TopRight),
			BottomLeft:  convertZone(req.Badges.BottomLeft),
			BottomRight: convertZone(req.Badges.BottomRight),
		},
		ContentRows: rows,
	}
}
