package core

import (
	"fmt"
	"time"

	"goonhub/internal/data"
)

// on_this_day: scenes watched and/or added on today's month-day in previous years.
// Config: mode watched|added|both (default both).
// Widget payload: {"date": "MM-DD", "groups": [{"year": 2024, "watched": [ids], "added": [ids]}]}
// Scenes holds all the listed scenes (most recent years first).

type onThisDayGroup struct {
	Year    int    `json:"year"`
	Watched []uint `json:"watched"`
	Added   []uint `json:"added"`
}

type onThisDayPayload struct {
	Date   string           `json:"date"`
	Groups []onThisDayGroup `json:"groups"`
}

// onThisDayNow is the reference day (overridable in tests).
var onThisDayNow = time.Now

func init() {
	registerHomepageWidget("on_this_day", HomepageWidgetSpec{
		Fetch: fetchOnThisDayWidget,
		Validate: func(cfg map[string]interface{}) error {
			switch widgetConfigString(cfg, "mode", "both") {
			case "watched", "added", "both":
				return nil
			default:
				return fmt.Errorf("mode must be watched, added or both")
			}
		},
	})
}

// groupOnThisDay groups the rows by year (in row order) and returns the distinct scene IDs.
func groupOnThisDay(rows []data.OnThisDayRow) ([]onThisDayGroup, []uint) {
	var groups []onThisDayGroup
	index := map[int]int{}
	seen := map[uint]bool{}
	var ids []uint
	for _, r := range rows {
		i, ok := index[r.Year]
		if !ok {
			groups = append(groups, onThisDayGroup{Year: r.Year, Watched: []uint{}, Added: []uint{}})
			i = len(groups) - 1
			index[r.Year] = i
		}
		if r.Kind == "added" {
			groups[i].Added = append(groups[i].Added, r.SceneID)
		} else {
			groups[i].Watched = append(groups[i].Watched, r.SceneID)
		}
		if !seen[r.SceneID] {
			seen[r.SceneID] = true
			ids = append(ids, r.SceneID)
		}
	}
	if groups == nil {
		groups = []onThisDayGroup{}
	}
	return groups, ids
}

func fetchOnThisDayWidget(s *HomepageService, userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	queries, err := sceneWidgetQueries(s)
	if err != nil {
		return nil, err
	}
	mode := widgetConfigString(section.Config, "mode", "both")
	now := onThisDayNow()
	rows, err := queries.OnThisDay(userID, now, mode != "added", mode != "watched", sceneWidgetLimit(section, 12))
	if err != nil {
		return nil, fmt.Errorf("failed to list scenes of this day: %w", err)
	}
	groups, ids := groupOnThisDay(rows)
	scenes, err := loadWidgetScenes(s, ids)
	if err != nil {
		return nil, err
	}
	return &HomepageSectionData{
		Section: section,
		Scenes:  scenes,
		Total:   int64(len(scenes)),
		Widget:  onThisDayPayload{Date: now.Format("01-02"), Groups: groups},
	}, nil
}
