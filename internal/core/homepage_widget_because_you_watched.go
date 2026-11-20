package core

import (
	"fmt"

	"goonhub/internal/data"
)

// because_you_watched: scenes related to the user's last watched scene. When the last scene
// has no (unwatched) related scenes, the previous watched scenes are tried.
// Config: exclude_watched (default true).
// Widget payload: {"source": {"id", "title"} | null}

// becauseYouWatchedSources is how many recently watched scenes are tried as the source.
const becauseYouWatchedSources = 5

type becauseYouWatchedSource struct {
	ID    uint   `json:"id"`
	Title string `json:"title"`
}

type becauseYouWatchedPayload struct {
	Source *becauseYouWatchedSource `json:"source"`
}

func init() {
	registerHomepageWidget("because_you_watched", HomepageWidgetSpec{Fetch: fetchBecauseYouWatchedWidget})
}

func fetchBecauseYouWatchedWidget(s *HomepageService, userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	empty := &HomepageSectionData{Section: section, Scenes: []data.Scene{}, Widget: becauseYouWatchedPayload{}}
	related := s.WidgetDeps().RelatedScenes
	if related == nil {
		return empty, nil
	}
	queries, err := sceneWidgetQueries(s)
	if err != nil {
		return nil, err
	}
	limit := sceneWidgetLimit(section, 12)
	excludeWatched := widgetConfigBool(section.Config, "exclude_watched", true)

	sources, err := queries.RecentlyWatchedSceneIDs(userID, becauseYouWatchedSources)
	if err != nil {
		return nil, fmt.Errorf("failed to get the last watched scenes: %w", err)
	}
	for _, sourceID := range sources {
		candidates, err := related.GetRelatedScenes(sourceID, userID, 50)
		if err != nil {
			return nil, fmt.Errorf("failed to get related scenes: %w", err)
		}
		picked, err := pickRelatedScenes(queries, userID, sourceID, candidates, excludeWatched, limit)
		if err != nil {
			return nil, err
		}
		if len(picked) == 0 {
			continue
		}
		payload := becauseYouWatchedPayload{}
		if source, err := s.sceneRepo.GetByID(sourceID); err == nil && source != nil {
			payload.Source = &becauseYouWatchedSource{ID: source.ID, Title: source.Title}
		}
		return &HomepageSectionData{Section: section, Scenes: picked, Total: int64(len(picked)), Widget: payload}, nil
	}
	return empty, nil
}

// pickRelatedScenes keeps up to limit candidates, without the source and (optionally) the
// scenes the user already watched.
func pickRelatedScenes(queries *data.HomepageSceneQueries, userID, sourceID uint, candidates []data.Scene, excludeWatched bool, limit int) ([]data.Scene, error) {
	watched := map[uint]bool{}
	if excludeWatched && len(candidates) > 0 {
		ids := make([]uint, len(candidates))
		for i, c := range candidates {
			ids[i] = c.ID
		}
		var err error
		if watched, err = queries.WatchedSceneIDs(userID, ids); err != nil {
			return nil, fmt.Errorf("failed to check watched scenes: %w", err)
		}
	}
	picked := []data.Scene{}
	for _, c := range candidates {
		if c.ID == sourceID || watched[c.ID] || c.TrashedAt != nil {
			continue
		}
		picked = append(picked, c)
		if len(picked) >= limit {
			break
		}
	}
	return picked, nil
}
