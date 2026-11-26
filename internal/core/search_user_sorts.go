package core

import (
	"fmt"
	"sort"

	"goonhub/internal/data"
)

// Sorts ordering scenes by the user's own interactions (besides the o sorts of
// search_service.go), done in Go over all the matching IDs like the o sorts. Scenes without the
// interaction (not rated, not liked, never watched) come last in both directions; ties are
// broken by ascending scene ID. The profile custom lists use them with the owner's
// interactions (docs/PROFILES.md, "Custom lists").
const (
	SortMyRatingDesc    = "my_rating_desc"
	SortMyRatingAsc     = "my_rating_asc"
	SortLikedAtDesc     = "liked_at_desc"
	SortMyViewsDesc     = "my_views_desc"
	SortLastWatchedDesc = "last_watched_desc"
)

// isUserSortOrder reports whether the sort orders scenes by the user's rating, like date,
// views or last watch.
func isUserSortOrder(sort string) bool {
	switch sort {
	case SortMyRatingDesc, SortMyRatingAsc, SortLikedAtDesc, SortMyViewsDesc, SortLastWatchedDesc:
		return true
	}
	return false
}

// userSortKeys reads the sort key of each scene the user interacted with.
func (s *SearchService) userSortKeys(params data.SceneSearchParams, ids []uint) (map[uint]float64, error) {
	keys := make(map[uint]float64, len(ids))
	if params.UserID == 0 || s.interactionRepo == nil {
		return keys, nil
	}
	for start := 0; start < len(ids); start += jizzLookupBatch {
		batch := ids[start:min(start+jizzLookupBatch, len(ids))]
		switch params.Sort {
		case SortMyRatingDesc, SortMyRatingAsc:
			ratings, err := s.interactionRepo.GetRatingsBySceneIDs(params.UserID, batch)
			if err != nil {
				return nil, fmt.Errorf("failed to get ratings: %w", err)
			}
			for id, r := range ratings {
				keys[id] = r
			}
		case SortLikedAtDesc:
			times, err := s.interactionRepo.GetLikeTimes(params.UserID, batch)
			if err != nil {
				return nil, fmt.Errorf("failed to get like dates: %w", err)
			}
			for id, at := range times {
				keys[id] = float64(at.UnixNano())
			}
		case SortMyViewsDesc, SortLastWatchedDesc:
			stats, err := s.interactionRepo.GetWatchStats(params.UserID, batch)
			if err != nil {
				return nil, fmt.Errorf("failed to get watch stats: %w", err)
			}
			for id, st := range stats {
				if params.Sort == SortMyViewsDesc {
					keys[id] = float64(st.Count)
				} else {
					keys[id] = float64(st.Last.UnixNano())
				}
			}
		}
	}
	return keys, nil
}

// handleUserSort orders all matching IDs by the user's interaction, then paginates.
func (s *SearchService) handleUserSort(allIDs []uint, params data.SceneSearchParams) (*SearchResult, error) {
	keys, err := s.userSortKeys(params, allIDs)
	if err != nil {
		return nil, err
	}
	desc := params.Sort != SortMyRatingAsc
	ordered := orderByKeys(allIDs, keys, desc)

	total := int64(len(ordered))
	page, limit := params.Page, params.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	offset := (page - 1) * limit
	if offset >= len(ordered) {
		return &SearchResult{Scenes: []data.Scene{}, Total: total}, nil
	}
	end := min(offset+limit, len(ordered))
	scenes, err := s.sceneRepo.GetByIDs(ordered[offset:end])
	if err != nil {
		return nil, fmt.Errorf("failed to fetch scenes by IDs: %w", err)
	}
	return &SearchResult{Scenes: scenes, Total: total}, nil
}

// orderByKeys sorts ids by their key (missing keys last), ties by ascending id.
func orderByKeys(ids []uint, keys map[uint]float64, desc bool) []uint {
	ordered := make([]uint, len(ids))
	copy(ordered, ids)
	sort.SliceStable(ordered, func(i, j int) bool {
		ki, okI := keys[ordered[i]]
		kj, okJ := keys[ordered[j]]
		if okI != okJ {
			return okI
		}
		if ki != kj {
			if desc {
				return ki > kj
			}
			return ki < kj
		}
		return ordered[i] < ordered[j]
	})
	return ordered
}
