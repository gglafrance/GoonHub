package core

import (
	"context"
	"fmt"

	"goonhub/internal/data"
)

// jizz: the videos the owner jizzed to last, with their O count, and the O total. Sensitive:
// its privacy item (jizz) defaults to friends, like the other activity items.

// ProfileJizzPayload is the payload of the jizz block: the O total over the videos still in
// the library and the number of those videos.
type ProfileJizzPayload struct {
	Total  int64 `json:"total"`
	Scenes int64 `json:"scenes"`
}

func init() {
	registerProfileBlock(ProfileBlockSpec{
		Type:             "jizz",
		Label:            "Jizz",
		Description:      "Your O total and the videos you came to last",
		PrivacyItem:      PrivacyJizz,
		ViewerPermission: PermScenesView,
		Options:          []ProfileBlockOption{profileCountOption(6)},
		DefaultVisible:   true,
		Order:            76,
		Fetch:            fetchJizzBlock,
	})
}

func fetchJizzBlock(ctx context.Context, deps *ProfileBlockDeps, req ProfileBlockRequest) (*ProfileBlockData, error) {
	summary, err := deps.Repo.JizzSummary(ctx, req.OwnerID)
	if err != nil {
		return nil, fmt.Errorf("failed to read the O total: %w", err)
	}
	refs, _, err := deps.Repo.JizzScenes(ctx, req.OwnerID, data.ProfileListQuery{Sort: data.ProfileSortJizzRecent, Page: 1, Limit: req.Config.Int("count")})
	if err != nil {
		return nil, fmt.Errorf("failed to read the jizz videos: %w", err)
	}
	items, err := profileSceneItems(ctx, deps, refs)
	if err != nil {
		return nil, err
	}
	total := summary.Scenes
	return &ProfileBlockData{
		Scenes: asJizzItems(items),
		Widget: ProfileJizzPayload{Total: summary.Total, Scenes: summary.Scenes},
		Total:  &total,
		Empty:  summary.Scenes == 0,
	}, nil
}
