package core

import (
	"fmt"
	"sort"
	"time"

	"goonhub/internal/data"
)

// processing_status (admins only): pending, running and recently failed jobs per phase, and the
// share of library scenes having each generated asset.

// insightsProcessingFailedWindow is how far back failed jobs are counted.
const insightsProcessingFailedWindow = 24 * time.Hour

var insightsProcessingNow = time.Now

// InsightsPhaseStatus counts the jobs of a processing phase.
type InsightsPhaseStatus struct {
	Phase   string `json:"phase"`
	Pending int64  `json:"pending"`
	Running int64  `json:"running"`
	Failed  int64  `json:"failed"`
}

// InsightsProcessingPayload is the processing_status widget payload.
type InsightsProcessingPayload struct {
	// Forbidden is set (and nothing else) for non-admin users
	Forbidden         bool                   `json:"forbidden,omitempty"`
	Phases            []InsightsPhaseStatus  `json:"phases,omitempty"`
	Coverage          *data.InsightsCoverage `json:"coverage,omitempty"`
	FailedWindowHours int                    `json:"failed_window_hours,omitempty"`
}

func init() {
	registerHomepageWidget("processing_status", HomepageWidgetSpec{Fetch: fetchInsightsProcessingWidget})
}

func fetchInsightsProcessingWidget(s *HomepageService, userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	if !insightsUserIsAdmin(s, userID) {
		return emptyWidgetData(section, InsightsProcessingPayload{Forbidden: true}), nil
	}
	q, err := insightsQueries(s)
	if err != nil {
		return nil, err
	}
	counts, err := q.ActiveJobCounts(insightsProcessingNow().Add(-insightsProcessingFailedWindow))
	if err != nil {
		return nil, fmt.Errorf("processing status: failed to count jobs: %w", err)
	}
	coverage, err := q.Coverage()
	if err != nil {
		return nil, fmt.Errorf("processing status: failed to compute coverage: %w", err)
	}
	return emptyWidgetData(section, InsightsProcessingPayload{
		Phases:            insightsGroupPhases(counts),
		Coverage:          &coverage,
		FailedWindowHours: int(insightsProcessingFailedWindow / time.Hour),
	}), nil
}

// insightsProcessingPhaseOrder is the display order of the processing phases.
var insightsProcessingPhaseOrder = map[string]int{
	"metadata": 0, "thumbnail": 1, "sprites": 2, "animated_thumbnails": 3, "fingerprint": 4,
}

// insightsGroupPhases folds the per (phase, status) counts into one row per phase.
func insightsGroupPhases(counts []data.InsightsJobCount) []InsightsPhaseStatus {
	byPhase := map[string]*InsightsPhaseStatus{}
	for _, c := range counts {
		p := byPhase[c.Phase]
		if p == nil {
			p = &InsightsPhaseStatus{Phase: c.Phase}
			byPhase[c.Phase] = p
		}
		switch c.Status {
		case "pending":
			p.Pending += c.Count
		case "running":
			p.Running += c.Count
		case "failed":
			p.Failed += c.Count
		}
	}
	out := make([]InsightsPhaseStatus, 0, len(byPhase))
	for _, p := range byPhase {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool {
		oi, iok := insightsProcessingPhaseOrder[out[i].Phase]
		oj, jok := insightsProcessingPhaseOrder[out[j].Phase]
		if iok != jok {
			return iok
		}
		if oi != oj {
			return oi < oj
		}
		return out[i].Phase < out[j].Phase
	})
	return out
}
