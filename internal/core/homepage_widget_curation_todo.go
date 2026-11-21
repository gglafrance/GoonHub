package core

import (
	"fmt"

	"goonhub/internal/data"
)

// curation_todo: library scenes missing tags, actors, studio, thumbnail or sprites, scenes the
// user watched without rating them, and (admins only) unresolved duplicate groups.

// InsightsCurationPayload is the curation_todo widget payload.
type InsightsCurationPayload struct {
	data.InsightsCurationCounts
	IsAdmin bool `json:"is_admin"`
	// UnresolvedDuplicates is only set for admins (nil otherwise, or when detection is off)
	UnresolvedDuplicates *int64 `json:"unresolved_duplicates,omitempty"`
}

func init() {
	registerHomepageWidget("curation_todo", HomepageWidgetSpec{Fetch: fetchInsightsCurationWidget})
}

func fetchInsightsCurationWidget(s *HomepageService, userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	q, err := insightsQueries(s)
	if err != nil {
		return nil, err
	}
	counts, err := q.CurationCounts(userID)
	if err != nil {
		return nil, fmt.Errorf("curation to-do: failed to count scenes: %w", err)
	}
	payload := InsightsCurationPayload{InsightsCurationCounts: counts, IsAdmin: insightsUserIsAdmin(s, userID)}
	if payload.IsAdmin && s.WidgetDeps().Duplicates != nil {
		if stats, err := s.WidgetDeps().Duplicates.GetStats(); err == nil {
			payload.UnresolvedDuplicates = &stats.Unresolved
		} else {
			s.logger.Warn("curation to-do: failed to read duplicate stats")
		}
	}
	return emptyWidgetData(section, payload), nil
}
