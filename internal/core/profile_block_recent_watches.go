package core

import (
	"context"
	"fmt"
)

// recent_watches: the videos the owner watched last (one card per video, last watch date).

func init() {
	registerProfileBlock(ProfileBlockSpec{
		Type:             "recent_watches",
		Label:            "Recently watched",
		Description:      "The videos you watched last",
		PrivacyItem:      PrivacyActivity,
		ViewerPermission: PermScenesView,
		Options:          []ProfileBlockOption{profileCountOption(8)},
		DefaultVisible:   true,
		Order:            30,
		Fetch:            fetchRecentWatchesBlock,
	})
}

func fetchRecentWatchesBlock(ctx context.Context, deps *ProfileBlockDeps, req ProfileBlockRequest) (*ProfileBlockData, error) {
	refs, err := deps.Repo.RecentWatches(ctx, req.OwnerID, req.Config.Int("count"))
	if err != nil {
		return nil, fmt.Errorf("failed to read the recent watches: %w", err)
	}
	items, err := profileSceneItems(ctx, deps, refs)
	if err != nil {
		return nil, err
	}
	return &ProfileBlockData{Scenes: items, Empty: len(items) == 0}, nil
}
