package core

import (
	"context"
	"fmt"
)

// ratings: the chart of the owner's ratings (Letterboxd style): one bar per half star, the
// number of ratings and their average. A bar opens the videos rated with that value (the
// rated list, same privacy item).

func init() {
	registerProfileBlock(ProfileBlockSpec{
		Type:             "ratings",
		Label:            "Ratings",
		Description:      "The chart of your ratings, from half a star to five stars",
		PrivacyItem:      PrivacyRatings,
		ViewerPermission: PermScenesView,
		DefaultVisible:   true,
		Order:            75,
		Fetch:            fetchRatingsBlock,
	})
}

func fetchRatingsBlock(ctx context.Context, deps *ProfileBlockDeps, req ProfileBlockRequest) (*ProfileBlockData, error) {
	summary, err := deps.Repo.RatingSummary(ctx, req.OwnerID)
	if err != nil {
		return nil, fmt.Errorf("failed to read the ratings: %w", err)
	}
	h := BuildRatingHistogram(summary)
	total := h.Total
	return &ProfileBlockData{Widget: h, Total: &total, Empty: h.Total == 0}, nil
}
