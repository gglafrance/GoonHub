package core

import (
	"fmt"
	"goonhub/internal/data"
	"slices"
	"strings"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

var allowedTagSorts = map[string]bool{
	"az":    true,
	"za":    true,
	"most":  true,
	"least": true,
}

var allowedSortOrders = map[string]bool{
	"created_at_desc": true,
	"created_at_asc":  true,
	"title_asc":       true,
	"title_desc":      true,
	"duration_asc":    true,
	"duration_desc":   true,
	"size_asc":        true,
	"size_desc":       true,
	"view_count_desc": true,
	"view_count_asc":  true,
	"last_jizz_desc":  true,
	"last_jizz_asc":   true,
	"jizz_count_desc": true,
	"jizz_count_asc":  true,
	"random":          true,
}

var allowedActorSorts = map[string]bool{
	"name_asc":         true,
	"name_desc":        true,
	"scene_count_desc": true,
	"scene_count_asc":  true,
	"created_at_desc":  true,
	"created_at_asc":   true,
}

var allowedStudioSorts = map[string]bool{
	"name_asc":         true,
	"name_desc":        true,
	"scene_count_desc": true,
	"scene_count_asc":  true,
	"created_at_desc":  true,
	"created_at_asc":   true,
}

var allowedMarkerSorts = map[string]bool{
	"label_asc":  true,
	"label_desc": true,
	"count_desc": true,
	"count_asc":  true,
	"recent":     true,
	"oldest":     true,
}

var allowedEntitySceneSorts = map[string]bool{
	"":                true,
	"created_at_asc":  true,
	"title_asc":       true,
	"title_desc":      true,
	"duration_asc":    true,
	"duration_desc":   true,
	"size_asc":        true,
	"size_desc":       true,
	"view_count_desc": true,
	"view_count_asc":  true,
	"last_jizz_desc":  true,
	"last_jizz_asc":   true,
	"jizz_count_desc": true,
	"jizz_count_asc":  true,
	"random":          true,
}

var allowedMarkerViews = map[string]bool{
	"grouped": true,
	"all":     true,
}

var allowedPlaylistAutoAdvance = map[string]bool{
	"instant":   true,
	"countdown": true,
	"manual":    true,
}

var allowedSectionTypes = map[string]bool{
	"latest":            true,
	"actor":             true,
	"studio":            true,
	"tag":               true,
	"saved_search":      true,
	"continue_watching": true,
	"most_viewed":       true,
	"liked":             true,
	"playlist":          true,
}

type SettingsService struct {
	settingsRepo data.UserSettingsRepository
	userRepo     data.UserRepository
	logger       *zap.Logger
}

func NewSettingsService(settingsRepo data.UserSettingsRepository, userRepo data.UserRepository, logger *zap.Logger) *SettingsService {
	return &SettingsService{
		settingsRepo: settingsRepo,
		userRepo:     userRepo,
		logger:       logger,
	}
}

func (s *SettingsService) GetSettings(userID uint) (*data.UserSettings, error) {
	settings, err := s.settingsRepo.GetByUserID(userID)
	if err != nil {
		// Return defaults if no row exists
		return &data.UserSettings{
			UserID:                 userID,
			Autoplay:               false,
			DefaultVolume:          100,
			Loop:                   false,
			AbLoopControls:         false,
			MarkerLoop:             false,
			VideosPerPage:          20,
			DefaultSortOrder:       "created_at_desc",
			DefaultTagSort:         "az",
			MarkerThumbnailCycling: true,
			DefaultMarkerView:      "grouped",
			PaginationPosition:     "bottom",
			DateFormat:             DefaultDateFormat,
			PlayerGestures:         true,
			GestureSeekStep:        DefaultGestureSeekStep,
			HomepageConfig:         data.DefaultHomepageConfig(),
			SortPreferences:        data.DefaultSortPreferences(),
			SceneCardConfig:        data.DefaultSceneCardConfig(),
		}, nil
	}
	return settings, nil
}

func (s *SettingsService) GetHomepageConfig(userID uint) (*data.HomepageConfig, error) {
	settings, err := s.settingsRepo.GetByUserID(userID)
	if err != nil {
		config := data.DefaultHomepageConfig()
		return &config, nil
	}
	return &settings.HomepageConfig, nil
}

func (s *SettingsService) UpdateHomepageConfig(userID uint, config data.HomepageConfig) (*data.UserSettings, error) {
	if err := s.validateHomepageConfig(&config); err != nil {
		return nil, err
	}

	settings, err := s.settingsRepo.GetByUserID(userID)
	if err != nil {
		settings = &data.UserSettings{UserID: userID, HomepageConfig: data.DefaultHomepageConfig()}
	}

	settings.HomepageConfig = config

	if err := s.settingsRepo.Upsert(settings); err != nil {
		return nil, fmt.Errorf("failed to update homepage config: %w", err)
	}

	return settings, nil
}

const (
	maxHomepageTabs          = 12
	maxHomepageTabNameLength = 40
	maxHomepageSectionsInTab = 20
)

func (s *SettingsService) validateHomepageConfig(config *data.HomepageConfig) error {
	// Legacy configs (sections without tabs) become a single tab
	config.Normalize()

	if len(config.Tabs) > maxHomepageTabs {
		return fmt.Errorf("maximum of %d tabs allowed", maxHomepageTabs)
	}
	seenTabs := make(map[string]bool)
	seenIDs := make(map[string]bool)
	for t, tab := range config.Tabs {
		if tab.ID == "" {
			return fmt.Errorf("tab %d: id is required", t)
		}
		if seenTabs[tab.ID] {
			return fmt.Errorf("tab %d: duplicate id '%s'", t, tab.ID)
		}
		seenTabs[tab.ID] = true
		if strings.TrimSpace(tab.Name) == "" {
			return fmt.Errorf("tab %d: name is required", t)
		}
		if len(tab.Name) > maxHomepageTabNameLength {
			return fmt.Errorf("tab %d: name must be %d characters or less", t, maxHomepageTabNameLength)
		}
		if len(tab.Sections) > maxHomepageSectionsInTab {
			return fmt.Errorf("maximum of %d sections allowed", maxHomepageSectionsInTab)
		}
		if err := s.validateHomepageSections(tab.Sections, seenIDs); err != nil {
			return fmt.Errorf("tab '%s': %w", tab.Name, err)
		}
	}
	return nil
}

func (s *SettingsService) validateHomepageSections(sections []data.HomepageSection, seenIDs map[string]bool) error {
	for i, section := range sections {
		if section.ID == "" {
			return fmt.Errorf("section %d: id is required", i)
		}
		if seenIDs[section.ID] {
			return fmt.Errorf("section %d: duplicate id '%s'", i, section.ID)
		}
		seenIDs[section.ID] = true

		if !allowedSectionTypes[section.Type] && !IsHomepageWidgetType(section.Type) {
			return fmt.Errorf("section %d: invalid type '%s'", i, section.Type)
		}

		if section.Title == "" {
			return fmt.Errorf("section %d: title is required", i)
		}
		if len(section.Title) > 100 {
			return fmt.Errorf("section %d: title must be 100 characters or less", i)
		}

		if section.Limit < 1 || section.Limit > 50 {
			return fmt.Errorf("section %d: limit must be between 1 and 50", i)
		}

		if section.Sort != "" && !allowedSortOrders[section.Sort] {
			return fmt.Errorf("section %d: invalid sort order '%s'", i, section.Sort)
		}

		// Validate type-specific config
		if err := s.validateSectionConfig(&section); err != nil {
			return fmt.Errorf("section %d: %w", i, err)
		}
		if err := validateHomepageWidgetConfig(&section); err != nil {
			return fmt.Errorf("section %d: %w", i, err)
		}
	}

	return nil
}

func (s *SettingsService) validateSectionConfig(section *data.HomepageSection) error {
	switch section.Type {
	case "actor":
		if _, ok := section.Config["actor_uuid"]; !ok {
			return fmt.Errorf("actor section requires actor_uuid in config")
		}
	case "studio":
		if _, ok := section.Config["studio_uuid"]; !ok {
			return fmt.Errorf("studio section requires studio_uuid in config")
		}
	case "tag":
		if _, ok := section.Config["tag_id"]; !ok {
			return fmt.Errorf("tag section requires tag_id in config")
		}
	case "saved_search":
		if _, ok := section.Config["saved_search_uuid"]; !ok {
			return fmt.Errorf("saved_search section requires saved_search_uuid in config")
		}
	}
	return nil
}

func (s *SettingsService) ChangePassword(userID uint, currentPassword, newPassword string) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return fmt.Errorf("failed to find user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(currentPassword)); err != nil {
		return fmt.Errorf("current password is incorrect")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	if err := s.userRepo.UpdatePassword(userID, string(hashedPassword)); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	s.logger.Info("Password changed", zap.Uint("user_id", userID))
	return nil
}

func (s *SettingsService) ChangeUsername(userID uint, newUsername string) error {
	if len(newUsername) < 3 {
		return fmt.Errorf("username must be at least 3 characters")
	}

	exists, err := s.userRepo.Exists(newUsername)
	if err != nil {
		return fmt.Errorf("failed to check username availability: %w", err)
	}
	if exists {
		return fmt.Errorf("username already taken")
	}

	if err := s.userRepo.UpdateUsername(userID, newUsername); err != nil {
		return fmt.Errorf("failed to update username: %w", err)
	}

	s.logger.Info("Username changed", zap.Uint("user_id", userID), zap.String("new_username", newUsername))
	return nil
}

func (s *SettingsService) GetParsingRules(userID uint) (*data.ParsingRulesSettings, error) {
	settings, err := s.settingsRepo.GetByUserID(userID)
	if err != nil {
		defaultRules := data.DefaultParsingRulesSettings()
		return &defaultRules, nil
	}
	return &settings.ParsingRules, nil
}

func (s *SettingsService) UpdateParsingRules(userID uint, rules data.ParsingRulesSettings) (*data.UserSettings, error) {
	if err := s.validateParsingRules(&rules); err != nil {
		return nil, err
	}

	settings, err := s.settingsRepo.GetByUserID(userID)
	if err != nil {
		settings = &data.UserSettings{
			UserID:         userID,
			HomepageConfig: data.DefaultHomepageConfig(),
			ParsingRules:   data.DefaultParsingRulesSettings(),
		}
	}

	settings.ParsingRules = rules

	if err := s.settingsRepo.Upsert(settings); err != nil {
		return nil, fmt.Errorf("failed to update parsing rules: %w", err)
	}

	return settings, nil
}

// SettingsOption applies (and validates) an additional setting in UpdateAllSettings.
type SettingsOption func(settings *data.UserSettings) error

var allowedPaginationPositions = map[string]bool{"top": true, "bottom": true, "both": true}

// DefaultDateFormat follows the browser locale.
const DefaultDateFormat = "auto"

// AllowedDateFormats are the date display patterns a user can choose (rendered by the frontend).
var AllowedDateFormats = []string{
	DefaultDateFormat,
	"DD/MM/YYYY",
	"MM/DD/YYYY",
	"YYYY-MM-DD",
	"DD.MM.YYYY",
	"D MMM YYYY",
	"MMM D, YYYY",
}

// WithDateFormat sets how dates are displayed (empty means auto).
func WithDateFormat(format string) SettingsOption {
	return func(settings *data.UserSettings) error {
		if format == "" {
			format = DefaultDateFormat
		}
		for _, allowed := range AllowedDateFormats {
			if format == allowed {
				settings.DateFormat = format
				return nil
			}
		}
		return fmt.Errorf("invalid date format: %s", format)
	}
}

// DefaultGestureSeekStep is the seek of a double-tap on the player sides, in seconds.
const DefaultGestureSeekStep = 10

// AllowedGestureSeekSteps are the double-tap seek steps a user can choose, in seconds.
var AllowedGestureSeekSteps = []int{5, 10, 15, 30}

// WithPlayerGestures turns the player touch gestures on or off and sets the double-tap seek step.
// A nil enabled or a zero seek step keeps the current value, so that a client which does not know
// these settings does not reset them.
func WithPlayerGestures(enabled *bool, seekStep int) SettingsOption {
	return func(settings *data.UserSettings) error {
		if seekStep == 0 {
			seekStep = settings.GestureSeekStep
			if !slices.Contains(AllowedGestureSeekSteps, seekStep) {
				seekStep = DefaultGestureSeekStep
			}
		}
		if !slices.Contains(AllowedGestureSeekSteps, seekStep) {
			return fmt.Errorf("invalid gesture seek step: %d", seekStep)
		}
		if enabled != nil {
			settings.PlayerGestures = *enabled
		}
		settings.GestureSeekStep = seekStep
		return nil
	}
}

// WithPaginationPosition sets where page controls are shown (top, bottom or both; empty means
// bottom).
func WithPaginationPosition(position string) SettingsOption {
	return func(settings *data.UserSettings) error {
		if position == "" {
			position = "bottom"
		}
		if !allowedPaginationPositions[position] {
			return fmt.Errorf("invalid pagination position: %s", position)
		}
		settings.PaginationPosition = position
		return nil
	}
}

func (s *SettingsService) UpdateAllSettings(userID uint, autoplay bool, volume int, loop bool, abLoopControls bool, markerLoop bool, videosPerPage int, sortOrder string, tagSort string, markerThumbnailCycling bool, homepageConfig data.HomepageConfig, parsingRules data.ParsingRulesSettings, sortPrefs data.SortPreferences, playlistAutoAdvance string, playlistCountdownSeconds int, showPageSizeSelector bool, advancedMarkerFilters bool, defaultMarkerView string, sceneCardConfig data.SceneCardConfig, opts ...SettingsOption) (*data.UserSettings, error) {
	if volume < 0 || volume > 100 {
		return nil, fmt.Errorf("volume must be between 0 and 100")
	}
	if videosPerPage < 1 {
		return nil, fmt.Errorf("videos per page must be at least 1")
	}
	if !allowedSortOrders[sortOrder] {
		return nil, fmt.Errorf("invalid sort order: %s", sortOrder)
	}
	if !allowedTagSorts[tagSort] {
		return nil, fmt.Errorf("invalid tag sort: %s", tagSort)
	}
	if !allowedActorSorts[sortPrefs.Actors] {
		return nil, fmt.Errorf("invalid actors sort: %s", sortPrefs.Actors)
	}
	if !allowedStudioSorts[sortPrefs.Studios] {
		return nil, fmt.Errorf("invalid studios sort: %s", sortPrefs.Studios)
	}
	if !allowedMarkerSorts[sortPrefs.Markers] {
		return nil, fmt.Errorf("invalid markers sort: %s", sortPrefs.Markers)
	}
	if !allowedEntitySceneSorts[sortPrefs.ActorScenes] {
		return nil, fmt.Errorf("invalid actor_scenes sort: %s", sortPrefs.ActorScenes)
	}
	if !allowedEntitySceneSorts[sortPrefs.StudioScenes] {
		return nil, fmt.Errorf("invalid studio_scenes sort: %s", sortPrefs.StudioScenes)
	}
	if err := s.validateHomepageConfig(&homepageConfig); err != nil {
		return nil, err
	}
	if err := s.validateParsingRules(&parsingRules); err != nil {
		return nil, err
	}
	if err := s.validateSceneCardConfig(&sceneCardConfig); err != nil {
		return nil, err
	}
	if playlistAutoAdvance == "" {
		playlistAutoAdvance = "countdown"
	}
	if defaultMarkerView == "" {
		defaultMarkerView = "grouped"
	}
	if !allowedMarkerViews[defaultMarkerView] {
		return nil, fmt.Errorf("invalid default marker view: %s", defaultMarkerView)
	}
	if !allowedPlaylistAutoAdvance[playlistAutoAdvance] {
		return nil, fmt.Errorf("invalid playlist auto advance: %s", playlistAutoAdvance)
	}
	if playlistCountdownSeconds < 3 || playlistCountdownSeconds > 15 {
		if playlistCountdownSeconds == 0 {
			playlistCountdownSeconds = 5
		} else {
			return nil, fmt.Errorf("playlist countdown seconds must be between 3 and 15")
		}
	}

	settings, err := s.settingsRepo.GetByUserID(userID)
	if err != nil {
		settings = &data.UserSettings{
			UserID:          userID,
			HomepageConfig:  data.DefaultHomepageConfig(),
			ParsingRules:    data.DefaultParsingRulesSettings(),
			SortPreferences: data.DefaultSortPreferences(),
			SceneCardConfig: data.DefaultSceneCardConfig(),
			PlayerGestures:  true,
			GestureSeekStep: DefaultGestureSeekStep,
		}
	}

	settings.Autoplay = autoplay
	settings.DefaultVolume = volume
	settings.Loop = loop
	settings.AbLoopControls = abLoopControls
	settings.MarkerLoop = markerLoop
	settings.VideosPerPage = videosPerPage
	settings.DefaultSortOrder = sortOrder
	settings.DefaultTagSort = tagSort
	settings.MarkerThumbnailCycling = markerThumbnailCycling
	settings.HomepageConfig = homepageConfig
	settings.ParsingRules = parsingRules
	settings.SortPreferences = sortPrefs
	settings.PlaylistAutoAdvance = playlistAutoAdvance
	settings.PlaylistCountdownSeconds = playlistCountdownSeconds
	settings.ShowPageSizeSelector = showPageSizeSelector
	settings.AdvancedMarkerFilters = advancedMarkerFilters
	settings.DefaultMarkerView = defaultMarkerView
	settings.SceneCardConfig = sceneCardConfig

	for _, opt := range opts {
		if err := opt(settings); err != nil {
			return nil, err
		}
	}

	if err := s.settingsRepo.Upsert(settings); err != nil {
		return nil, fmt.Errorf("failed to update settings: %w", err)
	}

	return settings, nil
}

var allowedRuleTypes = map[string]bool{
	"remove_brackets":      true,
	"remove_numbers":       true,
	"remove_years":         true,
	"remove_special_chars": true,
	"remove_stopwords":     true,
	"remove_duplicates":    true,
	"regex_remove":         true,
	"text_replace":         true,
	"word_length_filter":   true,
	"case_normalize":       true,
}

var allowedCaseTypes = map[string]bool{
	"lower": true,
	"upper": true,
	"title": true,
}

func (s *SettingsService) validateParsingRules(rules *data.ParsingRulesSettings) error {
	if len(rules.Presets) > 20 {
		return fmt.Errorf("maximum of 20 presets allowed")
	}

	seenIDs := make(map[string]bool)
	for i, preset := range rules.Presets {
		if preset.ID == "" {
			return fmt.Errorf("preset %d: id is required", i)
		}
		if seenIDs[preset.ID] {
			return fmt.Errorf("preset %d: duplicate id '%s'", i, preset.ID)
		}
		seenIDs[preset.ID] = true

		if preset.Name == "" {
			return fmt.Errorf("preset %d: name is required", i)
		}
		if len(preset.Name) > 100 {
			return fmt.Errorf("preset %d: name must be 100 characters or less", i)
		}

		if len(preset.Rules) > 50 {
			return fmt.Errorf("preset %d: maximum of 50 rules per preset allowed", i)
		}

		for j, rule := range preset.Rules {
			if rule.ID == "" {
				return fmt.Errorf("preset %d, rule %d: id is required", i, j)
			}
			if !allowedRuleTypes[rule.Type] {
				return fmt.Errorf("preset %d, rule %d: invalid type '%s'", i, j, rule.Type)
			}

			// Validate rule-specific config
			if rule.Type == "case_normalize" && rule.Config.CaseType != "" {
				if !allowedCaseTypes[rule.Config.CaseType] {
					return fmt.Errorf("preset %d, rule %d: invalid case type '%s'", i, j, rule.Config.CaseType)
				}
			}

			if rule.Type == "word_length_filter" && rule.Config.MinLength < 0 {
				return fmt.Errorf("preset %d, rule %d: minLength must be non-negative", i, j)
			}
		}
	}

	// Validate activePresetId references an existing preset
	if rules.ActivePresetID != nil && *rules.ActivePresetID != "" {
		found := false
		for _, preset := range rules.Presets {
			if preset.ID == *rules.ActivePresetID {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("activePresetId references non-existent preset")
		}
	}

	return nil
}

var allowedBadgeFields = map[string]bool{
	"liked":      true,
	"rating":     true,
	"duration":   true,
	"resolution": true,
	"views":      true,
	"jizz_count": true,
	"watched":    true,
	"file_size":  true,
	"added_at":   true,
	"frame_rate": true,
	"tags":       true,
	"actors":     true,
}

var allowedBadgeDirections = map[string]bool{
	"vertical":   true,
	"horizontal": true,
}

var allowedContentFields = map[string]bool{
	"description": true,
	"tags":        true,
	"actors":      true,
	"studio":      true,
	"file_size":   true,
	"added_at":    true,
	"views":       true,
	"resolution":  true,
	"frame_rate":  true,
	"jizz_count":  true,
	"rating":      true,
}

var allowedContentRowTypes = map[string]bool{
	"full":  true,
	"split": true,
}

var allowedContentModes = map[string]bool{
	"short": true,
	"long":  true,
}

func (s *SettingsService) validateSceneCardConfig(config *data.SceneCardConfig) error {
	// Validate badge zones
	zones := []*data.BadgeZone{
		&config.Badges.TopLeft,
		&config.Badges.TopRight,
		&config.Badges.BottomLeft,
		&config.Badges.BottomRight,
	}
	zoneNames := []string{"top_left", "top_right", "bottom_left", "bottom_right"}

	for i, zone := range zones {
		if !allowedBadgeDirections[zone.Direction] {
			return fmt.Errorf("badge zone %s: invalid direction '%s'", zoneNames[i], zone.Direction)
		}
		if len(zone.Items) > 5 {
			return fmt.Errorf("badge zone %s: maximum of 5 items allowed", zoneNames[i])
		}
		for _, item := range zone.Items {
			if !allowedBadgeFields[item] {
				return fmt.Errorf("badge zone %s: invalid badge field '%s'", zoneNames[i], item)
			}
		}
	}

	// Validate content rows
	if len(config.ContentRows) > 10 {
		return fmt.Errorf("maximum of 10 content rows allowed")
	}

	for i, row := range config.ContentRows {
		if !allowedContentRowTypes[row.Type] {
			return fmt.Errorf("content row %d: invalid type '%s'", i, row.Type)
		}

		switch row.Type {
		case "full":
			if row.Field == "" {
				return fmt.Errorf("content row %d: field is required for full-width rows", i)
			}
			if !allowedContentFields[row.Field] {
				return fmt.Errorf("content row %d: invalid field '%s'", i, row.Field)
			}
			if (row.Field == "tags" || row.Field == "actors") && row.Mode != "" {
				if !allowedContentModes[row.Mode] {
					return fmt.Errorf("content row %d: invalid mode '%s'", i, row.Mode)
				}
			}
		case "split":
			if row.Left == "" || row.Right == "" {
				return fmt.Errorf("content row %d: left and right fields are required for split rows", i)
			}
			if !allowedContentFields[row.Left] {
				return fmt.Errorf("content row %d: invalid left field '%s'", i, row.Left)
			}
			if !allowedContentFields[row.Right] {
				return fmt.Errorf("content row %d: invalid right field '%s'", i, row.Right)
			}
			if (row.Left == "tags" || row.Left == "actors") && row.LeftMode != "" {
				if !allowedContentModes[row.LeftMode] {
					return fmt.Errorf("content row %d: invalid left_mode '%s'", i, row.LeftMode)
				}
			}
			if (row.Right == "tags" || row.Right == "actors") && row.RightMode != "" {
				if !allowedContentModes[row.RightMode] {
					return fmt.Errorf("content row %d: invalid right_mode '%s'", i, row.RightMode)
				}
			}
		}
	}

	return nil
}
