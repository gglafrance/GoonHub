package core

import (
	"context"
	"fmt"
)

// most_watched: the videos the owner watched the most over a period, with their view count.

func init() {
	registerProfileBlock(ProfileBlockSpec{
		Type:             "most_watched",
		Label:            "Most watched",
		Description:      "The videos you watched the most over a period",
		PrivacyItem:      PrivacyActivity,
		ViewerPermission: PermScenesView,
		Options:          []ProfileBlockOption{profileCountOption(8), profilePeriodOption(profilePeriodMonth)},
		Order:            40,
		Fetch:            fetchMostWatchedBlock,
	})
}

func fetchMostWatchedBlock(ctx context.Context, deps *ProfileBlockDeps, req ProfileBlockRequest) (*ProfileBlockData, error) {
	since := profilePeriodSince(req.Config.String("period"), deps.now())
	refs, err := deps.Repo.MostWatched(ctx, req.OwnerID, since, req.Config.Int("count"))
	if err != nil {
		return nil, fmt.Errorf("failed to read the most watched videos: %w", err)
	}
	items, err := profileSceneItems(ctx, deps, refs)
	if err != nil {
		return nil, err
	}
	return &ProfileBlockData{Scenes: items, Empty: len(items) == 0}, nil
}
