package core

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"gorm.io/gorm"

	"goonhub/internal/data"
)

// Homepage widgets are section types beyond the built-in scene lists (stats, markers, tools...).
// Each widget lives in its own homepage_widget_<name>.go file and registers itself from init():
//
//	func init() {
//		registerHomepageWidget("my_widget", HomepageWidgetSpec{Fetch: fetchMyWidget, Validate: validateMyWidget})
//	}
//
// A widget returns a HomepageSectionData: Scenes for scene based widgets, and/or any JSON
// serializable payload in Widget for the frontend component of the same type.

// HomepageWidgetDeps gives widgets access to data beyond the HomepageService repositories.
type HomepageWidgetDeps struct {
	DB            *gorm.DB
	Markers       data.MarkerRepository
	RelatedScenes *RelatedScenesService
	JobHistory    data.JobHistoryRepository
	Duplicates    *DuplicateService
	StoragePaths  data.StoragePathRepository
}

// HomepageWidgetSpec describes a widget section type.
type HomepageWidgetSpec struct {
	// Fetch builds the section data for a user.
	Fetch func(s *HomepageService, userID uint, section data.HomepageSection) (*HomepageSectionData, error)
	// Validate checks the section config when the homepage config is saved (optional).
	Validate func(config map[string]interface{}) error
}

var homepageWidgets = map[string]HomepageWidgetSpec{}

// registerHomepageWidget registers a widget section type (call from init()).
func registerHomepageWidget(sectionType string, spec HomepageWidgetSpec) {
	if spec.Fetch == nil {
		panic(fmt.Sprintf("homepage widget %q has no Fetch function", sectionType))
	}
	if _, exists := homepageWidgets[sectionType]; exists {
		panic(fmt.Sprintf("homepage widget %q registered twice", sectionType))
	}
	homepageWidgets[sectionType] = spec
}

// IsHomepageWidgetType reports whether a section type is a registered widget.
func IsHomepageWidgetType(sectionType string) bool {
	_, ok := homepageWidgets[sectionType]
	return ok
}

// HomepageWidgetTypes lists the registered widget types (sorted).
func HomepageWidgetTypes() []string {
	types := make([]string, 0, len(homepageWidgets))
	for t := range homepageWidgets {
		types = append(types, t)
	}
	sort.Strings(types)
	return types
}

func validateHomepageWidgetConfig(section *data.HomepageSection) error {
	spec, ok := homepageWidgets[section.Type]
	if !ok || spec.Validate == nil {
		return nil
	}
	cfg := section.Config
	if cfg == nil {
		cfg = map[string]interface{}{}
	}
	return spec.Validate(cfg)
}

// SetWidgetDeps provides the dependencies used by homepage widgets.
func (s *HomepageService) SetWidgetDeps(deps HomepageWidgetDeps) {
	s.widgetDeps = deps
}

// WidgetDeps returns the dependencies available to homepage widgets.
func (s *HomepageService) WidgetDeps() HomepageWidgetDeps {
	return s.widgetDeps
}

// emptyWidgetData returns a section with no scenes, for widgets that only use Widget.
func emptyWidgetData(section data.HomepageSection, payload any) *HomepageSectionData {
	return &HomepageSectionData{Section: section, Scenes: []data.Scene{}, Widget: payload}
}

// --- Section config helpers (JSON numbers decode as float64) ---

func widgetConfigInt(cfg map[string]interface{}, key string, def int) int {
	switch v := cfg[key].(type) {
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return def
		}
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	case string:
		var n int
		if _, err := fmt.Sscanf(strings.TrimSpace(v), "%d", &n); err == nil {
			return n
		}
	}
	return def
}

func widgetConfigString(cfg map[string]interface{}, key, def string) string {
	if v, ok := cfg[key].(string); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return def
}

func widgetConfigBool(cfg map[string]interface{}, key string, def bool) bool {
	if v, ok := cfg[key].(bool); ok {
		return v
	}
	return def
}

func widgetConfigStrings(cfg map[string]interface{}, key string) []string {
	var out []string
	switch v := cfg[key].(type) {
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
	case []string:
		for _, s := range v {
			if strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
	case string:
		for _, s := range strings.Split(v, ",") {
			if strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
	}
	return out
}

// clampInt bounds a config value.
func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
