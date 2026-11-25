package core

import (
	"fmt"

	"goonhub/internal/data"
)

// Shared helpers of the insight widgets (trends, performer_spotlight, curation_todo,
// processing_status, saved_search_tiles).

// insightsQueries returns the SQL helper of the insight widgets.
func insightsQueries(s *HomepageService) (*data.InsightsWidgetQueries, error) {
	db := s.WidgetDeps().DB
	if db == nil {
		return nil, fmt.Errorf("homepage widgets: database not available")
	}
	return data.NewInsightsWidgetQueries(db), nil
}

// insightsUserIsAdmin reports whether the user has the library:insights permission (the
// curation and processing details of the insight widgets).
func insightsUserIsAdmin(s *HomepageService, userID uint) bool {
	return s.userHas(userID, PermLibraryInsights)
}

// userHas reports whether the user currently has a permission (false without a permission
// checker).
func (s *HomepageService) userHas(userID uint, permission string) bool {
	perms := s.WidgetDeps().Permissions
	if perms == nil {
		return false
	}
	return perms.UserHasPermission(userID, permission)
}

// insightsValidateOneOf checks an optional string config value against allowed values.
func insightsValidateOneOf(cfg map[string]interface{}, key string, allowed ...string) error {
	raw, ok := cfg[key]
	if !ok || raw == nil {
		return nil
	}
	value, ok := raw.(string)
	if !ok {
		return fmt.Errorf("%s must be a string", key)
	}
	if value == "" {
		return nil
	}
	for _, a := range allowed {
		if value == a {
			return nil
		}
	}
	return fmt.Errorf("%s must be one of %v", key, allowed)
}

// insightsValidateIntRange checks an optional numeric config value.
func insightsValidateIntRange(cfg map[string]interface{}, key string, lo, hi int) error {
	if _, ok := cfg[key]; !ok {
		return nil
	}
	v := widgetConfigInt(cfg, key, lo-1)
	if v < lo || v > hi {
		return fmt.Errorf("%s must be between %d and %d", key, lo, hi)
	}
	return nil
}
