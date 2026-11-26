package core

import (
	"context"
	"fmt"
	"math"
)

// custom_list: the first videos of one of the owner's custom lists (profile_custom_lists.go),
// as a grid of cards or a compact row, with a "Browse" link to the whole list. The block is
// shown to a viewer only when the list is (its audience and the privacy items of its criteria,
// checked by the service before Fetch); its privacy item is the profile item itself.

const customListBlockType = "custom_list"

// ProfileCustomListPayload is the payload of a custom_list block.
type ProfileCustomListPayload struct {
	ListID  uint   `json:"list_id"`
	Title   string `json:"title"`
	Display string `json:"display"`
	Total   int64  `json:"total"`
}

func init() {
	registerProfileBlock(ProfileBlockSpec{
		Type:             customListBlockType,
		Label:            "Custom list",
		Description:      "The first videos of one of your custom lists (search criteria), with Browse",
		PrivacyItem:      PrivacyProfile,
		ViewerPermission: PermScenesView,
		Options: []ProfileBlockOption{
			{Key: "list_id", Label: "List", Kind: ProfileOptionInt, Min: 0, Max: math.MaxInt32, Default: 0},
			{Key: "count", Label: "Items", Kind: ProfileOptionInt, Min: 2, Max: 24, Default: 6},
			{Key: "display", Label: "Display", Kind: ProfileOptionChoice, DefaultChoice: "grid", Choices: []ProfileBlockChoice{
				{"grid", "Grid"}, {"row", "Compact row"},
			}},
		},
		Order: 110,
		Multi: true,
		Fetch: fetchCustomListBlock,
	})
}

func fetchCustomListBlock(ctx context.Context, deps *ProfileBlockDeps, req ProfileBlockRequest) (*ProfileBlockData, error) {
	if req.List == nil {
		return &ProfileBlockData{Empty: true}, nil
	}
	if deps.RunList == nil {
		return nil, fmt.Errorf("custom lists are not available")
	}
	scenes, total, err := deps.RunList(ctx, req.OwnerID, req.Viewer, req.List.Filters, 1, req.Config.Int("count"))
	if err != nil {
		return nil, err
	}
	items := make([]ProfileSceneItem, len(scenes))
	for i, s := range scenes {
		items[i] = ProfileSceneItem{Scene: s}
	}
	return &ProfileBlockData{
		Scenes: items,
		Widget: ProfileCustomListPayload{ListID: req.List.List.ID, Title: req.List.List.Title, Display: req.Config.String("display"), Total: total},
		Total:  &total,
		Empty:  len(items) == 0,
		Title:  req.List.List.Title,
	}, nil
}
