package core

import (
	"context"
	"fmt"
)

// favorite_markers: the favorite markers showcase, in the owner's order. The owner picks
// markers they see (their own or shared with them); a marker that the owner no longer sees
// (its owner stopped sharing) or whose scene left the library is skipped.

func init() {
	registerProfileBlock(ProfileBlockSpec{
		Type:             "favorite_markers",
		Label:            "Favorite markers",
		Description:      "A showcase of the markers you pick, in your order",
		PrivacyItem:      PrivacyFavoriteMarkers,
		ViewerPermission: PermScenesView,
		DefaultVisible:   true,
		Order:            20,
		Fetch:            fetchFavoriteMarkersBlock,
	})
}

func fetchFavoriteMarkersBlock(ctx context.Context, deps *ProfileBlockDeps, req ProfileBlockRequest) (*ProfileBlockData, error) {
	ids, err := deps.Repo.FavoriteMarkerIDs(ctx, req.OwnerID)
	if err != nil {
		return nil, fmt.Errorf("failed to read the favorite markers: %w", err)
	}
	markers, err := deps.Repo.VisibleMarkers(ctx, req.OwnerID, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to load the favorite markers: %w", err)
	}
	return &ProfileBlockData{Markers: withMarkerTags(deps, markers), Empty: len(markers) == 0}, nil
}
