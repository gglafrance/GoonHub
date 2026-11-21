package core

import (
	"fmt"

	"goonhub/internal/data"
)

// preview_wall: a mosaic of scene previews playing together.
//
// Config: source (random|unwatched|liked|latest, default random), columns (3-6, default 4).
// Payload: {"source": "...", "columns": n}. Scenes with a generated preview come first.

const (
	previewWallDefaultColumns = 4
	previewWallMinColumns     = 3
	previewWallMaxColumns     = 6
)

var previewWallSources = []string{data.WidgetSourceRandom, data.WidgetSourceUnwatched, data.WidgetSourceLiked, data.WidgetSourceLatest}

type previewWallWidgetPayload struct {
	Source  string `json:"source"`
	Columns int    `json:"columns"`
}

func init() {
	registerHomepageWidget("preview_wall", HomepageWidgetSpec{
		Fetch:    fetchPreviewWallWidget,
		Validate: validatePreviewWallWidget,
	})
}

func validatePreviewWallWidget(cfg map[string]interface{}) error {
	if err := displayValidateOneOf(cfg, "source", previewWallSources...); err != nil {
		return fmt.Errorf("preview wall: %w", err)
	}
	if err := displayValidateIntRange(cfg, "columns", previewWallMinColumns, previewWallMaxColumns); err != nil {
		return fmt.Errorf("preview wall: %w", err)
	}
	return nil
}

func fetchPreviewWallWidget(s *HomepageService, userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	q, err := displayWidgetQueries(s)
	if err != nil {
		return nil, err
	}
	source := widgetConfigString(section.Config, "source", data.WidgetSourceRandom)
	columns := clampInt(widgetConfigInt(section.Config, "columns", previewWallDefaultColumns), previewWallMinColumns, previewWallMaxColumns)

	scenes, err := displaySourceScenes(q, userID, source, clampInt(section.Limit, 1, 50))
	if err != nil {
		return nil, fmt.Errorf("failed to get preview wall scenes: %w", err)
	}
	return &HomepageSectionData{
		Section: section,
		Scenes:  scenes,
		Total:   int64(len(scenes)),
		Widget:  previewWallWidgetPayload{Source: source, Columns: columns},
	}, nil
}
