package core

import (
	"context"
	"fmt"

	"goonhub/internal/data"
)

// activity_calendar: the heatmap of the owner's views and/or O per day, with streaks. It
// reuses the activity_calendar homepage widget (homepage_widget_activity_calendar.go) for the
// owner: the payload only holds daily counts.

func init() {
	registerProfileBlock(ProfileBlockSpec{
		Type:        "activity_calendar",
		Label:       "Activity calendar",
		Description: "A heatmap of your views or O per day, with your streaks",
		PrivacyItem: PrivacyActivity,
		Options: []ProfileBlockOption{
			{Key: "weeks", Label: "Weeks", Kind: ProfileOptionInt, Min: activityCalendarMinWeeks, Max: activityCalendarMaxWeeks, Default: 26},
			{Key: "metric", Label: "Metric", Kind: ProfileOptionChoice, DefaultChoice: statsMetricViews, Choices: []ProfileBlockChoice{
				{statsMetricViews, "Views"}, {statsMetricJizz, "O"}, {statsMetricBoth, "Views and O"},
			}},
		},
		DefaultVisible: true,
		Order:          80,
		Fetch:          fetchActivityCalendarBlock,
	})
}

func fetchActivityCalendarBlock(ctx context.Context, deps *ProfileBlockDeps, req ProfileBlockRequest) (*ProfileBlockData, error) {
	if deps.Homepage == nil {
		return nil, fmt.Errorf("activity calendar: homepage widgets not available")
	}
	section := data.HomepageSection{
		ID:      "profile-activity-calendar",
		Type:    "activity_calendar",
		Enabled: true,
		Config:  map[string]interface{}{"weeks": req.Config.Int("weeks"), "metric": req.Config.String("metric")},
	}
	out, err := fetchActivityCalendarWidget(deps.Homepage, req.OwnerID, section)
	if err != nil {
		return nil, err
	}
	empty := true
	if payload, ok := out.Widget.(ActivityCalendarPayload); ok {
		for _, s := range payload.Series {
			if s.Total > 0 || s.LongestStreak > 0 {
				empty = false
			}
		}
	}
	return &ProfileBlockData{Widget: out.Widget, Empty: empty}, nil
}
