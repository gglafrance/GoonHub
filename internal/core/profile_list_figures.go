package core

import (
	"context"
	"fmt"
	"time"

	"goonhub/internal/data"
)

// Figures of the custom lists (docs/PROFILES.md, "Figures"): under each card of a list, the
// owner's interaction the list is about (Top O: the O count, Top rated: the rating, Recently
// liked: the like date, Most watched: the views). Which figures a list shows is its Figures
// setting (auto, none, or one kind), which a custom_list block may override. "auto" takes the
// main figure from the sort and a second one from the filters on the owner's interactions.
//
// A figure reveals owner data, so each kind needs its privacy item: a figure the viewer may not
// see is not read, let alone sent (no administrator bypass), even when the list itself is
// visible (a rating shown on a Top O list needs the ratings item, not only jizz).

const (
	ListFiguresAuto = "auto"
	ListFiguresNone = "none"
	// ListFiguresList is a block option value only: follow the setting of the list.
	ListFiguresList = "list"

	ListFigureJizz   = "jizz"   // O count and last O date
	ListFigureRating = "rating" // the owner's rating
	ListFigureLiked  = "liked"  // the like date
	ListFigureViews  = "views"  // views and last watch date
)

// listFigureMax is the number of figures shown under a card (the main one, and a second one
// from the filters).
const listFigureMax = 2

// listFigureKinds are the figure kinds, in the order of the secondary figures.
var listFigureKinds = []string{ListFigureJizz, ListFigureRating, ListFigureLiked, ListFigureViews}

// listFigurePrivacyItem is the privacy item a figure reveals.
func listFigurePrivacyItem(kind string) string {
	switch kind {
	case ListFigureJizz:
		return PrivacyJizz
	case ListFigureRating:
		return PrivacyRatings
	case ListFigureLiked:
		return PrivacyLiked
	case ListFigureViews:
		return PrivacyActivity
	}
	return ""
}

// listFigurePrivacyItems are the privacy items of every figure kind.
func listFigurePrivacyItems() []string {
	out := make([]string, len(listFigureKinds))
	for i, k := range listFigureKinds {
		out[i] = listFigurePrivacyItem(k)
	}
	return out
}

// ValidListFiguresSetting reports whether v is a list setting (auto, none or a kind).
func ValidListFiguresSetting(v string) bool {
	return v == ListFiguresAuto || v == ListFiguresNone || listFigurePrivacyItem(v) != ""
}

// sortFigure is the main figure of a sort ("" for a sort on no owner data).
func sortFigure(sort string) string {
	switch {
	case isJizzSortOrder(sort):
		return ListFigureJizz
	case sort == SortMyRatingDesc || sort == SortMyRatingAsc:
		return ListFigureRating
	case sort == SortLikedAtDesc:
		return ListFigureLiked
	case sort == SortMyViewsDesc || sort == SortLastWatchedDesc:
		return ListFigureViews
	}
	return ""
}

// filterFigures are the figures the filters select scenes on (a filter on the absence of an
// interaction, unrated or unwatched, has nothing to show).
func filterFigures(f data.Filters) map[string]bool {
	return map[string]bool{
		ListFigureJizz:   (f.MinJizzCount != nil && *f.MinJizzCount > 0) || (f.MaxJizzCount != nil && *f.MaxJizzCount > 0),
		ListFigureRating: (f.MinRating != nil && *f.MinRating > 0) || (f.MaxRating != nil && *f.MaxRating > 0) || (f.Rated != nil && *f.Rated),
		ListFigureLiked:  f.Liked != nil && *f.Liked,
		ListFigureViews:  f.Watched != nil && *f.Watched,
	}
}

// AutoListFigures returns the figures of the auto setting: the one of the sort first, then
// those of the filters, at most listFigureMax; none for criteria on no owner data.
func AutoListFigures(f data.Filters) []string {
	out := []string{}
	if k := sortFigure(f.Sort); k != "" {
		out = append(out, k)
	}
	filters := filterFigures(f)
	for _, k := range listFigureKinds {
		if len(out) >= listFigureMax {
			break
		}
		if filters[k] && (len(out) == 0 || out[0] != k) {
			out = append(out, k)
		}
	}
	return out
}

// ListFigures returns the figures a setting shows for criteria, main first ("" = auto).
func ListFigures(setting string, f data.Filters) []string {
	switch setting {
	case "", ListFiguresAuto:
		return AutoListFigures(f)
	case ListFiguresNone:
		return []string{}
	}
	if listFigurePrivacyItem(setting) == "" {
		return []string{}
	}
	return []string{setting}
}

// effectiveListFigures returns what a block or the browse page shows: the setting (a block
// override, or "list" / "" for the list's own), the kinds the viewer may see only (visible:
// the owner's privacy items for the viewer).
func effectiveListFigures(rl ResolvedProfileList, override string, visible map[string]bool) []string {
	setting := override
	if setting == "" || setting == ListFiguresList {
		setting = rl.List.Figures
	}
	out := []string{}
	for _, k := range ListFigures(setting, rl.Filters) {
		if visible[listFigurePrivacyItem(k)] {
			out = append(out, k)
		}
	}
	return out
}

// ProfileSceneFigures are the owner's figures of a scene of a custom list (only the kinds the
// list shows are read; nil or zero = none).
type ProfileSceneFigures struct {
	JizzCount     int64
	LastJizzAt    *time.Time
	Rating        *float64
	LikedAt       *time.Time
	Views         int64
	LastWatchedAt *time.Time
}

// ProfileFigureReader reads a user's interactions with scenes (data.InteractionRepository).
type ProfileFigureReader interface {
	GetJizzCountsBySceneIDs(userID uint, sceneIDs []uint) (map[uint]int, error)
	GetLastJizzTimes(userID uint, sceneIDs []uint) (map[uint]time.Time, error)
	GetRatingsBySceneIDs(userID uint, sceneIDs []uint) (map[uint]float64, error)
	GetLikeTimes(userID uint, sceneIDs []uint) (map[uint]time.Time, error)
	GetWatchStats(userID uint, sceneIDs []uint) (map[uint]data.SceneWatchStat, error)
}

// WithListFigures gives the service what the figures of the custom lists read.
func (s *ProfileService) WithListFigures(r ProfileFigureReader) *ProfileService {
	s.figures = r
	s.deps.ListFigures = s.fillListFigures
	return s
}

// fillListFigures reads the owner's figures of the given kinds (already checked for the viewer)
// for the scenes of items, a few batched queries.
func (s *ProfileService) fillListFigures(_ context.Context, ownerID uint, items []ProfileSceneItem, kinds []string) error {
	if len(kinds) == 0 || len(items) == 0 || s.figures == nil {
		return nil
	}
	ids := make([]uint, len(items))
	for i, it := range items {
		ids[i] = it.Scene.ID
	}
	figs := make(map[uint]*ProfileSceneFigures, len(ids))
	for _, id := range ids {
		figs[id] = &ProfileSceneFigures{}
	}
	at := func(t time.Time) *time.Time {
		if t.IsZero() {
			return nil
		}
		return &t
	}
	for _, kind := range kinds {
		switch kind {
		case ListFigureJizz:
			counts, err := s.figures.GetJizzCountsBySceneIDs(ownerID, ids)
			if err != nil {
				return fmt.Errorf("failed to read the O counts: %w", err)
			}
			last, err := s.figures.GetLastJizzTimes(ownerID, ids)
			if err != nil {
				return fmt.Errorf("failed to read the last O dates: %w", err)
			}
			for id, f := range figs {
				if n := counts[id]; n > 0 {
					f.JizzCount = int64(n)
					f.LastJizzAt = at(last[id])
				}
			}
		case ListFigureRating:
			ratings, err := s.figures.GetRatingsBySceneIDs(ownerID, ids)
			if err != nil {
				return fmt.Errorf("failed to read the ratings: %w", err)
			}
			for id, f := range figs {
				if r, ok := ratings[id]; ok && r > 0 {
					f.Rating = &r
				}
			}
		case ListFigureLiked:
			likes, err := s.figures.GetLikeTimes(ownerID, ids)
			if err != nil {
				return fmt.Errorf("failed to read the like dates: %w", err)
			}
			for id, f := range figs {
				if t, ok := likes[id]; ok {
					f.LikedAt = at(t)
				}
			}
		case ListFigureViews:
			stats, err := s.figures.GetWatchStats(ownerID, ids)
			if err != nil {
				return fmt.Errorf("failed to read the views: %w", err)
			}
			for id, f := range figs {
				if st, ok := stats[id]; ok && st.Count > 0 {
					f.Views = st.Count
					f.LastWatchedAt = at(st.Last)
				}
			}
		}
	}
	for i := range items {
		items[i].Figures = figs[items[i].Scene.ID]
	}
	return nil
}
