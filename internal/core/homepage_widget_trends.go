package core

import (
	"fmt"
	"sort"
	"time"

	"goonhub/internal/data"
)

// trends: tags and actors whose share of the user's views grew the most in the recent window
// (config recent_days, default 30) compared to the baseline window just before it (config
// baseline_days, default 180). Config kind: both|tags|actors, min_views: minimum recent views.

const (
	insightsTrendsDefaultRecentDays   = 30
	insightsTrendsDefaultBaselineDays = 180
	insightsTrendsDefaultMinViews     = 2
)

var insightsTrendsNow = time.Now

// InsightsTrendItem is a rising tag or actor.
type InsightsTrendItem struct {
	Kind        string  `json:"kind"` // tag | actor
	ID          uint    `json:"id"`
	UUID        string  `json:"uuid,omitempty"`
	Name        string  `json:"name"`
	Color       string  `json:"color,omitempty"`
	RecentViews int64   `json:"recent_views"`
	BaseViews   int64   `json:"base_views"`
	RecentShare float64 `json:"recent_share"` // share of the recent views (0-1)
	BaseShare   float64 `json:"base_share"`   // share of the baseline views (0-1)
	// ChangePct is the relative change of the share in %, nil when absent from the baseline
	ChangePct *float64 `json:"change_pct,omitempty"`
	New       bool     `json:"new"`
}

// InsightsTrendsPayload is the trends widget payload.
type InsightsTrendsPayload struct {
	RecentDays   int                 `json:"recent_days"`
	BaselineDays int                 `json:"baseline_days"`
	RecentViews  int64               `json:"recent_views"`
	BaseViews    int64               `json:"base_views"`
	Items        []InsightsTrendItem `json:"items"`
}

func init() {
	registerHomepageWidget("trends", HomepageWidgetSpec{Fetch: fetchInsightsTrendsWidget, Validate: validateInsightsTrendsWidget})
}

func validateInsightsTrendsWidget(cfg map[string]interface{}) error {
	if err := insightsValidateOneOf(cfg, "kind", "both", "tags", "actors"); err != nil {
		return fmt.Errorf("trends: %w", err)
	}
	if err := insightsValidateIntRange(cfg, "recent_days", 1, 365); err != nil {
		return fmt.Errorf("trends: %w", err)
	}
	if err := insightsValidateIntRange(cfg, "baseline_days", 1, 1825); err != nil {
		return fmt.Errorf("trends: %w", err)
	}
	if err := insightsValidateIntRange(cfg, "min_views", 1, 1000); err != nil {
		return fmt.Errorf("trends: %w", err)
	}
	return nil
}

func fetchInsightsTrendsWidget(s *HomepageService, userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	q, err := insightsQueries(s)
	if err != nil {
		return nil, err
	}
	recentDays := clampInt(widgetConfigInt(section.Config, "recent_days", insightsTrendsDefaultRecentDays), 1, 365)
	baselineDays := clampInt(widgetConfigInt(section.Config, "baseline_days", insightsTrendsDefaultBaselineDays), 1, 1825)
	minViews := int64(clampInt(widgetConfigInt(section.Config, "min_views", insightsTrendsDefaultMinViews), 1, 1000))
	kind := widgetConfigString(section.Config, "kind", "both")
	limit := clampInt(section.Limit, 1, 50)

	now := insightsTrendsNow()
	recentFrom := now.AddDate(0, 0, -recentDays)
	baseFrom := recentFrom.AddDate(0, 0, -baselineDays)

	totals, err := q.ViewTotals(userID, baseFrom, recentFrom)
	if err != nil {
		return nil, fmt.Errorf("trends: failed to count views: %w", err)
	}

	var candidates []InsightsTrendItem
	if kind != "actors" {
		rows, err := q.TagTrends(userID, baseFrom, recentFrom)
		if err != nil {
			return nil, fmt.Errorf("trends: failed to load tag views: %w", err)
		}
		candidates = append(candidates, insightsTrendItems("tag", rows, totals)...)
	}
	if kind != "tags" {
		rows, err := q.ActorTrends(userID, baseFrom, recentFrom)
		if err != nil {
			return nil, fmt.Errorf("trends: failed to load actor views: %w", err)
		}
		candidates = append(candidates, insightsTrendItems("actor", rows, totals)...)
	}

	return emptyWidgetData(section, InsightsTrendsPayload{
		RecentDays:   recentDays,
		BaselineDays: baselineDays,
		RecentViews:  totals.Recent,
		BaseViews:    totals.Base,
		Items:        insightsRankTrends(candidates, minViews, limit),
	}), nil
}

// insightsTrendItems computes the view shares of each row in both windows.
func insightsTrendItems(kind string, rows []data.InsightsTrendRow, totals data.InsightsViewTotals) []InsightsTrendItem {
	items := make([]InsightsTrendItem, 0, len(rows))
	for _, r := range rows {
		item := InsightsTrendItem{
			Kind: kind, ID: r.ID, UUID: r.UUID, Name: r.Name, Color: r.Color,
			RecentViews: r.RecentViews, BaseViews: r.BaseViews,
		}
		if totals.Recent > 0 {
			item.RecentShare = float64(r.RecentViews) / float64(totals.Recent)
		}
		if totals.Base > 0 {
			item.BaseShare = float64(r.BaseViews) / float64(totals.Base)
		}
		if item.BaseShare > 0 {
			pct := (item.RecentShare - item.BaseShare) / item.BaseShare * 100
			item.ChangePct = &pct
		} else {
			item.New = item.RecentViews > 0
		}
		items = append(items, item)
	}
	return items
}

// insightsRankTrends keeps the rising items with enough recent views, the largest share gain
// first (ties: more recent views, then name).
func insightsRankTrends(items []InsightsTrendItem, minViews int64, limit int) []InsightsTrendItem {
	rising := make([]InsightsTrendItem, 0, len(items))
	for _, it := range items {
		if it.RecentViews >= minViews && it.RecentShare > it.BaseShare {
			rising = append(rising, it)
		}
	}
	sort.SliceStable(rising, func(i, j int) bool {
		gi := rising[i].RecentShare - rising[i].BaseShare
		gj := rising[j].RecentShare - rising[j].BaseShare
		if gi != gj {
			return gi > gj
		}
		if rising[i].RecentViews != rising[j].RecentViews {
			return rising[i].RecentViews > rising[j].RecentViews
		}
		return rising[i].Name < rising[j].Name
	})
	if len(rising) > limit {
		rising = rising[:limit]
	}
	return rising
}
