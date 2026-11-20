package core

import (
	"fmt"

	"go.uber.org/zap"

	"goonhub/internal/data"
)

// hero: a large banner featuring one scene, cycling through a pool (section.Limit scenes).
//
// Config: source (liked|top_jizz|random|unwatched, default liked; falls back to random when the
// source is empty), rotate_seconds (0 = manual, 5-300, default 0).
// Payload: {"source", "rotate_seconds", "details": {scene_id: {actors, tags, liked, jizz_count}}}.

const heroMaxRotateSeconds = 300

var heroSources = []string{data.WidgetSourceLiked, data.WidgetSourceTopJizz, data.WidgetSourceRandom, data.WidgetSourceUnwatched}

type heroSceneDetails struct {
	Actors    []string `json:"actors"`
	Tags      []string `json:"tags"`
	Liked     bool     `json:"liked"`
	JizzCount int      `json:"jizz_count"`
}

type heroWidgetPayload struct {
	Source        string                    `json:"source"`
	RotateSeconds int                       `json:"rotate_seconds"`
	Details       map[uint]heroSceneDetails `json:"details"`
}

func init() {
	registerHomepageWidget("hero", HomepageWidgetSpec{
		Fetch:    fetchHeroWidget,
		Validate: validateHeroWidget,
	})
}

func validateHeroWidget(cfg map[string]interface{}) error {
	if err := displayValidateOneOf(cfg, "source", heroSources...); err != nil {
		return fmt.Errorf("spotlight: %w", err)
	}
	if v, ok := cfg["rotate_seconds"]; ok && v != nil && widgetConfigInt(cfg, "rotate_seconds", -1) != 0 {
		if err := displayValidateIntRange(cfg, "rotate_seconds", 5, heroMaxRotateSeconds); err != nil {
			return fmt.Errorf("spotlight: %w (or 0 to disable)", err)
		}
	}
	return nil
}

func fetchHeroWidget(s *HomepageService, userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	q, err := displayWidgetQueries(s)
	if err != nil {
		return nil, err
	}
	source := widgetConfigString(section.Config, "source", data.WidgetSourceLiked)
	rotate := clampInt(widgetConfigInt(section.Config, "rotate_seconds", 0), 0, heroMaxRotateSeconds)
	limit := clampInt(section.Limit, 1, 20)

	scenes, err := displaySourceScenes(q, userID, source, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get spotlight scenes: %w", err)
	}
	if len(scenes) == 0 && source != data.WidgetSourceRandom {
		// Nothing liked/jizzed/unwatched yet: feature random scenes instead
		if scenes, err = displaySourceScenes(q, userID, data.WidgetSourceRandom, limit); err != nil {
			return nil, fmt.Errorf("failed to get spotlight scenes: %w", err)
		}
	}

	return &HomepageSectionData{
		Section: section,
		Scenes:  scenes,
		Total:   int64(len(scenes)),
		Widget: heroWidgetPayload{
			Source:        source,
			RotateSeconds: rotate,
			Details:       heroDetails(s, userID, scenes),
		},
	}, nil
}

// heroDetails loads the actors, tags and interactions shown on the banner (best effort).
func heroDetails(s *HomepageService, userID uint, scenes []data.Scene) map[uint]heroSceneDetails {
	details := make(map[uint]heroSceneDetails, len(scenes))
	if len(scenes) == 0 {
		return details
	}
	ids := displaySceneIDs(scenes)
	for _, id := range ids {
		details[id] = heroSceneDetails{Actors: []string{}, Tags: []string{}}
	}
	if s.actorRepo != nil {
		if actors, err := s.actorRepo.GetSceneActorsMultiple(ids); err == nil {
			for id, list := range actors {
				d := details[id]
				for _, a := range list {
					d.Actors = append(d.Actors, a.Name)
				}
				details[id] = d
			}
		} else {
			s.logger.Warn("spotlight: failed to load actors", zap.Error(err))
		}
	}
	if s.tagRepo != nil {
		if tags, err := s.tagRepo.GetSceneTagsMultiple(ids); err == nil {
			for id, list := range tags {
				d := details[id]
				for _, t := range list {
					d.Tags = append(d.Tags, t.Name)
				}
				details[id] = d
			}
		} else {
			s.logger.Warn("spotlight: failed to load tags", zap.Error(err))
		}
	}
	if s.interactionRepo != nil {
		if likes, err := s.interactionRepo.GetLikesBySceneIDs(userID, ids); err == nil {
			for id, liked := range likes {
				d := details[id]
				d.Liked = liked
				details[id] = d
			}
		}
		if counts, err := s.interactionRepo.GetJizzCountsBySceneIDs(userID, ids); err == nil {
			for id, count := range counts {
				d := details[id]
				d.JizzCount = count
				details[id] = d
			}
		}
	}
	return details
}
