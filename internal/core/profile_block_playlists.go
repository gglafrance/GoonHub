package core

import (
	"context"
	"fmt"
	"time"
)

// playlists: the owner's public playlists. The viewer needs playlists:view_public (as for the
// playlists page); the covers (first scene thumbnail) also need scenes:view.

// ProfilePlaylistItem is a playlist of the playlists block.
type ProfilePlaylistItem struct {
	UUID       string    `json:"uuid"`
	Name       string    `json:"name"`
	SceneCount int64     `json:"scene_count"`
	CoverScene *uint     `json:"cover_scene_id,omitempty"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ProfilePlaylistsPayload is the payload of the playlists block.
type ProfilePlaylistsPayload struct {
	Items []ProfilePlaylistItem `json:"items"`
}

func init() {
	registerProfileBlock(ProfileBlockSpec{
		Type:             "playlists",
		Label:            "Public playlists",
		Description:      "Your public playlists",
		PrivacyItem:      PrivacyPlaylists,
		ViewerPermission: PermPlaylistsPublic,
		Options:          []ProfileBlockOption{profileCountOption(6)},
		DefaultVisible:   true,
		Order:            90,
		Fetch:            fetchPlaylistsBlock,
	})
}

func fetchPlaylistsBlock(ctx context.Context, deps *ProfileBlockDeps, req ProfileBlockRequest) (*ProfileBlockData, error) {
	rows, total, err := deps.Repo.PublicPlaylists(ctx, req.OwnerID, req.Config.Int("count"))
	if err != nil {
		return nil, fmt.Errorf("failed to read the playlists: %w", err)
	}
	covers := req.Viewer.Has(PermScenesView)
	payload := ProfilePlaylistsPayload{Items: make([]ProfilePlaylistItem, len(rows))}
	for i, p := range rows {
		item := ProfilePlaylistItem{UUID: p.UUID, Name: p.Name, SceneCount: p.SceneCount, UpdatedAt: p.UpdatedAt}
		if covers {
			item.CoverScene = p.CoverSceneID
		}
		payload.Items[i] = item
	}
	return &ProfileBlockData{Widget: payload, Total: &total, Empty: len(rows) == 0}, nil
}
