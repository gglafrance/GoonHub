package core

import (
	"context"
	"fmt"
	"math"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"
)

// Full lists of a profile (docs/PROFILES.md, "Lists"): the liked, rated and jizz videos of a
// user and the histogram of their ratings. Each one needs, for every viewer (no administrator
// bypass), the profile item and its own privacy item (liked, ratings, jizz), and scenes:view
// (they show library content). Nothing is read before these checks pass.

// Rating scale: half stars from 0.5 to 5 (InteractionService.SetRating).
const (
	ProfileRatingMin  = 0.5
	ProfileRatingMax  = 5.0
	ProfileRatingStep = 0.5
)

// ProfileRatingSteps are the values of the histogram bars, ascending.
func ProfileRatingSteps() []float64 {
	n := int(math.Round((ProfileRatingMax-ProfileRatingMin)/ProfileRatingStep)) + 1
	out := make([]float64, n)
	for i := range out {
		out[i] = ProfileRatingMin + float64(i)*ProfileRatingStep
	}
	return out
}

// ValidProfileRating reports whether v is a value of the rating scale (a bar).
func ValidProfileRating(v float64) bool {
	if math.IsNaN(v) || v < ProfileRatingMin || v > ProfileRatingMax {
		return false
	}
	return math.Abs(v*2-math.Round(v*2)) < 1e-9
}

// ProfileRatingHistogram is the ratings chart of a profile: one bucket per value of the scale
// (empty ones included, ascending), the number of ratings and their average.
type ProfileRatingHistogram struct {
	Buckets []data.ProfileRatingBucket `json:"buckets"`
	Total   int64                      `json:"total"`
	Average *float64                   `json:"average"`
}

// BuildRatingHistogram spreads the non-empty buckets of the repository over the full scale. A
// value off the scale (an imported rating) goes to the nearest bucket.
func BuildRatingHistogram(summary *data.ProfileRatingSummary) ProfileRatingHistogram {
	steps := ProfileRatingSteps()
	out := ProfileRatingHistogram{Buckets: make([]data.ProfileRatingBucket, len(steps))}
	for i, v := range steps {
		out.Buckets[i].Rating = v
	}
	if summary == nil {
		return out
	}
	for _, b := range summary.Buckets {
		i := int(math.Round((b.Rating - ProfileRatingMin) / ProfileRatingStep))
		if i < 0 {
			i = 0
		}
		if i >= len(steps) {
			i = len(steps) - 1
		}
		out.Buckets[i].Count += b.Count
		out.Total += b.Count
	}
	if out.Total > 0 && summary.Average != nil {
		avg := math.Round(*summary.Average*100) / 100
		out.Average = &avg
	}
	return out
}

// ProfileList is a list of a profile.
type ProfileList string

const (
	ProfileListLiked ProfileList = "liked"
	ProfileListRated ProfileList = "rated"
	ProfileListJizz  ProfileList = "jizz"
)

// privacyItem is the privacy item of a list.
func (l ProfileList) privacyItem() string {
	switch l {
	case ProfileListRated:
		return PrivacyRatings
	case ProfileListJizz:
		return PrivacyJizz
	default:
		return PrivacyLiked
	}
}

// sorts are the sorts of a list, the default first.
func (l ProfileList) sorts() []string {
	switch l {
	case ProfileListRated:
		return []string{data.ProfileSortRatingDesc, data.ProfileSortRatingAsc, data.ProfileSortRatedDesc, data.ProfileSortRatedAsc}
	case ProfileListJizz:
		return []string{data.ProfileSortJizzRecent, data.ProfileSortJizzCount}
	default:
		return []string{data.ProfileSortLikedDesc, data.ProfileSortLikedAsc}
	}
}

// ProfileListInput is a page of a list as asked by a viewer.
type ProfileListInput struct {
	Sort string
	// Rating keeps one bar of the histogram (rated list only)
	Rating *float64
	Page   int
	Limit  int
}

// ProfileListPage is a page of a list, sort resolved.
type ProfileListPage struct {
	Items []ProfileSceneItem
	Total int64
	Page  int
	Limit int
	Sort  string
}

// ProfileListsVisibility tells which lists of a profile a viewer may open (the tabs).
type ProfileListsVisibility struct {
	Liked   bool
	Ratings bool
	Jizz    bool
}

// listOwner resolves the owner of a list and checks that the viewer may see it (profile and
// item, scenes:view). ErrPrivacyRestricted when hidden.
func (s *ProfileService) listOwner(ctx context.Context, viewer ProfileViewer, username, item string) (uint, error) {
	if !viewer.Has(PermScenesView) {
		return 0, apperrors.NewForbiddenError("insufficient permissions")
	}
	owner, err := s.social.UserByUsername(ctx, username)
	if err != nil {
		return 0, err
	}
	ok, err := s.privacy.CanViewAll(ctx, viewer.UserID, owner.ID, PrivacyProfile, item)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, apperrors.ErrPrivacyRestricted()
	}
	return owner.ID, nil
}

// RatingHistogram returns the ratings chart of username as the viewer may see it.
func (s *ProfileService) RatingHistogram(ctx context.Context, viewer ProfileViewer, username string) (*ProfileRatingHistogram, error) {
	ownerID, err := s.listOwner(ctx, viewer, username, PrivacyRatings)
	if err != nil {
		return nil, err
	}
	summary, err := s.repo.RatingSummary(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("failed to read the ratings: %w", err)
	}
	h := BuildRatingHistogram(summary)
	return &h, nil
}

// List returns a page of a list of username (liked, rated or jizz videos).
func (s *ProfileService) List(ctx context.Context, viewer ProfileViewer, username string, list ProfileList, in ProfileListInput) (*ProfileListPage, error) {
	sorts := list.sorts()
	sort := in.Sort
	if sort == "" {
		sort = sorts[0]
	}
	known := false
	for _, v := range sorts {
		known = known || v == sort
	}
	if !known {
		return nil, apperrors.NewValidationErrorWithField("sort", fmt.Sprintf("unknown sort %q", in.Sort))
	}
	if in.Rating != nil && (list != ProfileListRated || !ValidProfileRating(*in.Rating)) {
		return nil, apperrors.NewValidationErrorWithField("rating", "rating must be a half step between 0.5 and 5")
	}
	ownerID, err := s.listOwner(ctx, viewer, username, list.privacyItem())
	if err != nil {
		return nil, err
	}
	q := data.ProfileListQuery{Sort: sort, Rating: in.Rating, Page: in.Page, Limit: in.Limit}
	var refs []data.ProfileSceneRef
	var total int64
	switch list {
	case ProfileListRated:
		refs, total, err = s.repo.RatedScenes(ctx, ownerID, q)
	case ProfileListJizz:
		refs, total, err = s.repo.JizzScenes(ctx, ownerID, q)
	default:
		refs, total, err = s.repo.LikedScenesPage(ctx, ownerID, q)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read the %s videos: %w", list, err)
	}
	items, err := profileSceneItems(ctx, &s.deps, refs)
	if err != nil {
		return nil, err
	}
	if list == ProfileListJizz {
		items = asJizzItems(items)
	}
	return &ProfileListPage{Items: items, Total: total, Page: in.Page, Limit: in.Limit, Sort: sort}, nil
}

// listsVisibility tells which lists the viewer may open, from the visibility of the items.
func listsVisibility(viewer ProfileViewer, visible map[string]bool) ProfileListsVisibility {
	if !viewer.Has(PermScenesView) {
		return ProfileListsVisibility{}
	}
	return ProfileListsVisibility{Liked: visible[PrivacyLiked], Ratings: visible[PrivacyRatings], Jizz: visible[PrivacyJizz]}
}
