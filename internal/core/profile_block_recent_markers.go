package core

import (
	"context"
	"fmt"
)

// recent_markers: the markers the owner created last (their own markers only). Showing this
// block exposes these markers to its audience, which is its point (docs/PROFILES.md).

func init() {
	registerProfileBlock(ProfileBlockSpec{
		Type:             "recent_markers",
		Label:            "Recent markers",
		Description:      "The markers you created last",
		PrivacyItem:      PrivacyRecentMarkers,
		ViewerPermission: PermScenesView,
		Options:          []ProfileBlockOption{profileCountOption(8)},
		Order:            50,
		Fetch:            fetchRecentMarkersBlock,
	})
}

func fetchRecentMarkersBlock(ctx context.Context, deps *ProfileBlockDeps, req ProfileBlockRequest) (*ProfileBlockData, error) {
	markers, err := deps.Repo.RecentMarkers(ctx, req.OwnerID, req.Config.Int("count"))
	if err != nil {
		return nil, fmt.Errorf("failed to read the recent markers: %w", err)
	}
	return &ProfileBlockData{Markers: withMarkerTags(deps, markers), Empty: len(markers) == 0}, nil
}
