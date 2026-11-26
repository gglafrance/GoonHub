package core

import (
	"context"
	"fmt"
)

// liked_scenes: the videos the owner liked last, with the number of liked videos.

func init() {
	registerProfileBlock(ProfileBlockSpec{
		Type:             "liked_scenes",
		Label:            "Liked videos",
		Description:      "The videos you liked last",
		PrivacyItem:      PrivacyLiked,
		ViewerPermission: PermScenesView,
		Options:          []ProfileBlockOption{profileCountOption(8)},
		Order:            60,
		Fetch:            fetchLikedScenesBlock,
	})
}

func fetchLikedScenesBlock(ctx context.Context, deps *ProfileBlockDeps, req ProfileBlockRequest) (*ProfileBlockData, error) {
	refs, total, err := deps.Repo.LikedScenes(ctx, req.OwnerID, req.Config.Int("count"))
	if err != nil {
		return nil, fmt.Errorf("failed to read the liked videos: %w", err)
	}
	items, err := profileSceneItems(ctx, deps, refs)
	if err != nil {
		return nil, err
	}
	return &ProfileBlockData{Scenes: items, Total: &total, Empty: len(items) == 0}, nil
}
