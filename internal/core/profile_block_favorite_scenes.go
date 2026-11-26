package core

import (
	"context"
	"fmt"
)

// favorite_scenes: the favorite videos showcase, in the owner's order (Letterboxd "favorite
// films"). Edited with PUT /profiles/me/showcases/scenes.

func init() {
	registerProfileBlock(ProfileBlockSpec{
		Type:             "favorite_scenes",
		Label:            "Favorite videos",
		Description:      "A showcase of the videos you pick, in your order",
		PrivacyItem:      PrivacyFavorites,
		ViewerPermission: PermScenesView,
		DefaultVisible:   true,
		Order:            10,
		Fetch:            fetchFavoriteScenesBlock,
	})
}

func fetchFavoriteScenesBlock(ctx context.Context, deps *ProfileBlockDeps, req ProfileBlockRequest) (*ProfileBlockData, error) {
	ids, err := deps.Repo.FavoriteSceneIDs(ctx, req.OwnerID)
	if err != nil {
		return nil, fmt.Errorf("failed to read the favorite videos: %w", err)
	}
	scenes, err := deps.Repo.ScenesByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to load the favorite videos: %w", err)
	}
	items := make([]ProfileSceneItem, len(scenes))
	for i, s := range scenes {
		items[i] = ProfileSceneItem{Scene: s}
	}
	return &ProfileBlockData{Scenes: items, Empty: len(items) == 0}, nil
}
