package core

import (
	"fmt"
	"goonhub/internal/data"
	"slices"
	"strings"
	"sync"

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
	// writeMu serializes the read-modify-write updates of the settings rows
	writeMu sync.Mutex
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
		return defaultUserSettings(userID), nil
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

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

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

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

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

// SettingsOption applies (and validates) a setting in UpdateSettings and UpdateAllSettings.
// An option is applied twice (a validation dry run on the defaults, then the real update): it
// must not modify the values it captured, and only depend on the current settings to keep a
// value.
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
		value := format
		if value == "" {
			value = DefaultDateFormat
		}
		for _, allowed := range AllowedDateFormats {
			if value == allowed {
				settings.DateFormat = value
				return nil
			}
		}
		return fmt.Errorf("invalid date format: %s", value)
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
		step := seekStep
		if step == 0 {
			step = settings.GestureSeekStep
			if !slices.Contains(AllowedGestureSeekSteps, step) {
				step = DefaultGestureSeekStep
			}
		}
		if !slices.Contains(AllowedGestureSeekSteps, step) {
			return fmt.Errorf("invalid gesture seek step: %d", step)
		}
		if enabled != nil {
			settings.PlayerGestures = *enabled
		}
		settings.GestureSeekStep = step
		return nil
	}
}

// WithReplayHeatmap sets whether the "most replayed" heatmap is shown on the player and
// whether watched parts are recorded; nil keeps the current value (on for a new row).
func WithReplayHeatmap(show, record *bool) SettingsOption {
	return func(settings *data.UserSettings) error {
		if show != nil {
			settings.ShowReplayHeatmap = *show
		}
		if record != nil {
			settings.RecordReplayHeatmap = *record
		}
		return nil
	}
}

// WithScrubber sets whether the thumbnail scrubber strip is shown under the player on the watch
// page; nil keeps the current value (off for a new row).
func WithScrubber(show *bool) SettingsOption {
	return func(settings *data.UserSettings) error {
		if show != nil {
			settings.ShowScrubber = *show
		}
		return nil
	}
}

// WithScrubberWheel sets whether the mouse wheel over a thumbnail scrubber scrolls it or seeks;
// nil keeps the current value (off for a new row, so the wheel scrolls the page).
func WithScrubberWheel(enabled *bool) SettingsOption {
	return func(settings *data.UserSettings) error {
		if enabled != nil {
			settings.ScrubberWheel = *enabled
		}
		return nil
	}
}

// Modes of the thumbnail scrubber strip: free scrolls on its own (click to seek, follows
// playback), centered keeps the playback position under the centre line and seeks when the
// strip is moved (like Stash).
const (
	ScrubberModeFree     = "free"
	ScrubberModeCentered = "centered"
)

var allowedScrubberModes = map[string]bool{ScrubberModeFree: true, ScrubberModeCentered: true}

// WithScrubberMode sets the mode of the thumbnail scrubber strip; nil keeps the current value
// (free for a new row).
func WithScrubberMode(mode *string) SettingsOption {
	return func(settings *data.UserSettings) error {
		if mode == nil {
			return nil
		}
		if !allowedScrubberModes[*mode] {
			return fmt.Errorf("invalid scrubber mode: %q", *mode)
		}
		settings.ScrubberMode = *mode
		return nil
	}
}

// WithPaginationPosition sets where page controls are shown (top, bottom or both; empty means
// bottom).
func WithPaginationPosition(position string) SettingsOption {
	return func(settings *data.UserSettings) error {
		value := position
		if value == "" {
			value = "bottom"
		}
		if !allowedPaginationPositions[value] {
			return fmt.Errorf("invalid pagination position: %s", value)
		}
		settings.PaginationPosition = value
		return nil
	}
}

// Widths of the scene list pages: default keeps the usual maximum width, wide raises it, full
// goes edge to edge (with the usual side padding). The grids add columns as the width grows.
const (
	ContentWidthDefault = "default"
	ContentWidthWide    = "wide"
	ContentWidthFull    = "full"
)

var allowedContentWidths = map[string]bool{ContentWidthDefault: true, ContentWidthWide: true, ContentWidthFull: true}

// WithContentWidth sets the width of the scene list pages; nil keeps the current value (default
// for a new row).
func WithContentWidth(width *string) SettingsOption {
	return func(settings *data.UserSettings) error {
		if width == nil {
			return nil
		}
		if !allowedContentWidths[*width] {
			return fmt.Errorf("invalid content width: %q", *width)
		}
		settings.ContentWidth = *width
		return nil
	}
}

// DefaultGridDensity is the card size of the scene grids by default.
const DefaultGridDensity = "comfortable"

var allowedGridDensities = map[string]bool{"compact": true, DefaultGridDensity: true, "large": true}

// WithGridDensity sets the card size of the scene grids (empty means comfortable).
func WithGridDensity(density string) SettingsOption {
	return func(settings *data.UserSettings) error {
		value := density
		if value == "" {
			value = DefaultGridDensity
		}
		if !allowedGridDensities[value] {
			return fmt.Errorf("invalid grid density: %s", value)
		}
		settings.GridDensity = value
		return nil
	}
}

// WithInfiniteScroll turns infinite scroll on or off on the scene list pages (nil keeps the
// current value).
func WithInfiniteScroll(enabled *bool) SettingsOption {
	return func(settings *data.UserSettings) error {
		if enabled != nil {
			settings.InfiniteScroll = *enabled
		}
		return nil
	}
}

// SceneListPages are the page types whose scene list can be shown as a grid or a table.
var SceneListPages = []string{"search", "actor", "studio", "playlist", "explorer"}

// SceneTableColumns are the columns a scene table can show.
var SceneTableColumns = []string{
	"thumbnail", "title", "duration", "resolution", "file_size", "rating", "views",
	"studio", "actors", "release_date", "added_at",
}

// WithSceneListView sets the grid/table view per page and the table columns. A nil view keeps
// the current one (clients that do not know the setting).
func WithSceneListView(view *data.SceneListView) SettingsOption {
	return func(settings *data.UserSettings) error {
		if view == nil {
			return nil
		}
		validated, err := validateSceneListView(*view)
		if err != nil {
			return err
		}
		settings.SceneListView = validated
		return nil
	}
}

func validateSceneListView(view data.SceneListView) (data.SceneListView, error) {
	out := data.SceneListView{Pages: map[string]string{}, Columns: []string{}}
	for page, mode := range view.Pages {
		if !slices.Contains(SceneListPages, page) {
			return out, fmt.Errorf("invalid scene list page: %s", page)
		}
		if mode != data.SceneListViewGrid && mode != data.SceneListViewTable {
			return out, fmt.Errorf("invalid scene list view for %s: %s", page, mode)
		}
		out.Pages[page] = mode
	}
	for _, col := range view.Columns {
		if !slices.Contains(SceneTableColumns, col) {
			return out, fmt.Errorf("invalid scene table column: %s", col)
		}
		if slices.Contains(out.Columns, col) {
			return out, fmt.Errorf("duplicate scene table column: %s", col)
		}
		out.Columns = append(out.Columns, col)
	}
	out.Normalize()
	return out, nil
}

// WithAutoplay sets whether scenes start playing on their own.
func WithAutoplay(enabled bool) SettingsOption {
	return withBool(func(s *data.UserSettings) *bool { return &s.Autoplay }, enabled)
}

// WithLoop sets whether the player loops the scene.
func WithLoop(enabled bool) SettingsOption {
	return withBool(func(s *data.UserSettings) *bool { return &s.Loop }, enabled)
}

// WithAbLoopControls shows or hides the A-B loop controls of the player.
func WithAbLoopControls(enabled bool) SettingsOption {
	return withBool(func(s *data.UserSettings) *bool { return &s.AbLoopControls }, enabled)
}

// WithMarkerLoop sets whether playing a marker loops it.
func WithMarkerLoop(enabled bool) SettingsOption {
	return withBool(func(s *data.UserSettings) *bool { return &s.MarkerLoop }, enabled)
}

// WithMarkerThumbnailCycling sets whether marker thumbnails cycle on hover.
func WithMarkerThumbnailCycling(enabled bool) SettingsOption {
	return withBool(func(s *data.UserSettings) *bool { return &s.MarkerThumbnailCycling }, enabled)
}

// WithShowPageSizeSelector shows or hides the page size selector of the paginated pages.
func WithShowPageSizeSelector(enabled bool) SettingsOption {
	return withBool(func(s *data.UserSettings) *bool { return &s.ShowPageSizeSelector }, enabled)
}

// WithAdvancedMarkerFilters shows or hides the advanced filters of the markers page.
func WithAdvancedMarkerFilters(enabled bool) SettingsOption {
	return withBool(func(s *data.UserSettings) *bool { return &s.AdvancedMarkerFilters }, enabled)
}

func withBool(field func(*data.UserSettings) *bool, value bool) SettingsOption {
	return func(settings *data.UserSettings) error {
		*field(settings) = value
		return nil
	}
}

// WithDefaultVolume sets the initial player volume (0 to 100).
func WithDefaultVolume(volume int) SettingsOption {
	return func(settings *data.UserSettings) error {
		if volume < 0 || volume > 100 {
			return fmt.Errorf("volume must be between 0 and 100")
		}
		settings.DefaultVolume = volume
		return nil
	}
}

// WithVideosPerPage sets the page size of the paginated pages (at least 1; the handler caps it
// to the admin maximum).
func WithVideosPerPage(videosPerPage int) SettingsOption {
	return func(settings *data.UserSettings) error {
		if videosPerPage < 1 {
			return fmt.Errorf("videos per page must be at least 1")
		}
		settings.VideosPerPage = videosPerPage
		return nil
	}
}

// WithDefaultSortOrder sets the default sort order of the scene lists.
func WithDefaultSortOrder(sortOrder string) SettingsOption {
	return func(settings *data.UserSettings) error {
		if !allowedSortOrders[sortOrder] {
			return fmt.Errorf("invalid sort order: %s", sortOrder)
		}
		settings.DefaultSortOrder = sortOrder
		return nil
	}
}

// WithDefaultTagSort sets the default sort of the tag lists.
func WithDefaultTagSort(tagSort string) SettingsOption {
	return func(settings *data.UserSettings) error {
		if !allowedTagSorts[tagSort] {
			return fmt.Errorf("invalid tag sort: %s", tagSort)
		}
		settings.DefaultTagSort = tagSort
		return nil
	}
}

// WithSortPreferences replaces the default sorts of the actor, studio and marker pages.
func WithSortPreferences(prefs data.SortPreferences) SettingsOption {
	return func(settings *data.UserSettings) error {
		if !allowedActorSorts[prefs.Actors] {
			return fmt.Errorf("invalid actors sort: %s", prefs.Actors)
		}
		if !allowedStudioSorts[prefs.Studios] {
			return fmt.Errorf("invalid studios sort: %s", prefs.Studios)
		}
		if !allowedMarkerSorts[prefs.Markers] {
			return fmt.Errorf("invalid markers sort: %s", prefs.Markers)
		}
		if !allowedEntitySceneSorts[prefs.ActorScenes] {
			return fmt.Errorf("invalid actor_scenes sort: %s", prefs.ActorScenes)
		}
		if !allowedEntitySceneSorts[prefs.StudioScenes] {
			return fmt.Errorf("invalid studio_scenes sort: %s", prefs.StudioScenes)
		}
		settings.SortPreferences = prefs
		return nil
	}
}

// WithHomepageConfig replaces the homepage layout (tabs and sections).
func (s *SettingsService) WithHomepageConfig(config data.HomepageConfig) SettingsOption {
	return func(settings *data.UserSettings) error {
		value := config
		if err := s.validateHomepageConfig(&value); err != nil {
			return err
		}
		settings.HomepageConfig = value
		return nil
	}
}

// WithParsingRules replaces the title parsing presets.
func (s *SettingsService) WithParsingRules(rules data.ParsingRulesSettings) SettingsOption {
	return func(settings *data.UserSettings) error {
		value := rules
		if err := s.validateParsingRules(&value); err != nil {
			return err
		}
		settings.ParsingRules = value
		return nil
	}
}

// WithSceneCardConfig replaces the scene card template (badges and content rows).
func (s *SettingsService) WithSceneCardConfig(config data.SceneCardConfig) SettingsOption {
	return func(settings *data.UserSettings) error {
		value := config
		if err := s.validateSceneCardConfig(&value); err != nil {
			return err
		}
		settings.SceneCardConfig = value
		return nil
	}
}

// WithDefaultMarkerView sets the default view of the markers page (empty means grouped).
func WithDefaultMarkerView(view string) SettingsOption {
	return func(settings *data.UserSettings) error {
		value := view
		if value == "" {
			value = "grouped"
		}
		if !allowedMarkerViews[value] {
			return fmt.Errorf("invalid default marker view: %s", value)
		}
		settings.DefaultMarkerView = value
		return nil
	}
}

// WithPlaylistAutoAdvance sets what happens at the end of a playlist scene (empty means
// countdown).
func WithPlaylistAutoAdvance(mode string) SettingsOption {
	return func(settings *data.UserSettings) error {
		value := mode
		if value == "" {
			value = "countdown"
		}
		if !allowedPlaylistAutoAdvance[value] {
			return fmt.Errorf("invalid playlist auto advance: %s", value)
		}
		settings.PlaylistAutoAdvance = value
		return nil
	}
}

// WithPlaylistCountdownSeconds sets the countdown before the next playlist scene (3 to 15
// seconds, 0 means 5).
func WithPlaylistCountdownSeconds(seconds int) SettingsOption {
	return func(settings *data.UserSettings) error {
		value := seconds
		if value == 0 {
			value = 5
		}
		if value < 3 || value > 15 {
			return fmt.Errorf("playlist countdown seconds must be between 3 and 15")
		}
		settings.PlaylistCountdownSeconds = value
		return nil
	}
}

// WithGestureSeekStep sets the double-tap seek step of the player (one of
// AllowedGestureSeekSteps; unlike WithPlayerGestures, 0 is rejected).
func WithGestureSeekStep(seekStep int) SettingsOption {
	return func(settings *data.UserSettings) error {
		if !slices.Contains(AllowedGestureSeekSteps, seekStep) {
			return fmt.Errorf("invalid gesture seek step: %d", seekStep)
		}
		settings.GestureSeekStep = seekStep
		return nil
	}
}

// UpdateAllSettings replaces all the settings of the user (PUT /settings). The positional values
// are always applied; the options add the settings that older clients may leave out.
func (s *SettingsService) UpdateAllSettings(userID uint, autoplay bool, volume int, loop bool, abLoopControls bool, markerLoop bool, videosPerPage int, sortOrder string, tagSort string, markerThumbnailCycling bool, homepageConfig data.HomepageConfig, parsingRules data.ParsingRulesSettings, sortPrefs data.SortPreferences, playlistAutoAdvance string, playlistCountdownSeconds int, showPageSizeSelector bool, advancedMarkerFilters bool, defaultMarkerView string, sceneCardConfig data.SceneCardConfig, opts ...SettingsOption) (*data.UserSettings, error) {
	// Validated in this order, so that the first error stays the same as before
	all := []SettingsOption{
		WithDefaultVolume(volume),
		WithVideosPerPage(videosPerPage),
		WithDefaultSortOrder(sortOrder),
		WithDefaultTagSort(tagSort),
		WithSortPreferences(sortPrefs),
		s.WithHomepageConfig(homepageConfig),
		s.WithParsingRules(parsingRules),
		s.WithSceneCardConfig(sceneCardConfig),
		WithDefaultMarkerView(defaultMarkerView),
		WithPlaylistAutoAdvance(playlistAutoAdvance),
		WithPlaylistCountdownSeconds(playlistCountdownSeconds),
		WithAutoplay(autoplay),
		WithLoop(loop),
		WithAbLoopControls(abLoopControls),
		WithMarkerLoop(markerLoop),
		WithMarkerThumbnailCycling(markerThumbnailCycling),
		WithShowPageSizeSelector(showPageSizeSelector),
		WithAdvancedMarkerFilters(advancedMarkerFilters),
	}
	return s.UpdateSettings(userID, append(all, opts...)...)
}

// UpdateSettings applies the given settings to the stored ones of the user (PATCH /settings):
// the settings without an option keep their value. A user without a settings row starts from
// the defaults. Every option is validated before the settings are read, so an invalid value
// leaves them untouched.
func (s *SettingsService) UpdateSettings(userID uint, opts ...SettingsOption) (*data.UserSettings, error) {
	if len(opts) == 0 {
		return nil, fmt.Errorf("no settings to update")
	}
	// Dry run on the defaults: the options only depend on the current value to keep it
	probe := newSettingsRow(userID)
	for _, opt := range opts {
		if err := opt(probe); err != nil {
			return nil, err
		}
	}

	// Serializes the read-modify-write of the settings rows (two quick saves of different
	// settings must not overwrite each other)
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	settings, err := s.settingsRepo.GetByUserID(userID)
	created := false
	if err != nil {
		created = true
		settings = newSettingsRow(userID)
	}

	for _, opt := range opts {
		if err := opt(settings); err != nil {
			return nil, err
		}
	}

	intended := *settings
	if err := s.settingsRepo.Upsert(settings); err != nil {
		return nil, fmt.Errorf("failed to update settings: %w", err)
	}
	// On insert GORM leaves out zero values of columns that have a default and reads the column
	// defaults back into the struct (a setting that is on by default but turned off here would be
	// stored as on): write the new row once more with the intended values.
	if created && hasZeroedDefaults(&intended) {
		intended.ID, intended.CreatedAt, intended.UpdatedAt = settings.ID, settings.CreatedAt, settings.UpdatedAt
		*settings = intended
		if err := s.settingsRepo.Upsert(settings); err != nil {
			return nil, fmt.Errorf("failed to update settings: %w", err)
		}
	}

	return settings, nil
}

// hasZeroedDefaults reports whether a setting whose column has a non-zero default is set to
// its zero value (GORM does not write those on insert).
func hasZeroedDefaults(settings *data.UserSettings) bool {
	return !settings.ShowReplayHeatmap || !settings.RecordReplayHeatmap ||
		!settings.MarkerThumbnailCycling || settings.DefaultVolume == 0
}

// defaultUserSettings are the settings of a user without a settings row.
func defaultUserSettings(userID uint) *data.UserSettings {
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
		GridDensity:            DefaultGridDensity,
		ScrubberMode:           ScrubberModeFree,
		ContentWidth:           ContentWidthDefault,
		PlayerGestures:         true,
		GestureSeekStep:        DefaultGestureSeekStep,
		ShowReplayHeatmap:      true,
		RecordReplayHeatmap:    true,
		HomepageConfig:         data.DefaultHomepageConfig(),
		SortPreferences:        data.DefaultSortPreferences(),
		SceneCardConfig:        data.DefaultSceneCardConfig(),
		SceneListView:          data.DefaultSceneListView(),
	}
}

// newSettingsRow is the row created for a user on the first settings update: the defaults,
// with the playlist and parsing defaults that the row stores.
func newSettingsRow(userID uint) *data.UserSettings {
	settings := defaultUserSettings(userID)
	settings.ParsingRules = data.DefaultParsingRulesSettings()
	settings.PlaylistAutoAdvance = "countdown"
	settings.PlaylistCountdownSeconds = 5
	return settings
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
		if len(zone.Items) > data.MaxBadgeZoneItems {
			return fmt.Errorf("badge zone %s: maximum of %d items allowed", zoneNames[i], data.MaxBadgeZoneItems)
		}
		if zone.Columns < 0 || zone.Columns > data.MaxBadgeZoneColumns {
			return fmt.Errorf("badge zone %s: columns must be between 1 and %d, or left out", zoneNames[i], data.MaxBadgeZoneColumns)
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
