package core

import (
	"fmt"
	"time"

	"goonhub/internal/data"
)

// Shared helpers of the display widgets (shorts, preview_wall, hero, top_period, quick_rate).

func displayWidgetQueries(s *HomepageService) (data.DisplayWidgetQueries, error) {
	db := s.WidgetDeps().DB
	if db == nil {
		return data.DisplayWidgetQueries{}, fmt.Errorf("homepage widget database is not configured")
	}
	return data.NewDisplayWidgetQueries(db), nil
}

// displayValidateOneOf checks that an optional string config key is one of the allowed values.
func displayValidateOneOf(cfg map[string]interface{}, key string, allowed ...string) error {
	raw, present := cfg[key]
	if !present || raw == nil {
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

// displayValidateIntRange checks that an optional numeric config key is within [lo, hi].
func displayValidateIntRange(cfg map[string]interface{}, key string, lo, hi int) error {
	raw, present := cfg[key]
	if !present || raw == nil {
		return nil
	}
	if _, ok := raw.(float64); !ok {
		if _, ok := raw.(int); !ok {
			return fmt.Errorf("%s must be a number", key)
		}
	}
	v := widgetConfigInt(cfg, key, lo-1)
	if v < lo || v > hi {
		return fmt.Errorf("%s must be between %d and %d", key, lo, hi)
	}
	return nil
}

// displayValidateBool checks that an optional config key is a boolean.
func displayValidateBool(cfg map[string]interface{}, key string) error {
	raw, present := cfg[key]
	if !present || raw == nil {
		return nil
	}
	if _, ok := raw.(bool); !ok {
		return fmt.Errorf("%s must be a boolean", key)
	}
	return nil
}

func displaySceneIDs(scenes []data.Scene) []uint {
	ids := make([]uint, len(scenes))
	for i, s := range scenes {
		ids[i] = s.ID
	}
	return ids
}

// displayOrderScenes returns the scenes in the order of ids (scenes not found are skipped).
func displayOrderScenes(scenes []data.Scene, ids []uint) []data.Scene {
	byID := make(map[uint]data.Scene, len(scenes))
	for _, s := range scenes {
		byID[s.ID] = s
	}
	out := make([]data.Scene, 0, len(ids))
	for _, id := range ids {
		if s, ok := byID[id]; ok {
			out = append(out, s)
		}
	}
	return out
}

// displaySourceScenes returns scenes from a source, preferring those with a generated
// preview video and completing with the others when there are not enough.
func displaySourceScenes(q data.DisplayWidgetQueries, userID uint, source string, limit int) ([]data.Scene, error) {
	withPreview, err := q.SourceScenes(userID, source, true, limit)
	if err != nil {
		return nil, err
	}
	if len(withPreview) >= limit {
		return withPreview, nil
	}
	others, err := q.SourceScenes(userID, source, false, limit)
	if err != nil {
		return nil, err
	}
	seen := make(map[uint]bool, len(withPreview))
	for _, s := range withPreview {
		seen[s.ID] = true
	}
	for _, s := range others {
		if len(withPreview) >= limit {
			break
		}
		if !seen[s.ID] {
			withPreview = append(withPreview, s)
			seen[s.ID] = true
		}
	}
	return withPreview, nil
}

// displayPeriodWindow returns the rolling window of a period ("week", "month", "year") ending now,
// and the window just before it.
func displayPeriodWindow(period string, now time.Time) (from, to, prevFrom time.Time) {
	days := 30
	switch period {
	case "week":
		days = 7
	case "year":
		days = 365
	}
	to = now
	from = now.AddDate(0, 0, -days)
	prevFrom = from.AddDate(0, 0, -days)
	return from, to, prevFrom
}
